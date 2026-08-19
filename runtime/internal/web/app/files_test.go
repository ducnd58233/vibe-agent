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

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
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

func TestWorkspaceFileViewResolvesBareFilenameFromAiAgentsReferences(t *testing.T) {
	root := t.TempDir()
	refDir := filepath.Join(root, ".ai-agents", "references")
	if err := os.MkdirAll(refDir, 0o750); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(refDir, "loop-and-graph-engineering.md")
	if err := os.WriteFile(filePath, []byte("# Title\n\nhello"), 0o600); err != nil {
		t.Fatal(err)
	}

	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/files/view?path=loop-and-graph-engineering.md", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="file-viewer"`) {
		t.Fatalf("missing file viewer: %s", body)
	}
	if !strings.Contains(body, "<h1") {
		t.Fatalf("markdown not rendered: %s", body)
	}
}

func TestWorkspaceFileViewRejectsMissingBareFilename(t *testing.T) {
	root := t.TempDir()
	handler, err := NewHandlerWithPort(root, testToolkitRoot(t), 3080)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/files/view?path=missing.md", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceFileViewFallsBackToToolkitRootForAiAgentsReferences(t *testing.T) {
	workspaceRoot := t.TempDir()
	toolkitRoot := t.TempDir()

	refDir := filepath.Join(toolkitRoot, ".ai-agents", "references")
	if err := os.MkdirAll(refDir, 0o750); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(refDir, "loop-and-graph-engineering.md")
	if err := os.WriteFile(filePath, []byte("# Title\n\nhello"), 0o600); err != nil {
		t.Fatal(err)
	}

	handler, err := NewHandlerWithPort(workspaceRoot, toolkitRoot, 3080)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	rel := ".ai-agents/references/loop-and-graph-engineering.md"
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/workspace/files/view?path="+url.QueryEscape(rel),
		nil,
	)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, `data-testid="file-viewer"`) {
		t.Fatalf("missing file viewer: %s", body)
	}
	if !strings.Contains(body, "<h1") {
		t.Fatalf("markdown not rendered: %s", body)
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

type stubPicker struct {
	path string
	err  error
	kind domain.PickKind
}

func (s *stubPicker) Pick(_ context.Context, kind domain.PickKind) (string, error) {
	s.kind = kind
	return s.path, s.err
}

func handlerWithPicker(t *testing.T, picker domain.HostPicker) http.Handler {
	t.Helper()
	root := t.TempDir()
	d := newHTTPDeps(domain.NewRegistry(root, nil), testToolkitRoot(t), 3080)
	d.picker = picker
	handler, err := mountHTTP(d)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func TestWorkspacePickReturnsAbsoluteJSONPath(t *testing.T) {
	root := t.TempDir()
	fileBody := "token=" + testSecret
	filePath := filepath.Join(root, "my file.txt")
	if err := os.WriteFile(filePath, []byte(fileBody), 0o600); err != nil {
		t.Fatal(err)
	}
	folderPath := filepath.Join(root, "pkg")
	if err := os.Mkdir(folderPath, 0o750); err != nil {
		t.Fatal(err)
	}
	filePicker := &stubPicker{path: filePath}
	handler := handlerWithPicker(t, filePicker)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=file", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("file status = %d body=%s", rec.Code, rec.Body.String())
	}
	if filePicker.kind != domain.PickFile {
		t.Fatalf("kind = %q", filePicker.kind)
	}
	body := rec.Body.String()
	if strings.Contains(body, fileBody) || strings.Contains(body, testSecret) {
		t.Fatalf("file body leaked: %s", body)
	}
	if strings.Contains(body, "@") {
		t.Fatalf("attach must not use @: %s", body)
	}
	if !strings.Contains(body, `"path"`) {
		t.Fatalf("missing path: %s", body)
	}
	if !strings.Contains(body, "my file.txt") {
		t.Fatalf("missing file name: %s", body)
	}

	folderPicker := &stubPicker{path: folderPath}
	handler = handlerWithPicker(t, folderPicker)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=folder", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("folder status = %d body=%s", rec.Code, rec.Body.String())
	}
	if folderPicker.kind != domain.PickFolder {
		t.Fatalf("kind = %q", folderPicker.kind)
	}
	if !strings.Contains(rec.Body.String(), "pkg") {
		t.Fatalf("missing folder name: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=other", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("other kind status = %d", rec.Code)
	}

	cancelPicker := &stubPicker{err: domain.ErrPickCancelled}
	handler = handlerWithPicker(t, cancelPicker)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=file", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", rec.Code, rec.Body.String())
	}
	cancelBody := rec.Body.String()
	if !strings.Contains(cancelBody, `"cancelled":true`) && !strings.Contains(cancelBody, `"cancelled": true`) {
		t.Fatalf("expected cancelled: %s", cancelBody)
	}
	if strings.Contains(cancelBody, `"path"`) {
		t.Fatalf("cancel must not include a path: %s", cancelBody)
	}
}

func TestWorkspacePickUnavailableReturns501(t *testing.T) {
	handler := handlerWithPicker(t, &stubPicker{err: domain.ErrPickUnavailable})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=file", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspacePickBadPathReturns400(t *testing.T) {
	handler := handlerWithPicker(t, &stubPicker{path: "relative.txt"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/workspace/pick?kind=file", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspacePickRejectsGet(t *testing.T) {
	handler := handlerWithPicker(t, &stubPicker{path: t.TempDir()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/workspace/pick?kind=file", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}
