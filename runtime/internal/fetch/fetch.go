// Package fetch assembles the document-fetching module and is the only place
// that knows all of its parts.
//
// The layers below are independent: domain states what a document is, app
// declares the ports and the order of steps, and infra implements the ports
// over HTTP, an HTML extractor, and the workspace cache. Nothing there imports
// anything here. This file is where they meet, so a caller has one entry point
// and the wiring lives in one readable place.
package fetch

import (
	"context"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/app/usecases"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/extract"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/httpx"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/persistence"
)

// The names a caller uses. One definition each, in the layer that owns it.
type (
	Document = domain.Document
	Status   = domain.Status
	Options  = usecases.Options
)

const (
	StatusOK    = domain.StatusOK
	StatusEmpty = domain.StatusEmpty
	StatusThin  = domain.StatusThin
	StatusAsset = domain.StatusAsset
)

// CacheDir is where this module writes, exposed for the messages that tell a
// reader where a document went.
func CacheDir(workspaceRoot string) string { return persistence.CacheDir(workspaceRoot) }

// Advise turns a dead end into a next step: a browser-driving tool, or a text
// route the origin publishes itself.
func Advise(ctx context.Context, pageURL, problem string) string {
	return httpx.Advise(ctx, pageURL, problem)
}

// fetcher wires the ports to their implementations for one workspace.
func fetcher(workspaceRoot string) usecases.Fetcher {
	return usecases.Fetcher{
		Source:    httpx.Client{},
		Extractor: extract.HTML{},
		Guard:     httpx.Client{},
		Store:     persistence.Store{Root: workspaceRoot},
		Assets:    persistence.Assets{Root: workspaceRoot},
	}
}

// Get retrieves a source, extracts its readable text, and caches the result.
//
// The second return value reports whether the cache answered, so a caller can
// say so rather than implying a request happened.
func Get(ctx context.Context, workspaceRoot, source string, options Options) (Document, bool, error) {
	return fetcher(workspaceRoot).Get(ctx, strings.TrimSpace(source), options)
}

// CharsPerToken is the ratio budgets are estimated with. An approximation, and
// named as one: the consequence of it being off is a clip slightly early or
// late, not a wrong document.
const CharsPerToken = 4

// EstimateTokens approximates what a string costs to read.
func EstimateTokens(text string) int {
	return (len(text) + CharsPerToken - 1) / CharsPerToken
}

// Clip cuts text to a token budget at a line boundary, and reports what it left.
//
// The remainder is a number rather than nothing: an agent told that 400 lines
// remain asks for them, and one told nothing assumes it read the page.
func Clip(text string, budget int) (clipped string, omittedLines int) {
	if budget <= 0 || EstimateTokens(text) <= budget {
		return text, 0
	}
	head := text[:budget*CharsPerToken]
	if cut := strings.LastIndexByte(head, '\n'); cut > 0 {
		head = head[:cut]
	}
	return head, strings.Count(text[len(head):], "\n") + 1
}

// DefaultTimeout is how long one retrieval may take.
const DefaultTimeout = httpx.DefaultTimeout

// CacheLife is how long a fetched document is served without asking again.
const CacheLife = persistence.CacheLife

var _ = time.Second // keep the timeout constants readable as durations
