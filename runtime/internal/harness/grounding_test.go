package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTranscript builds a JSONL transcript and returns a payload aimed at it.
func writeTranscript(t *testing.T, lines ...string) payload {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var body payload
	body.TranscriptPath = path
	return body
}

// The whole point: a path in the final message that appears in no tool call is
// the thing the grounding rule forbids, and the rule was prose until now.
func TestAPathCitedButNeverOpenedIsReported(t *testing.T) {
	body := writeTranscript(t,
		`{"tool_input":{"file_path":"src/real.go"}}`,
		`{"message":{"content":[{"type":"text","text":"I read src/real.go and src/invented.go"}]}}`,
	)

	report := groundingReport(body)
	if !strings.Contains(report, "src/invented.go") {
		t.Errorf("the invented path was not reported: %q", report)
	}
	if strings.Contains(report, "src/real.go") {
		t.Errorf("a path that was opened was reported: %q", report)
	}
}

func TestAPathOpenedAnywhereInTheTranscriptIsGrounded(t *testing.T) {
	for _, evidence := range []string{
		`{"tool_input":{"path":"docs/guide.md"}}`,
		`{"tool_input":{"notebook_path":"docs/guide.md"}}`,
		// Tool results name files as prose rather than as fields.
		`{"tool_result":"Found 2 matches in docs/guide.md"}`,
	} {
		body := writeTranscript(t, evidence,
			`{"content":[{"type":"text","text":"docs/guide.md explains it"}]}`)
		if report := groundingReport(body); report != "" {
			t.Errorf("evidence %s was not credited: %q", evidence, report)
		}
	}
}

// Reporting a path as unreachable is the behaviour the rule asks for, so it
// must not be what trips the sensor.
func TestAnAccessFailedPathIsNotAFinding(t *testing.T) {
	body := writeTranscript(t,
		`{"tool_input":{"file_path":"src/real.go"}}`,
		`{"content":[{"type":"text","text":"ACCESS-FAILED: src/missing.go"}]}`,
	)
	if report := groundingReport(body); report != "" {
		t.Errorf("a correctly reported failure was flagged: %q", report)
	}
}

// Only the path attached to the marker is excused, not everything near it.
func TestAccessFailedExcusesOnlyItsOwnPath(t *testing.T) {
	body := writeTranscript(t,
		`{"content":[{"type":"text","text":"ACCESS-FAILED: a/missing.go and also b/invented.go"}]}`,
	)
	report := groundingReport(body)
	if strings.Contains(report, "a/missing.go") {
		t.Errorf("the excused path was reported: %q", report)
	}
	if !strings.Contains(report, "b/invented.go") {
		t.Errorf("a path sharing the line was excused: %q", report)
	}
}

// The final message is what is being checked, so its own text cannot count as
// evidence that the paths in it were opened.
func TestTheFinalMessageIsNotEvidenceForItself(t *testing.T) {
	body := writeTranscript(t,
		`{"content":[{"type":"text","text":"see src/only-mentioned.go"}]}`,
	)
	if report := groundingReport(body); !strings.Contains(report, "src/only-mentioned.go") {
		t.Errorf("a self-citing message passed: %q", report)
	}
}

// A relative mention of a path opened absolutely is the same file.
func TestARelativeMentionOfAnAbsolutePathIsGrounded(t *testing.T) {
	body := writeTranscript(t,
		`{"tool_input":{"file_path":"/home/me/project/src/app.go"}}`,
		`{"content":[{"type":"text","text":"src/app.go holds it"}]}`,
	)
	if report := groundingReport(body); report != "" {
		t.Errorf("a relative mention was flagged: %q", report)
	}
}

// Windows sends backslashes and the citation will not, so both are normalised
// before they are compared.
func TestAWindowsPathMatchesItsForwardSlashCitation(t *testing.T) {
	body := writeTranscript(t,
		`{"tool_input":{"file_path":"src\\infra\\store.go"}}`,
		`{"content":[{"type":"text","text":"src/infra/store.go does the write"}]}`,
	)
	if report := groundingReport(body); report != "" {
		t.Errorf("a backslash path was not matched: %q", report)
	}
}

// A blocked Stop has already had its turn; saying it again is how this loops.
func TestNothingIsReportedOnTheSecondStop(t *testing.T) {
	body := writeTranscript(t,
		`{"content":[{"type":"text","text":"src/invented.go"}]}`,
	)
	body.StopHookActive = true
	if report := groundingReport(body); report != "" {
		t.Errorf("reported on a repeat stop: %q", report)
	}
}

// Every unfamiliar input is silence, because this runs after the work is done
// and has nothing left to protect.
func TestAnUnreadableOrUnfamiliarTranscriptSaysNothing(t *testing.T) {
	var missing payload
	missing.TranscriptPath = filepath.Join(t.TempDir(), "gone.jsonl")
	if report := groundingReport(missing); report != "" {
		t.Errorf("missing transcript produced %q", report)
	}

	var none payload
	if report := groundingReport(none); report != "" {
		t.Errorf("payload with no transcript produced %q", report)
	}

	garbage := writeTranscript(t, "not json at all", `{"unfamiliar":true}`)
	if report := groundingReport(garbage); report != "" {
		t.Errorf("unfamiliar transcript produced %q", report)
	}
}

// Cursor names the same field differently, and reading only Claude's spelling
// would make this a no-op on that host.
func TestCursorsTranscriptFieldIsRead(t *testing.T) {
	base := writeTranscript(t,
		`{"content":[{"type":"text","text":"src/invented.go"}]}`,
	)
	var body payload
	body.AgentTranscriptPath = base.TranscriptPath

	if report := groundingReport(body); !strings.Contains(report, "src/invented.go") {
		t.Errorf("Cursor's field was not read: %q", report)
	}
}

// One bad turn should not fill the transcript with its own diagnosis.
func TestTheReportIsCapped(t *testing.T) {
	var cited []string
	for index := 0; index < maxReported+5; index++ {
		cited = append(cited, "src/file"+string(rune('a'+index))+".go")
	}
	body := writeTranscript(t,
		`{"content":[{"type":"text","text":"`+strings.Join(cited, " ")+`"}]}`,
	)

	report := groundingReport(body)
	if !strings.Contains(report, "and 5 more") {
		t.Errorf("the report was not capped: %q", report)
	}
	if got := strings.Count(report, "\n  - "); got != maxReported+1 {
		t.Errorf("listed %d lines, want %d plus the summary", got, maxReported)
	}
}
