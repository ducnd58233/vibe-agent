package app

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/hostpick"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

// NewHandler builds routes for the empty shell.
func NewHandler(workspaceRoot, toolkitRoot string) (http.Handler, error) {
	return NewHandlerWithPort(workspaceRoot, toolkitRoot, DefaultPort)
}

// NewHandlerWithPort builds routes using the given port for bind metadata.
func NewHandlerWithPort(workspaceRoot, toolkitRoot string, port int) (http.Handler, error) {
	reg, err := loadRegistry(filepath.Clean(workspaceRoot), nil)
	if err != nil {
		return nil, err
	}
	return mountHTTP(newHTTPDeps(reg, toolkitRoot, port))
}

// NewHandlerWithRegistry builds routes with an explicit workspace registry (tests).
func NewHandlerWithRegistry(reg domain.Registry, toolkitRoot string, port int) (http.Handler, error) {
	return mountHTTP(newHTTPDeps(reg, toolkitRoot, port))
}

type httpDeps struct {
	mu          *sync.Mutex
	registry    *domain.Registry
	toolkitRoot string
	bindAddr    string
	picker      domain.HostPicker
	sessionRead sessionread.Reader
}

func newHTTPDeps(reg domain.Registry, toolkitRoot string, port int) httpDeps {
	copyReg := reg
	return httpDeps{
		mu:          new(sync.Mutex),
		registry:    &copyReg,
		toolkitRoot: filepath.Clean(toolkitRoot),
		bindAddr:    Addr(port),
		picker:      hostpick.OS(),
		sessionRead: sessionread.NewFS(),
	}
}

func (d httpDeps) snapshotRegistry() domain.Registry {
	d.mu.Lock()
	defer d.mu.Unlock()
	return *d.registry
}

func (d httpDeps) storeRegistry(reg domain.Registry) {
	d.mu.Lock()
	defer d.mu.Unlock()
	*d.registry = reg
}

func renderError(w http.ResponseWriter, tmpl *template.Template, code int, title, detail string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = tmpl.ExecuteTemplate(w, "error.html", view.ErrorPage{
		Code:   code,
		Title:  title,
		Detail: detail,
	})
}

func mountHTTP(d httpDeps) (http.Handler, error) {
	tmpl, err := ui.Templates()
	if err != nil {
		return nil, err
	}
	static, err := ui.StaticHandler()
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", static)
	mux.HandleFunc("/catalog/commands", func(w http.ResponseWriter, r *http.Request) {
		d.handleCatalogCommands(w, r)
	})
	mux.HandleFunc("/catalog/skills", func(w http.ResponseWriter, r *http.Request) {
		d.handleCatalogSkills(w, r)
	})
	mux.HandleFunc("/workspace/files/view", func(w http.ResponseWriter, r *http.Request) {
		d.handleWorkspaceFileView(w, r)
	})
	mux.HandleFunc("/workspace/files/preview", func(w http.ResponseWriter, r *http.Request) {
		d.handleWorkspaceFilePreview(w, r)
	})
	mux.HandleFunc("/workspace/files", func(w http.ResponseWriter, r *http.Request) {
		d.handleWorkspaceFiles(w, r)
	})
	mux.HandleFunc("/workspace/pick", func(w http.ResponseWriter, r *http.Request) {
		d.handleWorkspacePick(w, r)
	})
	mux.HandleFunc("/workspace/browse", func(w http.ResponseWriter, r *http.Request) {
		d.handleWorkspaceBrowse(w, r)
	})
	mux.HandleFunc("/workspace/switch", func(w http.ResponseWriter, r *http.Request) {
		handleWorkspaceSwitch(w, r, d)
	})
	mux.HandleFunc("/workspace/open", func(w http.ResponseWriter, r *http.Request) {
		handleWorkspaceOpen(w, r, d)
	})
	mux.HandleFunc("/session/new", func(w http.ResponseWriter, r *http.Request) {
		handleNewSession(w, r, d)
	})
	mux.HandleFunc("/session/check-slug", func(w http.ResponseWriter, r *http.Request) {
		handleCheckSlug(w, r, d)
	})
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(r.URL.Path, "/"), "/checkpoint") {
			handleSessionCheckpoint(w, r, d)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(strings.Trim(r.URL.Path, "/"), "/send") {
			handleComposerSend(w, r, d)
			return
		}
		if strings.HasSuffix(strings.Trim(r.URL.Path, "/"), "/events/stream") {
			d.handleSessionEventsStream(w, r)
			return
		}
		if strings.HasSuffix(strings.Trim(r.URL.Path, "/"), "/events") {
			handleSessionEvents(w, r, d)
			return
		}
		slug := strings.TrimPrefix(r.URL.Path, "/session/")
		slug = strings.Trim(slug, "/")
		if slug == "" || strings.Contains(slug, "/") {
			renderError(w, tmpl, 404, "Session not found", "The requested session does not exist or the URL is invalid.")
			return
		}
		ws := d.activeWorkspace(r)
		page, err := view.BuildSessionPage(d.sessionRead, ws, d.toolkitRoot, d.bindAddr, slug, r.URL.Query().Get("view"), d.snapshotRegistry(), ws)
		if err != nil {
			if session.IsNotFound(err) {
				renderError(w, tmpl, 404, "Session not found", "No session with slug \""+slug+"\" exists in this workspace.")
				return
			}
			renderError(w, tmpl, 500, "Something went wrong", "Failed to load the session page. Check server logs for details.")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "session.html", page); err != nil {
			renderError(w, tmpl, 500, "Render failed", "The session page template could not be rendered.")
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			renderError(w, tmpl, 404, "Page not found", "The page you are looking for does not exist.")
			return
		}
		ws := d.activeWorkspace(r)
		page, err := view.BuildShellPage(d.sessionRead, ws, d.bindAddr, d.snapshotRegistry(), ws)
		if err != nil {
			renderError(w, tmpl, 500, "Something went wrong", "Failed to load the workspace page.")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "shell.html", page); err != nil {
			renderError(w, tmpl, 500, "Render failed", "The workspace template could not be rendered.")
		}
	})
	return mux, nil
}
