package app

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/streaming/sse"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

const ssePollInterval = time.Second

func (d httpDeps) handleSessionEventsStream(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
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
			writeBadAfter(w)
			return
		}
		after = n
	}
	conn, err := sse.Begin(w)
	if err != nil {
		httpserver.RespondError(w, r, http.StatusInternalServerError, "stream unsupported")
		return
	}
	tmpl, err := ui.Templates()
	if err != nil {
		_ = conn.WriteEvent(sse.Event{Type: "error", Data: msgTemplateError})
		return
	}
	ws := d.activeWorkspace(r)
	selectedView := r.URL.Query().Get("view")
	cursor := after
	_ = sse.Poll(r.Context(), conn, ssePollInterval, func(ctx context.Context) ([]sse.Event, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := view.EventsAfterForView(ws, slug, cursor, selectedView)
		if err != nil {
			if isNotFoundErr(err) {
				return nil, nil
			}
			return []sse.Event{{Type: "error", Data: "Could not read the session log."}}, nil
		}
		var out []sse.Event
		for _, row := range rows {
			var buf strings.Builder
			if err := tmpl.ExecuteTemplate(&buf, "event-row", row); err != nil {
				continue
			}
			out = append(out, sse.Event{ID: fmt.Sprint(row.Seq), Data: buf.String()})
			cursor = row.Seq
		}
		return out, nil
	})
}
