package view

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
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

type sessionLine struct {
	Payload struct {
		Client string `json:"client"`
	} `json:"payload"`
}

// ProjectSessions builds sidebar cards from run manifests and log metadata.
func ProjectSessions(workspaceRoot string, slugs []string, now time.Time) []SessionRow {
	rows := make([]SessionRow, 0, len(slugs)+1)
	for _, slug := range slugs {
		run, err := state.Load(state.ManifestPath(workspaceRoot, slug))
		if err != nil || run == nil {
			continue
		}
		rows = append(rows, SessionRow{
			Slug:        slug,
			Title:       slug,
			Host:        peekHost(session.LogPath(workspaceRoot, slug)),
			Status:      string(run.Status),
			StatusClass: statusClass(string(run.Status)),
			Rel:         FormatRel(now, run.UpdatedAt),
			UpdatedAt:   run.UpdatedAt,
		})
	}
	if hasAmbientSession(workspaceRoot) {
		updated := now
		if info, err := os.Stat(session.AmbientLogPath(workspaceRoot)); err == nil {
			updated = info.ModTime()
		}
		rows = append(rows, SessionRow{
			Slug:        "ambient",
			Title:       "Ambient journal",
			Host:        peekHost(session.AmbientLogPath(workspaceRoot)),
			Status:      "idle",
			StatusClass: "",
			Rel:         FormatRel(now, updated),
			UpdatedAt:   updated,
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

func peekHost(logPath string) string {
	file, err := os.Open(filepath.Clean(logPath))
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for i := 0; i < 8 && scanner.Scan(); i++ {
		var line sessionLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Payload.Client != "" {
			return line.Payload.Client
		}
	}
	_ = scanner.Err()
	return ""
}
