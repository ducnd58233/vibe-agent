package verifier

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

func TestDecideNoBotsIsAPass(t *testing.T) {
	passed, summary := decide([]prCheck{
		{Name: "check", Bucket: "pass"},
		{Name: "e2e", Bucket: "fail"},
	})
	if !passed {
		t.Errorf("passed = false, want true: %s", summary)
	}
}

func TestDecidePassesWhenEveryBotIsNonBlocking(t *testing.T) {
	passed, _ := decide([]prCheck{
		{Name: "CodeRabbit", Bucket: "pass"},
		{Name: "cursor-bugbot", Bucket: "skipping"},
		{Name: "check", Bucket: "fail"}, // not a review bot, must not count
	})
	if !passed {
		t.Error("passed = false, want true")
	}
}

func TestDecideFailsOnAPendingBot(t *testing.T) {
	passed, summary := decide([]prCheck{
		{Name: "CodeRabbit", Bucket: "pending"},
	})
	if passed {
		t.Error("passed = true, want false")
	}
	if summary == "" {
		t.Error("summary is empty")
	}
}

func TestDecideFailsOnAFailingBot(t *testing.T) {
	passed, _ := decide([]prCheck{
		{Name: "CodeRabbit", Bucket: "fail"},
	})
	if passed {
		t.Error("passed = true, want false")
	}
}

func TestDecideFailsIfAnyBotIsBlockingEvenWhenOthersPass(t *testing.T) {
	passed, _ := decide([]prCheck{
		{Name: "CodeRabbit", Bucket: "pass"},
		{Name: "Bugbot", Bucket: "pending"},
	})
	if passed {
		t.Error("passed = true, want false")
	}
}

func TestDecideMatchesBotNamesCaseInsensitively(t *testing.T) {
	passed, summary := decide([]prCheck{{Name: "CODERABBIT", Bucket: "fail"}})
	if passed {
		t.Errorf("a differently-cased bot name was not matched: %s", summary)
	}
}

func TestReviewBotsNeedsACommand(t *testing.T) {
	if _, err := (ReviewBots{}).Verify(t.Context(), Request{}); err == nil {
		t.Fatal("Verify accepted an empty command")
	}
}

// A real gh invocation with malformed output must not pass. sh -c "echo notjson"
// stands in for gh printing something unparseable.
func TestReviewBotsFailsClosedOnUnparseableOutput(t *testing.T) {
	command, args := shell("echo not-json-at-all")
	result, err := (ReviewBots{}).Verify(t.Context(), Request{Command: command, Args: args})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Error("unparseable output passed")
	}
	if result.Check.Source != state.SourceCIAPI {
		t.Errorf("source = %q, want ci_api", result.Check.Source)
	}
}

// Real JSON, containing double quotes, does not survive both sh -c and
// cmd /c the same way through an inline echo, so the fixture is a file
// instead — read back with cat or type, whichever the platform has.
func TestReviewBotsPassesOnRealJSONOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checks.json")
	if err := os.WriteFile(path, []byte(`[{"name":"CodeRabbit","bucket":"pass"}]`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	readCmd := "cat " + path
	if goruntime.GOOS == "windows" {
		readCmd = "type " + path
	}
	command, args := shell(readCmd)
	result, err := (ReviewBots{}).Verify(t.Context(), Request{Command: command, Args: args})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Check.Passed {
		t.Errorf("did not pass: %s", result.Summary)
	}
}

func TestTheRegistryOffersReviewBots(t *testing.T) {
	v, err := Default().Get("reviewbots")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Kind() != "reviewbots" {
		t.Errorf("Kind() = %q, want reviewbots", v.Kind())
	}
}
