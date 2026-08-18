package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

const ssePollInterval = time.Second

func (d httpDeps) handleSessionEventsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	const suffix = "/events/stream"
	if !strings.HasSuffix(r.URL.Path, suffix) {
		http.NotFound(w, r)
		return
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/session/"), suffix)
	slug = strings.Trim(slug, "/")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	after := 0
	if raw := r.URL.Query().Get("after"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			http.Error(w, "bad after", http.StatusBadRequest)
			return
		}
		after = n
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	tmpl, err := ui.Templates()
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: template error\n\n")
		flusher.Flush()
		return
	}
	ws := d.activeWorkspace(r)
	ticker := time.NewTicker(ssePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			rows, err := view.EventsAfter(ws, slug, after)
			if err != nil {
				if !isNotFoundErr(err) {
					_, _ = fmt.Fprintf(w, "event: error\ndata: Could not read the session log.\n\n")
					flusher.Flush()
				}
				continue
			}
			for _, row := range rows {
				var buf strings.Builder
				if err := tmpl.ExecuteTemplate(&buf, "event-row", row); err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", buf.String()); err != nil {
					return
				}
				flusher.Flush()
				after = row.Seq
			}
		}
	}
}
