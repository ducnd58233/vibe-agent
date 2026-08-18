package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

const defaultPort = 3080

// Config holds loopback server settings.
type Config struct {
	WorkspaceRoot string
	Port          int
}

// ListenHost is always loopback.
const ListenHost = "127.0.0.1"

// ValidateListenHost refuses non-loopback binds.
func ValidateListenHost(host string) error {
	switch host {
	case "127.0.0.1", "localhost":
		return nil
	case "0.0.0.0", "::", "[::]":
		return fmt.Errorf("refusing to bind %q: loopback only", host)
	default:
		ip := net.ParseIP(host)
		if ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("refusing to bind %q: loopback only", host)
	}
}

// Addr formats the listen address for a port.
func Addr(port int) string {
	if port <= 0 {
		port = defaultPort
	}
	return fmt.Sprintf("%s:%d", ListenHost, port)
}

// Run starts the loopback server and blocks until it stops.
func Run(cfg Config) error {
	if cfg.Port <= 0 {
		cfg.Port = defaultPort
	}
	if err := ValidateListenHost(ListenHost); err != nil {
		return err
	}
	handler, err := NewHandlerWithPort(cfg.WorkspaceRoot, cfg.Port)
	if err != nil {
		return err
	}
	addr := Addr(cfg.Port)
	if err := WriteState(cfg.WorkspaceRoot, State{
		URL:       "http://" + addr + "/",
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if _, err := fmt.Fprintf(os.Stdout, "vibe-agent web listening on http://%s/\n", addr); err != nil {
		return err
	}
	return srv.ListenAndServe()
}

// NewHandler builds routes for the empty shell.
func NewHandler(workspaceRoot string) (http.Handler, error) {
	return NewHandlerWithPort(workspaceRoot, defaultPort)
}

// NewHandlerWithPort builds routes using the given port for bind metadata.
func NewHandlerWithPort(workspaceRoot string, port int) (http.Handler, error) {
	tmpl, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(static))
	root := filepath.Clean(workspaceRoot)
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page, err := BuildShellPage(root, port)
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
