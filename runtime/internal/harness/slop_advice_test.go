package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSubject puts a file in a workspace and returns the request and subject
// the advisory would be given for it.
func writeSubject(t *testing.T, name, body string) (Request, subject) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return Request{WorkspaceRoot: root, Client: ClientClaude},
		subject{Path: name, Text: body, Language: "Go", Type: "programming"}
}

// A call whose result is assigned to the blank identifier is a medium finding,
// which is the severity this channel carries. The shape matters: `_ = value`
// does not fire, `_ = call()` does.
const discardedResult = `package demo

import "os"

func Demo() {
	file, _ := os.Open("x")
	defer func() { _ = file.Close() }()
}
`

func TestSlopAdviceReportsAMediumFinding(t *testing.T) {
	req, file := writeSubject(t, "demo.go", discardedResult)

	advice := slopAdvice(req, file)
	if advice == "" {
		t.Fatal("a discarded call result produced no advice; the medium-severity path is not reaching this channel")
	}
	if !strings.Contains(advice, "ignored_result") {
		t.Errorf("the finding did not name its rule: %s", advice)
	}
	if !strings.Contains(advice, "[slop]") {
		t.Errorf("advice is not labelled: %s", advice)
	}
	if !strings.Contains(advice, "Advisory") {
		t.Errorf("advice does not say it refused nothing: %s", advice)
	}
}

// Low severity is dominated by duplicate_line, which fires on ordinary repeated
// table rows and fixtures. Reporting those after every edit is how a channel
// gets ignored, and this one is shared with the credential guard.
func TestSlopAdviceStaysQuietOnLowSeverityOnly(t *testing.T) {
	repeated := "package demo\n\nvar (\n" +
		strings.Repeat("\ta = 1\n", 12) +
		")\n"
	req, file := writeSubject(t, "repeated.go", repeated)

	if advice := slopAdvice(req, file); advice != "" {
		if !strings.Contains(advice, "medium") && !strings.Contains(advice, "high") {
			t.Errorf("low-severity findings reached the advice channel: %s", advice)
		}
	}
}

// A clean file must produce nothing at all. An advisory that fires on every
// write is noise whatever it says.
func TestSlopAdviceIsSilentOnACleanFile(t *testing.T) {
	req, file := writeSubject(t, "clean.go", "package demo\n\n// Answer is the answer.\nfunc Answer() int { return 42 }\n")

	if advice := slopAdvice(req, file); advice != "" {
		t.Errorf("a clean file produced advice: %s", advice)
	}
}

// The cost ceiling. This runs after every file write, so it is the one binding
// of the four that has to justify itself on latency. TASKS.md records that this
// binding is dropped if it is slow; the number is measured here rather than
// asserted in prose.
func TestSlopAdviceCostsLittlePerEdit(t *testing.T) {
	req, file := writeSubject(t, "demo.go", discardedResult)

	start := time.Now()
	for range 5 {
		slopAdvice(req, file)
	}
	perEdit := time.Since(start) / 5

	t.Logf("slop advice costs %v per edit", perEdit)
	if perEdit > slopAdviceTimeout {
		t.Errorf("advice took %v per edit, past its own %v ceiling", perEdit, slopAdviceTimeout)
	}
}

// A missing file must not panic or hang. The subject was read before this runs,
// but a file can be removed between the write and the hook.
func TestSlopAdviceSurvivesAMissingFile(t *testing.T) {
	req := Request{WorkspaceRoot: t.TempDir(), Client: ClientClaude}
	file := subject{Path: "gone.go", Text: "package demo\n", Language: "Go", Type: "programming"}

	// The auditor reports an unreadable file as a high-severity scan_error
	// finding on line 1, measured rather than assumed. That is a fact about the
	// scan and not about the code, and announcing it here would name a file that
	// is not there as a slop problem.
	if advice := slopAdvice(req, file); advice != "" {
		t.Errorf("a missing file produced advice: %s", advice)
	}
}
