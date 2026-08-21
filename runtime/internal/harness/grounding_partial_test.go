package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A line past the scanner's cap ends the loop exactly as a clean end-of-file
// does. Without a check the caller cannot tell a whole transcript from most of
// one, and the report it builds is evidence a gate reads.
func TestAPartialTranscriptReadIsReportedAsPartial(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "transcript.jsonl")

	// One good line, then one longer than the 8 MB ceiling the scanner sets.
	var content strings.Builder
	content.WriteString(`{"role":"assistant","text":"see docs/only-here.md"}` + "\n")
	content.WriteString(`{"filler":"`)
	content.WriteString(strings.Repeat("x", 9*1024*1024))
	content.WriteString(`"}` + "\n")
	if err := os.WriteFile(path, []byte(content.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, whole := readTranscript(path)
	if whole {
		t.Error("a transcript that could not be read to the end reported itself as whole")
	}
}

// A transcript that reads cleanly must still say so, or the caveat appears on
// every report and stops meaning anything.
func TestACompleteTranscriptReadIsReportedAsWhole(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "transcript.jsonl")
	if err := os.WriteFile(path,
		[]byte(`{"role":"assistant","text":"hello"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, texts, whole := readTranscript(path)
	if !whole {
		t.Error("a complete read reported itself as partial")
	}
	if len(texts) == 0 {
		t.Error("a complete read produced no text")
	}
}

// The caveat has to say the finding may be an artefact, because that is the
// direction a partial read fails in: `observed` is the set of paths the
// transcript proved were opened, so a path can be listed precisely because its
// proof was in the part that did not load.
func TestThePartialCaveatSaysTheFindingMayBeAnArtefact(t *testing.T) {
	partial := groundingMessage([]string{"docs/a.md"}, false)
	whole := groundingMessage([]string{"docs/a.md"}, true)

	if !strings.Contains(partial, "could not be read to the end") {
		t.Errorf("the caveat does not say the read was short:\n%s", partial)
	}
	if !strings.Contains(partial, "prompt to check rather than a finding") {
		t.Errorf("the caveat does not soften the verdict:\n%s", partial)
	}
	if strings.Contains(whole, "could not be read to the end") {
		t.Errorf("a complete read carried the caveat anyway:\n%s", whole)
	}
	if !strings.Contains(whole, "docs/a.md") || !strings.Contains(partial, "docs/a.md") {
		t.Error("the cited path is missing from a report")
	}
}

// A file that cannot be opened is not a partial read of a file that exists, but
// it must not claim to be whole either.
func TestAnUnreadableTranscriptIsNotReportedAsWhole(t *testing.T) {
	_, _, whole := readTranscript(filepath.Join(t.TempDir(), "absent.jsonl"))
	if whole {
		t.Error("a transcript that could not be opened reported itself as whole")
	}
}
