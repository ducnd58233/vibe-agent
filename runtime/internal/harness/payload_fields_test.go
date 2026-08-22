package harness

import (
	"strings"
	"testing"
)

func TestAntigravityToolCallCommandLineIsRead(t *testing.T) {
	raw := `{
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "git push origin main", "Cwd": "/workspace"}
		},
		"transcriptPath": "/tmp/transcript.jsonl"
	}`
	body := readPayload(strings.NewReader(raw))
	if body.ToolName != "run_command" {
		t.Fatalf("tool name = %q, want run_command", body.ToolName)
	}
	if got := body.shellCommand(); got != "git push origin main" {
		t.Fatalf("command = %q, want git push origin main", got)
	}
	if body.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Fatalf("transcript = %q", body.TranscriptPath)
	}
}
