package observability

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewLoggerWritesTintedConsoleAndJSONFile(t *testing.T) {
	dir := t.TempDir()
	var console bytes.Buffer
	log, closer, err := NewLogger(Options{
		Service: "web",
		Level:   "info",
		Stdout:  &console,
		Dir:     dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	log.Error("request failed", "error", "boom", "stack", "frame1\nframe2")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}

	out := console.String()
	if out == "" {
		t.Fatal("console log is empty")
	}
	if strings.Contains(out, "frame1") {
		t.Fatalf("console should omit stack: %s", out)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open("web.log")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("file is not JSON: %s", raw)
	}
	if rec["stack"] == nil {
		t.Fatalf("file missing stack: %#v", rec)
	}
}
