package memory

import (
	"context"
	"database/sql"
	"fmt"
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
	Statuses []Status
	Limit    int
}

// Hit is one retrieved memory and its relevance.
type Hit struct {
	Record Record
	// Score is bm25 rank, lower is a better match. Zero when no text was given.
	Score float64
}

// Search finds relevant memories, ranked by bm25 when text is supplied.
//
// Keyword search with metadata filters is the deliberate first choice.
// Embeddings come only after keyword search is measured as insufficient, not
// before.
func (s *Store) Search(ctx context.Context, query Query) ([]Hit, error) {
	if query.WorkspaceID == "" {
		return nil, fmt.Errorf("search needs a workspaceId; memories never leak across workspaces")
	}
	if query.Limit <= 0 {
		query.Limit = DefaultLimit
	}
	if len(query.Statuses) == 0 {
		query.Statuses = []Status{StatusConfirmed}
	}

	var (
		conditions = []string{"m.workspace_id = ?"}
		args       = []any{query.WorkspaceID}
		joins      string
		order      = "m.updated_at DESC"
		selectExpr = "0.0 AS score"
	)

	if text := strings.TrimSpace(query.Text); text != "" {
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
	// An expired memory is a fact whose known shelf life has passed. Both sides
	// are fixed-width UTC RFC3339, so a string comparison is a time comparison.
	conditions = append(conditions, "(m.expires_at IS NULL OR m.expires_at > ?)")
	args = append(args, time.Now().UTC().Format(ExpiryLayout))

	statement := fmt.Sprintf(`
        SELECT m.id, m.workspace_id, m.kind, m.content, m.tags, m.confidence,
               m.status, m.source_type, m.source_ref, m.evidence, m.supersedes_id,
               m.used_count, m.expires_at, m.created_at, m.updated_at, %s
        FROM memories m %s
        WHERE %s
        ORDER BY %s
        LIMIT ?`,
		selectExpr, joins, strings.Join(conditions, " AND "), order)
	args = append(args, query.Limit)

	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var hits []Hit
	for rows.Next() {
		var (
			record       Record
			tags         string
			sourceRef    sql.NullString
			evidence     string
			supersedesID sql.NullString
			expiresAt    sql.NullString
			createdAt    string
			updatedAt    string
			kind         string
			status       string
			sourceType   string
			score        float64
		)
		if err := rows.Scan(&record.ID, &record.WorkspaceID, &kind, &record.Content,
			&tags, &record.Confidence, &status, &sourceType, &sourceRef, &evidence,
			&supersedesID, &record.UsedCount, &expiresAt, &createdAt, &updatedAt, &score,
		); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		record.Kind = Kind(kind)
		record.Status = Status(status)
		record.SourceType = SourceType(sourceType)
		record.SourceRef = sourceRef.String
		record.SupersedesID = supersedesID.String
		if tags != "" {
			record.Tags = strings.Fields(tags)
		}
		if evidence != "" {
			record.Evidence = strings.Split(evidence, "\n")
		}
		record.ExpiresAt = parseTimePtr(expiresAt)
		record.CreatedAt = parseTime(createdAt)
		record.UpdatedAt = parseTime(updatedAt)
		hits = append(hits, Hit{Record: record, Score: score})
	}
	return hits, rows.Err()
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
