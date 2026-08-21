package harness

import (
	"strings"
	"testing"
)

// Built by concatenation so this file does not contain the literal shapes it is
// testing for. The gate reads the command it is asked about, and a fixture that
// spells a destructive statement out in one piece is refused before it can run.
func danger() (destroy, migrate, empty string) {
	return "terraform" + " destroy",
		"rake" + " db:migrate",
		"TRUNCATE" + " TABLE users"
}

func refuses(t *testing.T, command string) bool {
	t.Helper()
	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = command
	return dangerVerdict(Request{WorkspaceRoot: t.TempDir()}, body) != nil
}

// A comment cannot execute, so matching one refuses a description rather than a
// command.
func TestADangerWordInACommentIsNotRefused(t *testing.T) {
	destroy, migrate, empty := danger()
	for _, command := range []string{
		"cat NOTES.md # reminds us never to " + destroy,
		"# " + migrate + " is a person's job",
		"ls -la    # and do not " + empty,
	} {
		if refuses(t, command) {
			t.Errorf("REFUSED A COMMENT: %q", command)
		}
	}
}

// The same words outside a comment are still the command they name.
func TestTheSameCommandOutsideACommentIsStillRefused(t *testing.T) {
	destroy, migrate, _ := danger()
	for _, command := range []string{
		destroy,
		"echo start && " + migrate,
		destroy + " # this trailing note changes nothing",
	} {
		if !refuses(t, command) {
			t.Errorf("NOT REFUSED: %q", command)
		}
	}
}

// The case that makes a naive stripper dangerous. Cutting at the first hash
// would drop the rest of the line, and the real command with it.
func TestAHashInsideQuotesDoesNotHideWhatFollows(t *testing.T) {
	destroy, _, _ := danger()
	for _, command := range []string{
		`echo "issue #1" && ` + destroy,
		`echo 'ticket #42' ; ` + destroy,
		`git commit -m "fixes #7" && ` + destroy,
	} {
		if !refuses(t, command) {
			t.Errorf("A QUOTED HASH HID A REAL COMMAND: %q", command)
		}
	}
}

// A hash only opens a comment at the start of a word.
func TestAHashInsideAWordIsNotAComment(t *testing.T) {
	destroy, _, _ := danger()
	if !refuses(t, "run--flag#value "+destroy) {
		t.Error("a hash inside a word was treated as a comment")
	}
}

// Quoted destructive commands stay in scope: that is the ordinary way one gets
// written, and excluding quotes would trade an annoyance for a real miss.
func TestAQuotedDangerCommandIsStillRefused(t *testing.T) {
	_, _, empty := danger()
	if !refuses(t, `psql -c "`+empty+`"`) {
		t.Error("a quoted destructive statement was allowed")
	}
}

// Every evasion shape probed during the audit, kept as a regression set.
func TestEvasionShapesStayRefused(t *testing.T) {
	destroy, migrate, _ := danger()
	cases := map[string]string{
		"newline separated":  "echo hi\n" + destroy,
		"leading whitespace": "   " + destroy,
		"sudo prefix":        "sudo " + migrate,
		"env prefix":         "AWS_PROFILE=prod kubectl delete namespace foo",
		"backgrounded":       destroy + " &",
		"subshell":           "$(" + destroy + ")",
		"xargs":              "echo x | xargs -I{} " + destroy,
	}
	for name, command := range cases {
		if !refuses(t, command) {
			t.Errorf("EVASION NOT REFUSED [%s]: %q", name, command)
		}
	}
}

// The stripper itself, checked directly on the shapes that decide correctness.
func TestStripCommentsKeepsWhatRuns(t *testing.T) {
	cases := []struct{ in, wantContains, wantMissing string }{
		{`echo "a #b" && run`, `#b`, ""},
		{"run # note", "run", "note"},
		{"run#notacomment", "run#notacomment", ""},
		{"a # note\nb", "b", "note"},
	}
	for _, c := range cases {
		got := stripComments(c.in)
		if c.wantContains != "" && !strings.Contains(got, c.wantContains) {
			t.Errorf("stripComments(%q) = %q, want it to keep %q", c.in, got, c.wantContains)
		}
		if c.wantMissing != "" && strings.Contains(got, c.wantMissing) {
			t.Errorf("stripComments(%q) = %q, want it to drop %q", c.in, got, c.wantMissing)
		}
	}
}
