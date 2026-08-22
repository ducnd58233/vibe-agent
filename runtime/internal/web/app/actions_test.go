package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/runpath"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
)

func TestCheckSlugReportsExactMatch(t *testing.T) {
	root, slug := writeFixtureSession(t)
	handler, err := NewHandlerWithPort(root, testutil.ToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/check-slug?slug="+slug, nil)
	handler.ServeHTTP(rec, req)
	var got slugExistsResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Exists {
		t.Fatalf("exists = false for the fixture's own slug %q", slug)
	}
}

func TestCheckSlugReportsCaseCollision(t *testing.T) {
	root := t.TempDir()
	if _, err := runpath.Allocate(root, "MyFeature", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testutil.ToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/check-slug?slug=myfeature", nil)
	handler.ServeHTTP(rec, req)
	var got slugExistsResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Exists {
		t.Fatal("exists = false for a slug differing only in case from MyFeature")
	}
}

func TestCheckSlugReportsFreeSlug(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testutil.ToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/session/check-slug?slug=brand-new", nil)
	handler.ServeHTTP(rec, req)
	var got slugExistsResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Exists {
		t.Fatal("exists = true for a slug nothing has taken")
	}
}
