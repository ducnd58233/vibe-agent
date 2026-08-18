package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/web/infra/persistence"
)

func TestDoctorFailsNonLoopbackWebJSON(t *testing.T) {
	root := t.TempDir()
	if err := persistence.WriteState(root, domain.State{
		URL:       "http://0.0.0.0:3080/",
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	report := &diagnostics{}
	checkWebState(report, root)
	if report.problems != 1 {
		t.Fatalf("problems = %d", report.problems)
	}
}

func TestDoctorAcceptsLoopbackWebJSON(t *testing.T) {
	root := t.TempDir()
	if err := persistence.WriteState(root, domain.State{
		URL:       "http://127.0.0.1:3080/",
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	report := &diagnostics{}
	checkWebState(report, root)
	if report.problems != 0 {
		t.Fatalf("problems = %d", report.problems)
	}
}

func TestDoctorWebJSONMissingIsNoteOnly(t *testing.T) {
	root := t.TempDir()
	report := &diagnostics{}
	checkWebState(report, root)
	if report.problems != 0 {
		t.Fatalf("problems = %d", report.problems)
	}
	if _, err := os.Stat(filepath.Join(root, ".agent-state", "web.json")); !os.IsNotExist(err) {
		t.Fatal("expected no web.json")
	}
}

func TestDoctorInvalidWebJSON(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agent-state")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	report := &diagnostics{}
	checkWebState(report, root)
	if report.problems != 1 {
		t.Fatalf("problems = %d", report.problems)
	}
}
