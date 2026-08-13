// Package usecases holds what fetching a document actually does, in the order
// it does it, with every dependency arriving as a port.
package usecases

import (
	"context"
	"fmt"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/app"
	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
)

// MaxDocumentBytes is the largest source read as text.
//
// Past this it is a download, not a document, and pulling it into memory to
// extract three paragraphs is the failure the cap exists to prevent. Assets are
// held to a larger limit by the port that keeps them, because none of an asset
// enters a context window.
const MaxDocumentBytes = 8 << 20

// Options adjusts one retrieval.
type Options struct {
	// Refresh ignores any cached copy and asks the source again.
	Refresh bool
}

// Fetcher retrieves a document, from cache where it can and from the source
// where it must.
type Fetcher struct {
	Source    app.Source
	Extractor app.Extractor
	Guard     app.Guard
	Store     app.Store
	Assets    app.Assets
}

// Get returns a document and whether the cache answered.
//
// The second return value exists so a caller can say "no request was made"
// rather than implying one happened.
func (f Fetcher) Get(ctx context.Context, source string, options Options) (domain.Document, bool, error) {
	if source == "" {
		return domain.Document{}, false, fmt.Errorf("fetch needs a URL or a file path")
	}

	if !options.Refresh {
		if cached, ok := f.Store.Load(source); ok {
			return cached, true, nil
		}
	}

	raw, contentType, err := f.Source.Read(ctx, source)
	if err != nil {
		return domain.Document{}, false, err
	}

	// Not text: keep the bytes out of context and hand back a path. Refusing was
	// the earlier behaviour and it left the runtime unable to help with an
	// illustration, a screenshot, a spec PDF, or a demo video.
	if !f.Assets.IsText(contentType) {
		doc, err := f.Assets.Keep(source, contentType, raw)
		if err != nil {
			return domain.Document{}, false, err
		}
		return f.remember(source, doc)
	}

	if len(raw) > MaxDocumentBytes {
		return domain.Document{}, false, fmt.Errorf(
			"%s is %d bytes of text, over the %d-byte limit for a document",
			source, len(raw), MaxDocumentBytes)
	}

	doc := f.Extractor.Extract(raw, source)
	doc.Source = source

	// A bot wall answers 200 with a page that reads like prose, so nothing above
	// this line can tell it from the article. Refusing is the whole response:
	// letting an agent read "Verifying you are human" as its answer is the
	// failure that cannot be recovered from.
	if reason := f.Guard.ChallengeReason(raw, doc); reason != "" {
		return domain.Document{}, false, fmt.Errorf("fetch %s: %s",
			source, f.Guard.Advise(ctx, source, "answered with a bot check, matching "+reason))
	}

	return f.remember(source, doc)
}

// remember stores a document, and treats a failed write as a lost saving rather
// than a failed fetch: the document is already in hand, and turning a disk
// problem into a missing answer helps nobody.
func (f Fetcher) remember(source string, doc domain.Document) (domain.Document, bool, error) {
	_ = f.Store.Save(source, doc)
	return doc, false, nil
}
