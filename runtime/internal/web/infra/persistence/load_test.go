package persistence

import (
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func TestLoadStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	want := domain.State{URL: "http://127.0.0.1:3080/", PID: 42, StartedAt: time.Now().UTC()}
	if err := WriteState(root, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadState(root)
	if err != nil || !ok {
		t.Fatalf("load = %v ok = %v", err, ok)
	}
	if got.URL != want.URL || got.PID != want.PID {
		t.Fatalf("got %+v", got)
	}
}
