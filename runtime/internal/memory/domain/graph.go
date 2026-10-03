package domain

import (
	"fmt"
	"time"
)

// Relation is how one memory relates to another. Typed edges let retrieval pull
// in a hit's neighbours, the idea behind Zettelkasten-style agent memory,
// without an embedding model deciding what is "similar".
type Relation string

const (
	// RelationSupersedes: the source replaced the destination.
	RelationSupersedes Relation = "supersedes"
	// RelationRelatesTo is a symmetric association.
	RelationRelatesTo Relation = "relates_to"
	// RelationDerivedFrom: the source was concluded from the destination.
	RelationDerivedFrom Relation = "derived_from"
	// RelationContradicts: the two cannot both be true. Retrieval never expands
	// across it; it exists so a reviewer can find the pair.
	RelationContradicts Relation = "contradicts"
)

func (r Relation) Valid() bool {
	switch r {
	case RelationSupersedes, RelationRelatesTo, RelationDerivedFrom, RelationContradicts:
		return true
	}
	return false
}

// Expands reports whether retrieval follows this relation to a neighbour.
func (r Relation) Expands() bool {
	return r == RelationRelatesTo || r == RelationDerivedFrom
}

// Link is one typed edge between two memories.
type Link struct {
	SrcID     string    `json:"srcId"`
	DstID     string    `json:"dstId"`
	Relation  Relation  `json:"relation"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Validate reports whether a link is well formed.
func (l Link) Validate() error {
	if !l.Relation.Valid() {
		return fmt.Errorf("relation %q is not one of supersedes, relates_to, derived_from, contradicts", l.Relation)
	}
	if l.SrcID == "" || l.DstID == "" {
		return fmt.Errorf("a link needs both memory ids")
	}
	if l.SrcID == l.DstID {
		return fmt.Errorf("a memory cannot link to itself")
	}
	return nil
}

// LedgerEntry is one line of a memory's append-only history. Rows in memories
// are edited in place; this is what survives the edits, so how a record got its
// current status can still be answered after the fact.
type LedgerEntry struct {
	ID         int64     `json:"id"`
	MemoryID   string    `json:"memoryId"`
	Action     string    `json:"action"`
	FromStatus string    `json:"fromStatus,omitempty"`
	ToStatus   string    `json:"toStatus,omitempty"`
	Actor      string    `json:"actor,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	At         time.Time `json:"at"`
}

// SessionHit is a past conversation turn or tool call matching a query: the
// episodic layer, searched by words rather than replayed in order.
type SessionHit struct {
	Scope    string    `json:"scope"`
	Sequence int       `json:"sequence"`
	Type     string    `json:"type"`
	At       time.Time `json:"at"`
	Snippet  string    `json:"snippet"`
}
