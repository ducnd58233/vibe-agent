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
)

func TestWorkspaceFilesRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/files?dir=..%2F..", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWorkspaceFilePreviewRedactsSecret(t *testing.T) {
	root := t.TempDir()
	secretPath := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("token="+testSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/files/preview?path=secret.txt", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, testSecret) {
		t.Fatalf("secret leaked in preview: %s", body)
	}
	if !strings.Contains(body, `data-testid="file-preview"`) {
		t.Fatalf("missing preview: %s", body)
	}
}

func TestWorkspaceFilesExposeAttachInsertAndClose(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o750); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/files", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="file-browser-close"`) {
		t.Fatalf("file picker must be dismissible without choosing a file: %s", body)
	}
	if !strings.Contains(body, `data-insert="@note.md"`) {
		t.Fatalf("file row must insert @path into the composer, got %s", body)
	}
	if !strings.Contains(body, `data-insert="@pkg"`) {
		t.Fatalf("folder row must insert @path into the composer, got %s", body)
	}
	if !strings.Contains(body, `data-testid="file-attach-folder"`) {
		t.Fatalf("attach picker must offer attach-this-folder, got %s", body)
	}
	if strings.Contains(body, `data-testid="file-preview"`) {
		t.Fatal("listing a file must not open the reader pane")
	}
}

func TestWorkspaceBrowseRejectsRelativeAndDotDot(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"relative", "..", "../outside"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/browse?dir="+url.QueryEscape(dir), nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("dir %q status = %d", dir, rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "no such file") || strings.Contains(strings.ToLower(body), "stat ") {
			t.Fatalf("os error leaked for dir %q: %s", dir, body)
		}
	}
}

func TestWorkspaceBrowseEmptyListsHome(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/browse", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="file-browser"`) {
		t.Fatalf("missing browser: %s", body)
	}
	if !strings.Contains(body, `data-mode="open"`) {
		t.Fatalf("browse listing must be open mode: %s", body)
	}
	if !strings.Contains(body, filepath.ToSlash(home)) && !strings.Contains(body, home) {
		t.Fatalf("roots view must include home path")
	}
	if strings.Contains(body, `data-testid="file-preview"`) || strings.Contains(body, "file-preview-excerpt") {
		t.Fatal("browse listing must not include file contents")
	}
}

func TestWorkspaceBrowseListsTempDirWithoutExcerpt(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "src")
	if err := os.Mkdir(child, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("token="+testSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/browse?dir="+url.QueryEscape(root), nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "src") {
		t.Fatalf("missing child dir: %s", body)
	}
	if !strings.Contains(body, "secret.txt") {
		t.Fatalf("missing file name: %s", body)
	}
	if strings.Contains(body, testSecret) {
		t.Fatal("file body leaked into browse listing")
	}
	if !strings.Contains(body, `data-testid="file-open-folder"`) {
		t.Fatalf("open-this-folder missing: %s", body)
	}
}
