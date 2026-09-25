package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ducnd58233/vibe-agent/runtime/internal/memory"
)

func stdoutOf(t *testing.T, run func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := run()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = oldStdout
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return out.String(), runErr
}

func storedMemories(t *testing.T, root string) []memory.Record {
	t.Helper()
	store, err := memory.Open(t.Context(), root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()
	records, err := store.List(t.Context(), memory.WorkspaceKey(root))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return records
}

func TestMemoryProposeStoresThroughThePolicyWithItsAuthor(t *testing.T) {
	root := t.TempDir()
	_, err := stdoutOf(t, func() error {
		return memoryCommand([]string{"propose", "--workspace", root,
			"--kind", "correction", "--content", "gh pr checks exits 8 while checks are pending, not on failure",
			"--evidence", "gh pr checks 184 printed pending and exited 8",
			"--source-type", "command_result", "--client", "claude-code", "--model", "claude-opus-5-5"})
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	records := storedMemories(t, root)
	if len(records) != 1 {
		t.Fatalf("want one memory, got %d", len(records))
	}
	if records[0].Status != memory.StatusProposed {
		t.Errorf("status = %s; a proposal must never land confirmed", records[0].Status)
	}
	if records[0].CreatedBy != "claude-code/claude-opus-5-5" {
		t.Errorf("CreatedBy = %q", records[0].CreatedBy)
	}
}

func TestMemoryProposeRefusesACredential(t *testing.T) {
	root := t.TempDir()
	_, err := stdoutOf(t, func() error {
		return memoryCommand([]string{"propose", "--workspace", root,
			"--kind", "semantic",
			"--content", "staging api_key = sk-0123456789abcdef0123", // vibe-agent: allow-credential-literal (fake fixture proving the filter rejects secrets)
			"--evidence", "read from the deploy config", "--source-type", "file_content"})
	})
	if err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("a credential was not refused: %v", err)
	}
	if got := storedMemories(t, root); len(got) != 0 {
		t.Errorf("a rejected proposal was stored: %v", got)
	}
}

func TestMemoryPromotionsListsOnlyMemoriesUsedEnough(t *testing.T) {
	root := t.TempDir()
	store, err := memory.Open(t.Context(), root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	ids := map[string]string{}
	for _, content := range []string{"the api listens on port 8080", "the build needs node 20"} {
		stored, decision, err := store.Propose(t.Context(), memory.Record{
			WorkspaceID: memory.WorkspaceKey(root), Kind: memory.KindSemantic, Content: content,
			Confidence: 0.8, SourceType: memory.SourceCommandResult,
			Evidence: []string{"observed while running the suite: " + content},
		}, now)
		if err != nil || decision.Verdict == memory.VerdictReject {
			t.Fatalf("propose %q: %v %s", content, err, decision.Reason)
		}
		if _, err := store.Confirm(t.Context(), stored.ID, memory.SourceCommandResult, "events#1", now); err != nil {
			t.Fatalf("confirm: %v", err)
		}
		ids[content] = stored.ID
	}
	for range memory.PromotionThreshold {
		if err := store.RecordUse(t.Context(), ids["the api listens on port 8080"], now); err != nil {
			t.Fatalf("RecordUse: %v", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	out, err := stdoutOf(t, func() error { return memoryCommand([]string{"promotions", "--workspace", root}) })
	if err != nil {
		t.Fatalf("promotions: %v", err)
	}
	if !strings.Contains(out, ids["the api listens on port 8080"]) {
		t.Errorf("a memory used %d times was not proposed: %s", memory.PromotionThreshold, out)
	}
	if strings.Contains(out, ids["the build needs node 20"]) {
		t.Errorf("an unused memory was proposed: %s", out)
	}
}
