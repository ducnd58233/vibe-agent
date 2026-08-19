package app

import (
	"errors"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/redact"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

const filePreviewMaxBytes = 4096
const fileViewMaxBytes = 64 * 1024

type fileBrowserModel struct {
	Mode      string
	Dir       string
	Rows      []domain.FileRow
	Parent    string
	AttachDir string
	OpenPath  string
}

func renderFileBrowser(w http.ResponseWriter, r *http.Request, model fileBrowserModel) {
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "file-browser", model); err != nil {
		writeTemplateError(w, r)
	}
}

func (d httpDeps) handleWorkspaceFiles(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ws := d.activeWorkspace(r)
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	absDir, err := domain.ResolveWorkspacePath(ws, dir)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad path")
		return
	}
	info, err := os.Stat(filepath.Clean(absDir))
	if err != nil || !info.IsDir() {
		writeHTMXOrError(w, r, http.StatusBadRequest, "not a directory")
		return
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "read error")
		return
	}
	var rows []domain.FileRow
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		rel, err := domain.RelWorkspacePath(ws, filepath.Join(absDir, entry.Name()))
		if err != nil {
			continue
		}
		rows = append(rows, domain.FileRow{
			Name:   entry.Name(),
			Path:   rel,
			IsDir:  entry.IsDir(),
			Attach: domain.FormatAttach(rel),
		})
	}
	relDir := filepath.ToSlash(strings.TrimPrefix(dir, "/"))
	model := fileBrowserModel{
		Mode: "attach",
		Dir:  relDir,
		Rows: rows,
	}
	if relDir != "" {
		model.AttachDir = domain.FormatAttach(relDir)
	}
	if dir != "" {
		parent := filepath.Dir(filepath.FromSlash(dir))
		if parent == "." {
			model.Parent = ""
		} else {
			model.Parent = filepath.ToSlash(parent)
		}
	}
	renderFileBrowser(w, r, model)
}

func (d httpDeps) handleWorkspaceBrowse(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		renderFileBrowser(w, r, fileBrowserModel{
			Mode: "open",
			Dir:  "",
			Rows: domain.HostRoots(),
		})
		return
	}
	abs, err := domain.ResolveHostDir(dir)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad path")
		return
	}
	rows, err := domain.ListHostEntries(abs)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad path")
		return
	}
	renderFileBrowser(w, r, fileBrowserModel{
		Mode:     "open",
		Dir:      filepath.ToSlash(abs),
		Rows:     rows,
		Parent:   domain.HostParent(abs),
		OpenPath: abs,
	})
}

func (d httpDeps) handleWorkspaceFilePreview(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ws := d.activeWorkspace(r)
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	abs, err := domain.ResolveWorkspacePath(ws, rel)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad path")
		return
	}
	info, err := os.Stat(filepath.Clean(abs))
	if err != nil || info.IsDir() {
		writeHTMXOrError(w, r, http.StatusBadRequest, "not a file")
		return
	}
	f, err := os.Open(filepath.Clean(abs))
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "read error")
		return
	}
	defer func() { _ = f.Close() }()
	limited := io.LimitReader(f, filePreviewMaxBytes)
	raw, err := io.ReadAll(limited)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "read error")
		return
	}
	text := redact.Text(string(raw))
	model := struct {
		Path    string
		Attach  string
		Excerpt string
	}{
		Path:    filepath.ToSlash(rel),
		Attach:  domain.FormatAttach(rel),
		Excerpt: text,
	}
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "file-preview", model); err != nil {
		writeTemplateError(w, r)
	}
}

