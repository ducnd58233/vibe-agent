package app

import (
	"context"
	"net/http"

	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/httpserver"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/infra/streaming/sse"
	ui "github.com/ducnd58233/vibe-agent/runtime/web"
	"github.com/ducnd58233/vibe-agent/runtime/web/view"
)

func (d httpDeps) handleSessionEventsStream(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	const suffix = "/events/stream"
	slug, ok := parseSessionSuffixPath(r.URL.Path, suffix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	after, ok := parseAfterQuery(r)
	if !ok {
		writeBadAfter(w)
		return
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
	_ = sse.Poll(r.Context(), conn, 0, func(ctx context.Context) ([]sse.Event, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := view.EventsAfterForView(ws, slug, cursor, selectedView)
		if err != nil {
			if session.IsNotFound(err) {
				return nil, nil
			}
			return []sse.Event{{Type: "error", Data: "Could not read the session log."}}, nil
		}
		events, err := eventRowsToSSE(tmpl, rows)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			cursor = row.Seq
		}
		return events, nil
	})
}
