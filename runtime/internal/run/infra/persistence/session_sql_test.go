package persistence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/run/domain"
	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/workspace"
)

func ambientPath(root string) string {
	return filepath.Join(workspace.StateDir(root), domain.SessionLogName)
}

func payload(body string) json.RawMessage {
	raw, _ := json.Marshal(map[string]string{"body": body})
	return raw
}

func TestSessionLogsLiveInTheDatabaseNotAFile(t *testing.T) {
	root := t.TempDir()
	_, runEvents := indexedPaths(t, root, "demo")
	runSession := filepath.Join(filepath.Dir(runEvents), domain.SessionLogName)

	for _, path := range []string{runSession, ambientPath(root)} {
		for i := 1; i <= 3; i++ {
			stored, err := AppendEvent(path, domain.Event{Type: "message", Payload: payload("hello"), At: fixedTime()})
			if err != nil {
				t.Fatalf("append to %s: %v", path, err)
			}
			if stored.Sequence != i {
				t.Errorf("sequence = %d, want %d", stored.Sequence, i)
			}
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s was written as a file", path)
		}
		events, err := ReadEvents(path)
		if err != nil || len(events) != 3 {
			t.Fatalf("read %s: %v (%d events)", path, err, len(events))
		}
	}

	// The two logs are separate scopes.
	run, _ := ReadEvents(runSession)
	ambient, _ := ReadEvents(ambientPath(root))
	if len(run) != 3 || len(ambient) != 3 {
		t.Fatalf("scopes bled into each other: %d / %d", len(run), len(ambient))
	}
}

func TestReadingAMissingSessionLogDoesNotCreateTheDatabase(t *testing.T) {
	root := t.TempDir()
	events, err := ReadEvents(ambientPath(root))
	if err != nil || len(events) != 0 {
		t.Fatalf("read: %v %v", events, err)
	}
	if _, err := os.Stat(workspace.MemoryDBPath(root)); err == nil {
		t.Fatal("a read created memory.db")
	}
}

func TestConcurrentAppendsNeverShareASequence(t *testing.T) {
	root := t.TempDir()
	path := ambientPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := AppendEvent(path, domain.Event{Type: "message", Payload: payload("x"), At: fixedTime()})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	events, err := ReadEvents(path)
	if err != nil || len(events) != writers {
		t.Fatalf("events = %d (%v), want %d", len(events), err, writers)
	}
	for i, ev := range events {
		if ev.Sequence != i+1 {
			t.Errorf("sequence[%d] = %d", i, ev.Sequence)
		}
	}
}

// A workspace that still has a session.ndjson must keep reading it until the
// next append adopts it; hiding it behind an empty table would lose the history.
func TestLegacySessionFileIsReadThenAdoptedOnAppend(t *testing.T) {
	root := t.TempDir()
	path := ambientPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	legacy := `{"sequence":1,"type":"prompt_submit","payload":{"body":"old one"},"at":"2026-08-01T10:00:00Z"}` + "\n" +
		`{"sequence":2,"type":"message","payload":{"body":"old two"},"at":"2026-08-01T10:00:01Z"}` + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := ReadEvents(path)
	if err != nil || len(events) != 2 {
		t.Fatalf("legacy read: %v %v", events, err)
	}

	stored, err := AppendEvent(path, domain.Event{Type: "message", Payload: payload("new"), At: fixedTime()})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if stored.Sequence != 3 {
		t.Errorf("sequence = %d, want it to continue from the legacy file", stored.Sequence)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the legacy file was not removed after adoption")
	}
	events, _ = ReadEvents(path)
	if len(events) != 3 {
		t.Errorf("events = %d, want 3", len(events))
	}
}

func TestCorruptLegacySessionFileIsSetAsideNotFatal(t *testing.T) {
	root := t.TempDir()
	path := ambientPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendEvent(path, domain.Event{Type: "message", Payload: payload("fresh"), At: fixedTime()}); err != nil {
		t.Fatalf("a corrupt legacy file wedged appends: %v", err)
	}
	if _, err := os.Stat(path + corruptSuffix); err != nil {
		t.Errorf("corrupt file was not preserved for repair: %v", err)
	}
}

func TestBackfillSessionsMovesRunAndAmbientFiles(t *testing.T) {
	root := t.TempDir()
	_, runEvents := indexedPaths(t, root, "demo")
	runSession := filepath.Join(filepath.Dir(runEvents), domain.SessionLogName)
	line := func(seq int, body string) string {
		return `{"sequence":` + string(rune('0'+seq)) + `,"type":"message","payload":{"body":"` + body + `"},"at":"2026-08-01T10:00:00Z"}` + "\n"
	}
	for path, content := range map[string]string{
		runSession:        line(1, "run one") + line(2, "run two"),
		ambientPath(root): line(1, "ambient one"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	moved, err := BackfillSessions(context.Background(), root)
	if err != nil || moved != 3 {
		t.Fatalf("backfill: moved=%d err=%v", moved, err)
	}
	for _, path := range []string{runSession, ambientPath(root)} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s still on disk", path)
		}
	}
	events, _ := ReadEvents(runSession)
	if len(events) != 2 || !strings.Contains(string(events[1].Payload), "run two") {
		t.Errorf("run events = %+v", events)
	}
	again, err := BackfillSessions(context.Background(), root)
	if err != nil || again != 0 {
		t.Errorf("second backfill: moved=%d err=%v", again, err)
	}
}
