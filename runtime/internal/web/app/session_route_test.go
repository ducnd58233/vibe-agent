package app

import (
	"context"
	"net/http"
	"testing"
)

func TestParseSessionSubpath(t *testing.T) {
	slug, ok := parseSessionSubpath("/session/my-slug/events", "events")
	if !ok || slug != "my-slug" {
		t.Fatalf("parseSessionSubpath events = %q, %v", slug, ok)
	}
	_, ok = parseSessionSubpath("/session/my-slug/send", "events")
	if ok {
		t.Fatal("expected mismatch on action")
	}
}

func TestParseSessionSuffixPath(t *testing.T) {
	slug, ok := parseSessionSuffixPath("/session/abc/events/stream", "/events/stream")
	if !ok || slug != "abc" {
		t.Fatalf("parseSessionSuffixPath = %q, %v", slug, ok)
	}
}

func TestParseAfterQuery(t *testing.T) {
	ctx := context.Background()
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/?after=3", nil)
	after, ok := parseAfterQuery(r)
	if !ok || after != 3 {
		t.Fatalf("after = %d, ok = %v", after, ok)
	}
	r, _ = http.NewRequestWithContext(ctx, http.MethodGet, "/?after=-1", nil)
	if _, ok := parseAfterQuery(r); ok {
		t.Fatal("expected bad after")
	}
}
