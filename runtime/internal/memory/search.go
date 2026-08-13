package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DefaultLimit caps retrieval. Eight evidence-backed facts fit in a prompt; a
// hundred is context flooding with extra steps.
const DefaultLimit = 8

// Disclaimer rides along with every retrieved memory, wherever it is retrieved
// from.
//
// Retrieved memory sits below repository facts in the source-of-truth order.
// Saying so on every response is cheap; a model treating a stale memory as
// authority is not.
const Disclaimer = "Supporting context only. The repository is the source of truth: verify each item against the current code and config before acting on it. A memory that contradicts the repository is stale."

// Query selects which memories to retrieve.
type Query struct {
	// WorkspaceID is required. Retrieval never crosses workspaces.
	WorkspaceID string
	Text        string
	Kinds       []Kind
	// Statuses defaults to confirmed only. Proposed memories are candidates,
	// not knowledge, and returning them would make the gate meaningless.
	//
	// An as-of query defaults to confirmed and stale together, because a fact
	// that was true then is usually stale now, and excluding it would make
	// every as-of query answer with the present.
	Statuses []Status
	// AsOf asks what the workspace believed at an instant rather than what it
	// believes now. Zero means now.
	AsOf  time.Time
	Limit int
}

// Hit is one retrieved memory and its fused score. Higher is a better match.
type Hit struct {
	Record Record
	Score  float64
}

// rrfK damps the contribution of low-ranked results in reciprocal rank fusion.
// 60 is the value the method was published with, and it is not tuned here:
// picking weights per workspace is exactly the kind of knob that looks like
// tuning and behaves like noise.
const rrfK = 60.0

// candidateFactor is how many rows are pulled per requested result before
// fusing. Reranking can only reorder what it was given, so the keyword stage
// hands over more than the caller asked for.
const candidateFactor = 4

// Search finds relevant memories.
//
// With text, two rankings are fused: keyword relevance from bm25, and recency.
// Neither alone is right. Keyword rank alone puts a year-old note above the
// note that replaced it whenever the older wording matched better; recency
// alone ignores the question. Reciprocal rank fusion combines the orderings
// without needing a scale that makes bm25 and timestamps comparable.
//
// Keyword search with metadata filters remains the deliberate first choice.
// Embeddings come only after this is measured as insufficient, not before.
func (s *Store) Search(ctx context.Context, query Query) ([]Hit, error) {
	if query.WorkspaceID == "" {
		return nil, fmt.Errorf("search needs a workspaceId; memories never leak across workspaces")
	}
	if query.Limit <= 0 {
		query.Limit = DefaultLimit
	}
	if len(query.Statuses) == 0 {
		if query.AsOf.IsZero() {
			query.Statuses = []Status{StatusConfirmed}
		} else {
			query.Statuses = []Status{StatusConfirmed, StatusStale}
		}
	}

	text := strings.TrimSpace(query.Text)
	fetch := query.Limit
	if text != "" {
		fetch = query.Limit * candidateFactor
	}

	hits, err := s.candidates(ctx, query, text, fetch)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return hits, nil
	}
	return fuse(hits, query.Limit), nil
}

