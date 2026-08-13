// Package domain holds what a fetched document is, and what a caller may
// conclude from one.
//
// No I/O and no imports outside the standard library: the rules here are about
// the shape of a result, not about how it was obtained. That is what lets the
// HTTP client, the extractor, and the cache all be replaced without touching
// them.
package domain

import "strings"

// Status is what a caller needs to know before trusting Text.
//
// A field rather than prose, because the caller that most needs this is a
// program deciding whether to answer from the document. Prose on stderr is for
// the person; this is for the branch.
type Status string

const (
	// StatusOK means the page carried prose and nothing suggests it is not the
	// page that was asked for.
	StatusOK Status = "ok"
	// StatusEmpty means the page parsed and held no readable text at all. Almost
	// always a client-rendered shell.
	StatusEmpty Status = "empty"
	// StatusThin means there is text, and too little of it relative to the
	// markup and script around it to be the content. A navigation bar and a
	// footer with nothing between them lands here.
	StatusThin Status = "thin"
	// StatusAsset means the source was not text and is now a file on disk.
	StatusAsset Status = "asset"
)

// Document is extracted content and where it came from.
type Document struct {
	Source string `json:"source"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text"`
	// OriginalBytes is what arrived before extraction, so the saving can be
	// reported rather than claimed.
	OriginalBytes int `json:"originalBytes"`
	// Status is what a caller must check before trusting Text.
	Status Status `json:"status"`
	// LocalPath is where a non-text source was put, for the caller's own file
	// reader to open. Empty for anything that became text.
	LocalPath   string `json:"localPath,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	// Empty marks a page that parsed cleanly and carried no prose.
	Empty bool `json:"empty,omitempty"`
}

// ThinTextRatio is the share of a page's bytes that has to survive extraction
// before the result is treated as the content.
//
// Derived from the shape of the failure rather than tuned: a real article keeps
// a few percent of a heavy page, and a shell keeps a fraction of one.
const ThinTextRatio = 0.005

// MinRealChars is the floor below which a ratio means nothing. A small page that
// is genuinely short must not be called thin because its markup is efficient.
const MinRealChars = 400

// Classify decides what a caller may do with an extracted document.
func Classify(text string, originalBytes int) Status {
	if strings.TrimSpace(text) == "" {
		return StatusEmpty
	}
	if len(text) >= MinRealChars {
		return StatusOK
	}
	if originalBytes > 0 && float64(len(text))/float64(originalBytes) < ThinTextRatio {
		return StatusThin
	}
	return StatusOK
}
