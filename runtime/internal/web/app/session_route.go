package app

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/streaming/sse"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

// parseSessionSubpath returns slug when the path is /session/{slug}/{action}.
func parseSessionSubpath(path, action string) (slug string, ok bool) {
	path = strings.TrimPrefix(path, "/session/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != action {
		return "", false
	}
	slug = parts[0]
	if slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

// parseSessionSuffixPath returns slug when path ends with /session/{slug}{suffix}.
func parseSessionSuffixPath(path, suffix string) (slug string, ok bool) {
	if !strings.HasSuffix(path, suffix) {
		return "", false
	}
	slug = strings.TrimSuffix(strings.TrimPrefix(path, "/session/"), suffix)
	slug = strings.Trim(slug, "/")
	if slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return slug, true
}

func parseAfterQuery(r *http.Request) (after int, ok bool) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func writeEventRows(w io.Writer, tmpl *template.Template, rows []view.EventRow) error {
	for _, row := range rows {
		if err := tmpl.ExecuteTemplate(w, "event-row", row); err != nil {
			return err
		}
	}
	return nil
}

func eventRowsToSSE(tmpl *template.Template, rows []view.EventRow) ([]sse.Event, error) {
	out := make([]sse.Event, 0, len(rows))
	for _, row := range rows {
		var buf strings.Builder
		if err := tmpl.ExecuteTemplate(&buf, "event-row", row); err != nil {
			return nil, err
		}
		out = append(out, sse.Event{ID: fmt.Sprint(row.Seq), Data: buf.String()})
	}
	return out, nil
}