// candidates runs the SQL side: filters, then bm25 order when there is a query
// to rank against and recency order when there is not.
func (s *Store) candidates(ctx context.Context, query Query, text string, limit int) ([]Hit, error) {
	var (
		conditions = []string{"m.workspace_id = ?"}
		args       = []any{query.WorkspaceID}
		joins      string
		order      = "m.updated_at DESC"
		selectExpr = "0.0 AS score"
	)

	if text != "" {
		joins = "JOIN memories_fts f ON f.memory_id = m.id"
		conditions = append(conditions, "memories_fts MATCH ?")
		args = append(args, ftsQuery(text))
		selectExpr = "bm25(memories_fts) AS score"
		order = "score ASC"
	}

	conditions = append(conditions, inClause("m.status", len(query.Statuses)))
	for _, status := range query.Statuses {
		args = append(args, string(status))
	}
	if len(query.Kinds) > 0 {
		conditions = append(conditions, inClause("m.kind", len(query.Kinds)))
		for _, kind := range query.Kinds {
			args = append(args, string(kind))
		}
	}

	// Every timestamp compared here is fixed-width UTC RFC3339, so a string
	// comparison is a time comparison. Nanosecond precision would make the
	// strings variable width and break that.
	instant := query.AsOf
	if instant.IsZero() {
		instant = time.Now()
	}
	stamp := instant.UTC().Format(ExpiryLayout)

	// An expired memory is a fact whose known shelf life has passed.
	conditions = append(conditions, "(m.expires_at IS NULL OR m.expires_at > ?)")
	args = append(args, stamp)

	if query.AsOf.IsZero() {
		conditions = append(conditions, "m.valid_to IS NULL")
	} else {
		conditions = append(conditions, "m.valid_from <= ?", "(m.valid_to IS NULL OR m.valid_to > ?)")
		args = append(args, stamp, stamp)
	}

	// Assembled from constants: selectExpr, joins and order are literals chosen
	// above, and conditions is a list of literal fragments carrying "?" for
	// every value. Nothing a caller supplies reaches the statement text; it all
	// arrives through args.
	var statement strings.Builder
	statement.WriteString(`
        SELECT m.id, m.workspace_id, m.kind, m.content, m.tags, m.confidence,
               m.status, m.source_type, m.source_ref, m.evidence, m.supersedes_id,
               m.used_count, m.expires_at, m.valid_from, m.valid_to,
               m.created_at, m.updated_at, `)
	statement.WriteString(selectExpr)
	statement.WriteString(" FROM memories m ")
	statement.WriteString(joins)
	statement.WriteString(" WHERE ")
	statement.WriteString(strings.Join(conditions, " AND "))
	statement.WriteString(" ORDER BY ")
	statement.WriteString(order)
	statement.WriteString(" LIMIT ?")
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, statement.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var hits []Hit
	for rows.Next() {
		var score float64
		record, err := scanRecord(rows.Scan, &score)
		if err != nil {
			return nil, err
		}
		hits = append(hits, Hit{Record: record, Score: score})
	}
	return hits, rows.Err()
}

// fuse reranks keyword candidates by combining their relevance order with
// their recency order, and returns the best limit of them.
func fuse(hits []Hit, limit int) []Hit {
	if len(hits) <= 1 {
		return hits
	}

	// hits arrive in bm25 order, so position is the relevance rank.
	fused := make([]Hit, len(hits))
	copy(fused, hits)

	byRecency := make([]int, len(hits))
	for i := range byRecency {
		byRecency[i] = i
	}
	sort.SliceStable(byRecency, func(a, b int) bool {
		return hits[byRecency[a]].Record.UpdatedAt.After(hits[byRecency[b]].Record.UpdatedAt)
	})

	score := make([]float64, len(hits))
	for rank, index := range byRecency {
		score[index] += 1 / (rrfK + float64(rank))
	}
	for rank := range hits {
		score[rank] += 1 / (rrfK + float64(rank))
	}
	for i := range fused {
		fused[i].Score = score[i]
	}

	// Ties are common and not a rounding artifact: two results that swap places
	// between the two rankings score identically, which is what reciprocal rank
	// fusion is supposed to do. Something still has to break it, and for a
	// memory store the newer fact is the right answer, because the usual reason
	// two memories say almost the same thing is that one replaced the other.
	sort.SliceStable(fused, func(a, b int) bool {
		left, right := fused[a], fused[b]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		if !left.Record.UpdatedAt.Equal(right.Record.UpdatedAt) {
			return left.Record.UpdatedAt.After(right.Record.UpdatedAt)
		}
		return left.Record.Confidence > right.Record.Confidence
	})
	if len(fused) > limit {
		fused = fused[:limit]
	}
	return fused
}

// ftsQuery turns free text into an FTS5 expression, quoting each term so
// punctuation in a phrase like "localhost:6379" cannot be read as syntax.
func ftsQuery(text string) string {
	fields := strings.Fields(text)
	quoted := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ReplaceAll(field, `"`, "")
		if field == "" {
			continue
		}
		quoted = append(quoted, `"`+field+`"`)
	}
	if len(quoted) == 0 {
		return `""`
	}
	return strings.Join(quoted, " OR ")
}

func inClause(column string, count int) string {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", count), ",")
	return fmt.Sprintf("%s IN (%s)", column, placeholders)
}
