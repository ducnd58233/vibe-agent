package httpx

import (
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/fetch/infra/extract"
)

// The detector has to be narrow. A page about Cloudflare, or one that uses the
// word "verifying", is still a page.
func TestOrdinaryProseIsNotMistakenForAChallenge(t *testing.T) {
	cases := []string{
		`<html><head><title>Verifying webhook signatures</title></head><body><main>
<p>Verifying you are human is what a CAPTCHA does. This guide explains how to
verify a webhook signature instead, with a shared secret and a timing-safe
comparison. Cloudflare is one provider that signs its webhooks this way.</p>
</main></body></html>`,
		`<html><head><title>Just a moment in time: date handling</title></head><body><main>
<p>Timestamps are the usual source of flaky tests. This page covers clock
skew, monotonic time, and why wall-clock comparisons drift.</p></main></body></html>`,
	}
	for _, page := range cases {
		if reason := challengeReason([]byte(page), extract.ExtractHTML([]byte(page))); reason != "" {
			t.Errorf("ordinary prose flagged as %q:\n%s", reason, page[:80])
		}
	}
}
