// Package web holds embedded UI assets and template rendering for the loopback
// control-plane viewer. HTTP routing and persistence live in internal/web.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

// Templates parses embedded HTML with view helpers registered.
func Templates() (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"KindOrder": func() []session.FilterKind { return view.KindOrder },
		"upper":     strings.ToUpper,
	}).ParseFS(templateFiles, "templates/*.html")
}

// StaticHandler serves vendored CSS and HTMX under /static/.
func StaticHandler() (http.Handler, error) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub))), nil
}
