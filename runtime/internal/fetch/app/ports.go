// Package app holds the fetch use cases and the ports they depend on.
//
// The ports are declared here, by the consumer, and implemented under infra.
// That is the direction that matters: the use case says what it needs, and the
// HTTP client, the extractor, and the cache each satisfy one of those needs
// without the use case knowing which of them it got.
package app

import (
	"context"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/domain"
)

// Source retrieves the bytes behind a URL or a path, and says what they are.
//
// The media type is returned rather than inferred by the caller, because only
// the thing that made the request knows what the server declared, and only the
// thing that read the file can sniff it.
type Source interface {
	Read(ctx context.Context, source string) (raw []byte, contentType string, err error)
}

// Extractor turns a page into the text a reader wants, keeping the author's own
// words. It never summarizes: a paraphrase would be model output wearing a
// source's authority.
type Extractor interface {
	Extract(raw []byte, pageURL string) domain.Document
}

// Guard reports that a response is not the page that was asked for.
//
// A bot check answers 200 with prose, so nothing upstream can tell it from an
// article. Returning a reason rather than a bool keeps the refusal able to name
// what it matched.
type Guard interface {
	ChallengeReason(raw []byte, doc domain.Document) string
	Advise(ctx context.Context, pageURL, problem string) string
}

// Store is the workspace-local cache of documents already fetched.
//
// Load reports a miss rather than an error for an absent or expired entry: a
// cold cache is the normal state, not a failure.
type Store interface {
	Load(source string) (domain.Document, bool)
	Save(source string, doc domain.Document) error
}

// Assets keeps a non-text source where a file reader can open it, and describes
// where it went. The bytes never enter a context window; the path does.
type Assets interface {
	Keep(source, contentType string, raw []byte) (domain.Document, error)
	IsText(contentType string) bool
}
