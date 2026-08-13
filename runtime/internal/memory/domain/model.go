// Package memory stores what a run learned, so the next run does not rediscover
// it.
//
// Three rules shape this package:
//
//   - Memory is supporting context, never authority. The repository is the
//     source of truth; a memory that contradicts the code is stale.
//   - The model may propose. Only a verifier result or a human confirms.
//   - Procedural memory does not belong here. A workflow worth keeping is
//     promoted to a skill, a rule, a stack profile, or a deterministic check,
//     where it is reviewable and versioned.
package domain

import (
	"fmt"
	"time"
)

// Kind separates memories that need different retrieval. Mixing episodic logs
// into a semantic index degrades both.
type Kind string

const (
	// KindSemantic is a fact about the codebase or domain.
	KindSemantic Kind = "semantic"
	// KindEpisodic is what happened during a run: what failed, what worked.
	KindEpisodic Kind = "episodic"
	// KindCorrection is a human overriding something the agent got wrong.
	KindCorrection Kind = "correction"
	// KindPreference is how this person or team wants work done.
	KindPreference Kind = "preference"
)

// KindProcedural is refused on purpose. It exists as a named constant so the
// rejection message can be specific rather than "unknown kind".
const KindProcedural Kind = "procedural"

func (k Kind) Valid() bool {
	switch k {
	case KindSemantic, KindEpisodic, KindCorrection, KindPreference:
		return true
	}
	return false
}

// Status is how far a memory has got through the write policy.
type Status string

const (
	// StatusProposed is the only status model output can produce.
	StatusProposed Status = "proposed"
	// StatusConfirmed requires a verifier result or a human event.
	StatusConfirmed Status = "confirmed"
	// StatusStale is superseded or contradicted by the repository.
	StatusStale Status = "stale"
	// StatusRejected failed the policy filter and is kept for the record.
	StatusRejected Status = "rejected"
)

func (s Status) Valid() bool {
	switch s {
	case StatusProposed, StatusConfirmed, StatusStale, StatusRejected:
		return true
	}
	return false
}

// SourceType is where a memory came from. Every value is outside the model.
type SourceType string

const (
	SourceCommandResult  SourceType = "command_result"
	SourceFileContent    SourceType = "file_content"
	SourceCIAPI          SourceType = "ci_api"
	SourceHumanStatement SourceType = "human_statement"
	SourceReviewComment  SourceType = "review_comment"
)

func (s SourceType) Valid() bool {
	switch s {
	case SourceCommandResult, SourceFileContent, SourceCIAPI,
		SourceHumanStatement, SourceReviewComment:
		return true
	}
	return false
}

// Record is one memory.
type Record struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	Kind        Kind       `json:"kind"`
	Content     string     `json:"content"`
	Tags        []string   `json:"tags,omitempty"`
	Confidence  float64    `json:"confidence"`
	Status      Status     `json:"status"`
	SourceType  SourceType `json:"sourceType"`
	SourceRef   string     `json:"sourceRef,omitempty"`
	// Evidence is concrete observations. A memory without any is a guess, and
	// the policy filter rejects it.
	Evidence     []string   `json:"evidence"`
	SupersedesID string     `json:"supersedesId,omitempty"`
	UsedCount    int        `json:"usedCount"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`

	// ValidFrom and ValidTo are when the fact was true, which is a different
	// question from CreatedAt and UpdatedAt, when this store learned about it.
	//
	// Keeping both is what lets a contradiction be resolved without losing the
	// history: the replaced fact is closed at the moment the replacement became
	// true, so it stops being retrieved while the record of having believed it
	// survives, and an as-of query can still reconstruct what was known then.
	//
	// A nil ValidTo means the fact is still held.
	ValidFrom time.Time  `json:"validFrom"`
	ValidTo   *time.Time `json:"validTo,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Validate reports whether a record is well formed, independent of policy.
func (r Record) Validate() error {
	if r.WorkspaceID == "" {
		return fmt.Errorf("workspaceId is required; memories never leak across workspaces")
	}
	if r.Kind == KindProcedural {
		return fmt.Errorf("procedural memory is not stored here: promote the workflow to a SKILL.md, an AGENTS.md rule, a stack profile, or a deterministic check")
	}
	if !r.Kind.Valid() {
		return fmt.Errorf("kind %q is not one of semantic, episodic, correction, preference", r.Kind)
	}
	if r.Content == "" {
		return fmt.Errorf("content must not be empty")
	}
	if len(r.Content) > 2000 {
		return fmt.Errorf("content is %d characters; keep a memory to one fact under 2000", len(r.Content))
	}
	if !r.Status.Valid() {
		return fmt.Errorf("status %q is not known", r.Status)
	}
	if !r.SourceType.Valid() {
		return fmt.Errorf("sourceType %q is not evidence; model inference is not a source", r.SourceType)
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("confidence %v is outside 0..1", r.Confidence)
	}
	if len(r.Evidence) == 0 {
		return fmt.Errorf("a memory needs at least one piece of evidence")
	}
	return nil
}
