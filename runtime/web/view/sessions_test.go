package view

import (
	"testing"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
	"github.com/ducnd58233/vibe-agent/runtime/internal/session"
	"github.com/ducnd58233/vibe-agent/runtime/internal/testutil"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/sessionread"
)

func TestFormatRel(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		delta time.Duration
		want  string
	}{
		{30 * time.Second, "now"},
		{5 * time.Minute, "5m"},
		{11 * time.Hour, "11h"},
		{48 * time.Hour, "2d"},
	}
	for _, tc := range cases {
		got := FormatRel(now, now.Add(-tc.delta))
		if got != tc.want {
			t.Fatalf("delta %s: got %q want %q", tc.delta, got, tc.want)
		}
	}
}

func TestProjectSessionsCardFields(t *testing.T) {
	root := t.TempDir()
	slug := "control-plane-activation"
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	run, err := state.NewRun(slug, "Control plane activation", "goal-delivery", 50, now.Add(-11*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	run.Status = state.StatusFailed
	run.UpdatedAt = now.Add(-11 * time.Hour)
	testutil.EnsureRunIndex(t, root, slug)
	if err := state.Save(state.ManifestPath(root, slug), run); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Append(session.LogPath(root, slug), session.Record{
		Type:   session.TypeSessionStart,
		Source: session.SourceHook,
		Client: "claude",
		Event:  "SessionStart",
		At:     now.Add(-11 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rows := ProjectSessions(sessionread.NewFS(), root, []string{slug}, now)
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	row := rows[0]
	if row.Title != slug {
		t.Fatalf("title = %q", row.Title)
	}
	if row.Host != "claude" {
		t.Fatalf("host = %q", row.Host)
	}
	if row.Status != "failed" || row.StatusClass != "fail" {
		t.Fatalf("status = %q class = %q", row.Status, row.StatusClass)
	}
	if row.Rel != "11h" {
		t.Fatalf("rel = %q", row.Rel)
	}
}

func TestStatusClass(t *testing.T) {
	if statusClass("awaiting_human") != "warn" {
		t.Fatal("awaiting_human")
	}
	if statusClass("running") != "live" {
		t.Fatal("running")
	}
	if statusClass("done") != "pass" {
		t.Fatal("done")
	}
}