func (d httpDeps) handleWorkspaceFileView(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ws := d.activeWorkspace(r)
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	abs, resolvedRel, err := resolveWorkspaceFileView(ws, rel)
	if err != nil {
		// Some links in session content point at vibe-agent toolkit assets
		// (like `.ai-agents/...`). If the active workspace does not contain
		// them, fall back to toolkitRoot so the viewer can still open.
		abs2, resolvedRel2, err2 := resolveWorkspaceFileView(d.toolkitRoot, rel)
		if err2 != nil {
			writeHTMXOrError(w, r, http.StatusBadRequest, "not a file")
			return
		}
		abs = abs2
		resolvedRel = resolvedRel2
	}
	f, err := os.Open(abs) //nolint:gosec // abs comes from ResolveWorkspacePath and a prior Stat check
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "read error")
		return
	}
	defer func() { _ = f.Close() }()
	limited := io.LimitReader(f, fileViewMaxBytes)
	raw, err := io.ReadAll(limited)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusInternalServerError, "read error")
		return
	}
	text := redact.Text(string(raw))
	isMarkdown := strings.HasSuffix(strings.ToLower(resolvedRel), ".md")
	model := fileViewerModel{
		Path:       filepath.ToSlash(resolvedRel),
		Content:    text,
		IsMarkdown: isMarkdown,
	}
	if isMarkdown {
		model.RenderedHTML = view.RenderMarkdown(text)
	}
	tmpl, err := ui.Templates()
	if err != nil {
		writeTemplateError(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "file-viewer", model); err != nil {
		writeTemplateError(w, r)
	}
}

func normalizeFileViewRel(rel string) string {
	trimmed := strings.TrimSpace(rel)
	if len(trimmed) >= 2 {
		first := trimmed[0]
		last := trimmed[len(trimmed)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
		if trimmed == "" {
			return ""
		}
	}
	trimmed = strings.TrimPrefix(trimmed, "./")
	return filepath.ToSlash(trimmed)
}

func resolveWorkspaceFileView(workspaceRoot, rel string) (abs string, resolvedRel string, err error) {
	rel = normalizeFileViewRel(rel)
	if rel == "" {
		return "", "", os.ErrNotExist
	}

	tryCandidate := func(candidateRel string) (string, bool) {
		absCandidate, err := domain.ResolveWorkspacePath(workspaceRoot, candidateRel)
		if err != nil {
			return "", false
		}
		info, err := os.Stat(filepath.Clean(absCandidate))
		if err != nil || info.IsDir() {
			return "", false
		}
		return absCandidate, true
	}

	if abs, ok := tryCandidate(rel); ok {
		return abs, rel, nil
	}

	// If the link was a bare filename (no directory), allow resolving it from
	// known references/docs roots. This supports inline code like
	// `loop-and-graph-engineering.md` where the prose omits `.ai-agents/...`.
	if strings.Contains(rel, "/") {
		return "", "", os.ErrNotExist
	}

	candidates := []string{
		filepath.ToSlash(filepath.Join(".ai-agents", "references", rel)),
		filepath.ToSlash(filepath.Join("docs", rel)),
		filepath.ToSlash(filepath.Join("runtime", rel)),
	}
	for _, cand := range candidates {
		if abs, ok := tryCandidate(cand); ok {
			return abs, cand, nil
		}
	}
	return "", "", os.ErrNotExist
}

type fileViewerModel struct {
	Path         string
	Content      string
	IsMarkdown   bool
	RenderedHTML template.HTML
}

type pickJSON struct {
	Path      string `json:"path,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

func (d httpDeps) handleWorkspacePick(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	kind, err := domain.ParsePickKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad kind")
		return
	}
	if d.picker == nil {
		writeHTMXOrError(w, r, http.StatusNotImplemented, "picker unavailable")
		return
	}
	raw, err := d.picker.Pick(r.Context(), kind)
	if err != nil {
		if errors.Is(err, domain.ErrPickCancelled) {
			httpserver.JSON(w, http.StatusOK, pickJSON{Cancelled: true})
			return
		}
		if errors.Is(err, domain.ErrPickUnavailable) {
			writeHTMXOrError(w, r, http.StatusNotImplemented, "picker unavailable")
			return
		}
		writeHTMXOrError(w, r, http.StatusBadGateway, "pick failed")
		return
	}
	formatted, err := domain.FormatAttachAbs(raw)
	if err != nil {
		writeHTMXOrError(w, r, http.StatusBadRequest, "bad path")
		return
	}
	httpserver.JSON(w, http.StatusOK, pickJSON{Path: formatted})
}
