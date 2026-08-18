package app

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if strings.Contains(body, `data-testid="file-preview"`) {
		t.Fatal("listing a file must not open the reader pane")
	}
}
