package view

import (
	"sort"
	"strconv"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

// SessionRow is one sidebar session card.
type SessionRow struct {
	Slug        string
	Title       string
	Host        string
	Status      string
	StatusClass string
	Rel         string
	UpdatedAt   time.Time
}

// ProjectSessions builds sidebar cards from run manifests and log metadata.
func ProjectSessions(logs sessionread.Reader, workspaceRoot string, slugs []string, now time.Time) []SessionRow {
	rows := make([]SessionRow, 0, len(slugs)+1)
	for _, slug := range slugs {
		run, err := state.Load(state.ManifestPath(workspaceRoot, slug))
		if err != nil || run == nil {
			continue
		}
		rows = append(rows, SessionRow{
			Slug:        slug,
			Title:       slug,
			Host:        logs.PeekHost(session.LogPath(workspaceRoot, slug)),
			Status:      string(run.Status),
			StatusClass: statusClass(string(run.Status)),
			Rel:         FormatRel(now, run.UpdatedAt),
			UpdatedAt:   run.UpdatedAt,
		})
	}
	if ambient := logs.AmbientStat(workspaceRoot); ambient.Present {
		rows = append(rows, SessionRow{
			Slug:        "ambient",
			Title:       "Ambient journal",
			Host:        logs.PeekHost(session.AmbientLogPath(workspaceRoot)),
			Status:      "idle",
			StatusClass: "",
			Rel:         FormatRel(now, ambient.ModTime),
			UpdatedAt:   ambient.ModTime,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	return rows
}

// FormatRel is the compact relative stamp used on session cards.
func FormatRel(now, then time.Time) string {
	if then.IsZero() {
		return ""
	}
	delta := now.Sub(then)
	if delta < 0 {
		delta = 0
	}
	switch {
	case delta < time.Minute:
		return "now"
	case delta < time.Hour:
		return strconv.Itoa(int(delta/time.Minute)) + "m"
	case delta < 36*time.Hour:
		return strconv.Itoa(int(delta/time.Hour)) + "h"
	default:
		return strconv.Itoa(int(delta/(24*time.Hour))) + "d"
	}
}

func statusClass(status string) string {
	switch state.Status(status) {
	case state.StatusFailed, state.StatusCancelled, state.StatusBudgetExceeded:
		return "fail"
	case state.StatusAwaitingHuman:
		return "warn"
	case state.StatusRunning:
		return "live"
	case state.StatusDone:
		return "pass"
	default:
		return ""
	}
}
