package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

// NewHandler builds routes for the empty shell.
func NewHandler(workspaceRoot string) (http.Handler, error) {
	return NewHandlerWithPort(workspaceRoot, defaultPort)
}

// NewHandlerWithPort builds routes using the given port for bind metadata.
func NewHandlerWithPort(workspaceRoot string, port int) (http.Handler, error) {
	return mountHTTP(httpDeps{
		workspaceRoot: filepath.Clean(workspaceRoot),
		bindAddr:      Addr(port),
	})
}

type httpDeps struct {
	workspaceRoot string
	bindAddr      string
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
	mux.HandleFunc("/session/", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimPrefix(r.URL.Path, "/session/")
		slug = strings.Trim(slug, "/")
		if slug == "" || strings.Contains(slug, "/") {
			http.NotFound(w, r)
			return
		}
		page, err := view.BuildSessionPage(d.workspaceRoot, d.bindAddr, slug)
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
		page, err := view.BuildShellPage(d.workspaceRoot, d.bindAddr)
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
