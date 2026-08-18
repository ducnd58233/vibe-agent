package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func TestWorkspaceSwitchSetsCookieAndRedirects(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	reg := domain.NewRegistry(rootA, []string{rootB})
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	idB := reg.ID(rootB)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/switch", strings.NewReader("workspace_id="+idB))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("location = %q", loc)
	}
	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == workspaceCookie && c.Value == idB {
			found = true
		}
	}
	if !found {
		t.Fatal("workspace cookie not set")
	}
}

func TestActiveWorkspaceCookieChangesSessionList(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	slug := "ws-b-session"
	if err := os.MkdirAll(filepath.Join(rootB, "tmp", slug), 0o750); err != nil {
		t.Fatal(err)
	}
	run, err := state.NewRun(slug, "goal", "goal-delivery", 50, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Save(state.ManifestPath(rootB, slug), run); err != nil {
		t.Fatal(err)
	}
	reg := domain.NewRegistry(rootA, []string{rootB})
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	//nolint:gosec // G124 test cookie mirrors loopback server flags
	req.AddCookie(&http.Cookie{
		Name:     workspaceCookie,
		Value:    reg.ID(rootB),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, slug) {
		t.Fatalf("session from workspace B missing: %s", body)
	}
	if !strings.Contains(body, `href="/session/`+slug+`"`) {
		t.Fatal("session list should link to the session page")
	}
	if !strings.Contains(body, `data-testid="workspace-list"`) {
		t.Fatal("expected workspace list")
	}
}

func TestWorkspaceSwitchIgnoresReturnSlug(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	reg := domain.NewRegistry(rootA, []string{rootB})
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	body := "workspace_id=" + reg.ID(rootB) + "&return_slug=missing-session"
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("location = %q, want /", loc)
	}
}
