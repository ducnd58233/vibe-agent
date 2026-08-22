package session

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	benchReplayOnce sync.Once
	benchReplayPath string
	errBenchReplay  error
)

func benchReplayLogPath() (string, error) {
	benchReplayOnce.Do(func() {
		dir, err := os.MkdirTemp("", "session-replay-bench-*")
		if err != nil {
			errBenchReplay = err
			return
		}
		path := filepath.Join(dir, LogName)
		for i := 0; i < 48; i++ {
			rec := Record{
				Type:   TypePromptSubmit,
				Source: SourceHook,
				Client: "claude",
				Body:   "benchmark prompt body",
			}
			if i%3 == 0 {
				rec.Type = TypeMessage
				rec.Role = "assistant"
				rec.Body = "assistant reply"
			}
			if _, err := Append(path, rec); err != nil {
				errBenchReplay = err
				return
			}
		}
		benchReplayPath = path
	})
	return benchReplayPath, errBenchReplay
}

func BenchmarkReplay(b *testing.B) {
	path, err := benchReplayLogPath()
	if err != nil {
		b.Fatal(err)
	}
	events, err := Replay(path)
	if err != nil {
		b.Fatal(err)
	}
	if len(events) != 48 {
		b.Fatalf("setup events = %d, want 48", len(events))
	}
	b.ReportAllocs()
	for b.Loop() {
		_, err := Replay(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}
