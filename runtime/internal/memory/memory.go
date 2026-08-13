// Package memory assembles the memory module and is the only place that knows
// all of its parts.
//
// Three rules shape the module, and they live in domain where they belong:
// memory is supporting context and never authority, the model may propose while
// only a verifier result or a human confirms, and procedural memory is refused
// because a workflow worth keeping is promoted to a reviewable asset instead.
package memory

import (
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/memory/infra/persistence"
)

// The names a caller uses. One definition each, in the layer that owns it.
type (
	Record     = domain.Record
	Kind       = domain.Kind
	Status     = domain.Status
	SourceType = domain.SourceType
	Decision   = domain.Decision
	Verdict    = domain.Verdict
	Promotion  = domain.Promotion
	Store      = persistence.Store
	Query      = persistence.Query
	Hit        = persistence.Hit
)

const (
	KindSemantic   = domain.KindSemantic
	KindEpisodic   = domain.KindEpisodic
	KindCorrection = domain.KindCorrection
	KindPreference = domain.KindPreference
	KindProcedural = domain.KindProcedural

	StatusProposed  = domain.StatusProposed
	StatusConfirmed = domain.StatusConfirmed
	StatusStale     = domain.StatusStale
	StatusRejected  = domain.StatusRejected

	SourceCommandResult  = domain.SourceCommandResult
	SourceFileContent    = domain.SourceFileContent
	SourceCIAPI          = domain.SourceCIAPI
	SourceHumanStatement = domain.SourceHumanStatement
	SourceReviewComment  = domain.SourceReviewComment

	VerdictStore  = domain.VerdictStore
	VerdictMerge  = domain.VerdictMerge
	VerdictReject = domain.VerdictReject

	PromotionThreshold = domain.PromotionThreshold
	ExpiryLayout       = persistence.ExpiryLayout
	Disclaimer         = persistence.Disclaimer
	DefaultLimit       = persistence.DefaultLimit
)

// ProposePromotions finds memories reused often enough to belong in a reviewed
// asset rather than in a store nobody reads.
func ProposePromotions(records []Record) []Promotion { return domain.ProposePromotions(records) }

// DBPath is where a workspace keeps its memory database.
func DBPath(workspaceRoot string) string { return persistence.DBPath(workspaceRoot) }

// Open creates or opens the store for a workspace; OpenAt takes an explicit path.
var (
	Open   = persistence.Open
	OpenAt = persistence.OpenAt
)
