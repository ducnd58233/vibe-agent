package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestShellShowsCurrentWorkspaceOnlyInRail(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	reg := domain.NewRegistry(rootA, []string{rootB})
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="workspace-current"`) {
		t.Fatal("expected current workspace chip")
	}
	if !strings.Contains(body, `data-testid="workspace-browse-dialog"`) {
		t.Fatal("expected browse workspace dialog")
	}
	if !strings.Contains(body, `data-testid="workspace-browse-panel"`) {
		t.Fatal("browse dialog must host the folder explorer")
	}
	labelA := filepath.Base(rootA)
	labelB := filepath.Base(rootB)
	currentIdx := strings.Index(body, `data-testid="workspace-current"`)
	listIdx := strings.Index(body, `data-testid="workspace-list"`)
	if currentIdx < 0 || listIdx < 0 || currentIdx > listIdx {
		t.Fatal("current chip should render before the recent list")
	}
	currentChunk := body[currentIdx:listIdx]
	if !strings.Contains(currentChunk, labelA) {
		t.Fatalf("current chip missing %q", labelA)
	}
	if strings.Contains(currentChunk, `data-testid="workspace-switch"`) {
		t.Fatal("rail current slot must not list switch buttons")
	}
	if !strings.Contains(body[listIdx:], labelB) {
		t.Fatalf("recent list missing %q", labelB)
	}
	if strings.Count(body, `data-testid="workspace-switch"`) != 1 {
		t.Fatalf("switch buttons = %d, want 1 recent workspace", strings.Count(body, `data-testid="workspace-switch"`))
	}
}

func TestWorkspaceOpenRegistersDirectoryAndSwitches(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	reg := domain.NewRegistry(rootA, nil)
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	form := strings.NewReader(url.Values{"path": {rootB}}.Encode())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/open", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	idB := domain.NewRegistry(rootA, []string{rootB}).ID(rootB)
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == workspaceCookie && c.Value == idB {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("open should set workspace cookie")
	}
	pageRec := httptest.NewRecorder()
	pageReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	pageReq.AddCookie(cookie)
	handler.ServeHTTP(pageRec, pageReq)
	body := pageRec.Body.String()
	currentIdx := strings.Index(body, `data-testid="workspace-current"`)
	listIdx := strings.Index(body, `data-testid="workspace-list"`)
	if currentIdx < 0 || listIdx < 0 {
		t.Fatal("expected current chip and recent list after open")
	}
	if !strings.Contains(body[currentIdx:listIdx], filepath.Base(rootB)) {
		t.Fatalf("opened workspace missing from current chip:\n%s", body)
	}
}

func TestWorkspaceOpenRejectsMissingPath(t *testing.T) {
	rootA := t.TempDir()
	reg := domain.NewRegistry(rootA, nil)
	handler, err := NewHandlerWithRegistry(reg, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	form := strings.NewReader(url.Values{"path": {filepath.Join(rootA, "no-such-dir")}}.Encode())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/open", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
