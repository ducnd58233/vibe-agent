package persistence

import (
	"os"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/web/domain"
)

func TestWriteStateCreatesWebJSON(t *testing.T) {
	root := t.TempDir()
	if err := WriteState(root, domain.State{URL: "http://127.0.0.1:3080/", PID: 42}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "127.0.0.1:3080") {
		t.Fatalf("web.json = %s", raw)
	}
}
