package hosts

import (
	"slices"
	"testing"
)

func TestPrintArgvClaudeModelLeavesCatalogUnchanged(t *testing.T) {
	host, ok := EvalHost("claude")
	if !ok {
		t.Fatal("claude")
	}
	orig := host.EvalCommand
	argv := PrintArgv(host, PrintOptions{Model: "opus"})
	if host.EvalCommand != orig {
		t.Fatalf("EvalCommand mutated: %q", host.EvalCommand)
	}
	again, _ := EvalHost("claude")
	if again.EvalCommand != orig {
		t.Fatalf("catalog mutated: %q", again.EvalCommand)
	}
	if !slices.Equal(argv[len(argv)-2:], []string{"--model", "opus"}) {
		t.Fatalf("argv = %v", argv)
	}
}

func TestPrintArgvClaudeIncludesHookEvents(t *testing.T) {
	host, ok := EvalHost("claude")
	if !ok {
		t.Fatal("claude")
	}
	orig := host.EvalCommand
	argv := PrintArgv(host, PrintOptions{})
	if host.EvalCommand != orig {
		t.Fatalf("EvalCommand mutated: %q", orig)
	}
	if !hasFlagValue(argv, "--output-format", "stream-json") {
		t.Fatalf("argv = %v, want stream-json", argv)
	}
	if !slices.Contains(argv, "--include-hook-events") {
		t.Fatalf("argv = %v, want --include-hook-events", argv)
	}
	if !slices.Contains(argv, "--verbose") {
		t.Fatalf("argv = %v, want --verbose", argv)
	}
}

func TestPrintArgvIgnoresModelOnCodex(t *testing.T) {
	host, ok := EvalHost("codex")
	if !ok {
		t.Fatal("codex")
	}
	argv := PrintArgv(host, PrintOptions{Model: "opus"})
	if slices.Contains(argv, "--model") {
		t.Fatalf("codex argv leaked --model: %v", argv)
	}
}

func TestPrintArgvCursorAskDefaultKeepsModeAsk(t *testing.T) {
	host, ok := EvalHost("cursor-agent")
	if !ok {
		t.Fatal("cursor-agent")
	}
	argv := PrintArgv(host, PrintOptions{})
	if !hasFlagValue(argv, "--mode", "ask") {
		t.Fatalf("ask default missing: %v", argv)
	}
}

func TestPrintArgvCursorAgentDropsModeAsk(t *testing.T) {
	host, ok := EvalHost("cursor-agent")
	if !ok {
		t.Fatal("cursor-agent")
	}
	argv := PrintArgv(host, PrintOptions{Mode: "agent", Model: "gpt-5"})
	if hasFlagValue(argv, "--mode", "ask") {
		t.Fatalf("agent still has --mode ask: %v", argv)
	}
	if slices.Contains(argv, "--mode") {
		t.Fatalf("agent should omit --mode (CLI default is agent): %v", argv)
	}
	if !slices.Equal(argv[len(argv)-2:], []string{"--model", "gpt-5"}) {
		t.Fatalf("argv = %v", argv)
	}
}

func TestModelSuggestionsClaudeFromHelp(t *testing.T) {
	host, ok := EvalHost("claude")
	if !ok {
		t.Fatal("claude")
	}
	got := ModelSuggestions(host)
	for _, want := range []string{"sonnet", "opus", "fable"} {
		if !slices.Contains(got, want) {
			t.Fatalf("claude suggestions %v missing %q", got, want)
		}
	}
	codex, ok := EvalHost("codex")
	if !ok {
		t.Fatal("codex")
	}
	if hints := ModelSuggestions(codex); len(hints) != 0 {
		t.Fatalf("codex suggestions = %v", hints)
	}
	cursor, ok := EvalHost("cursor-agent")
	if !ok {
		t.Fatal("cursor-agent")
	}
	gotCursor := ModelSuggestions(cursor)
	for _, want := range []string{"gpt-5", "sonnet-4-thinking"} {
		if !slices.Contains(gotCursor, want) {
			t.Fatalf("cursor-agent suggestions %v missing %q", gotCursor, want)
		}
	}
}

func hasFlagValue(args []string, flag, value string) bool {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
		if arg == flag+"="+value {
			return true
		}
	}
	return false
}
