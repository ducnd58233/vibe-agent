package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

// NewHandler builds routes for the empty shell.
func NewHandler(workspaceRoot, toolkitRoot string) (http.Handler, error) {
	return NewHandlerWithPort(workspaceRoot, toolkitRoot, defaultPort)
}

// NewHandlerWithPort builds routes using the given port for bind metadata.
func NewHandlerWithPort(workspaceRoot, toolkitRoot string, port int) (http.Handler, error) {
	reg, err := loadRegistry(filepath.Clean(workspaceRoot), nil)
	if err != nil {
		return nil, err
	}
	return mountHTTP(httpDeps{
		registry:    reg,
		toolkitRoot: filepath.Clean(toolkitRoot),
		bindAddr:    Addr(port),
	})
}

// NewHandlerWithRegistry builds routes with an explicit workspace registry (tests).
func NewHandlerWithRegistry(reg domain.Registry, toolkitRoot string, port int) (http.Handler, error) {
	return mountHTTP(httpDeps{
		registry:    reg,
		toolkitRoot: filepath.Clean(toolkitRoot),
		bindAddr:    Addr(port),
	})
}

type httpDeps struct {
	registry    domain.Registry
	toolkitRoot string
	bindAddr    string
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
	mux.HandleFunc("/workspace/switch", func(w http.ResponseWriter, r *http.Request) {
		handleWorkspaceSwitch(w, r, d)
	})
	mux.HandleFunc("/session/new", func(w http.ResponseWriter, r *http.Request) {
		handleNewSession(w, r, d)
	})
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
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
			http.NotFound(w, r)
			return
		}
		ws := d.activeWorkspace(r)
		page, err := view.BuildSessionPage(ws, d.toolkitRoot, d.bindAddr, slug, d.registry, ws)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "page error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "session.html", page); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ws := d.activeWorkspace(r)
		page, err := view.BuildShellPage(ws, d.bindAddr, d.registry, ws)
		if err != nil {
			http.Error(w, "page error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "shell.html", page); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	})
	return mux, nil
}
