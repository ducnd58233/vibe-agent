package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
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
