package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reminder is worth its context only if it stays out of ordinary prompts.
func TestTheAuthoringReminderFiresOnAssetWorkAndNothingElse(t *testing.T) {
	for _, prompt := range []string{
		"add a new skill for database migrations",
		"rename the doctor command",
		"which skill should I use for this?",
		"update the hooks router",
		"routing for a new subagent",
	} {
		if authoringContext(prompt) == "" {
			t.Errorf("no reminder for asset work: %q", prompt)
		}
	}

	for _, prompt := range []string{
		"why is this test failing?",
		"the skill worked fine yesterday",
		"add a retry to the HTTP client",
		"",
	} {
		if authoringContext(prompt) != "" {
			t.Errorf("reminder fired on an ordinary prompt: %q", prompt)
		}
	}
}

// RE2's \b covers ASCII only, so a verb starting with a letter it does not
// count as a word character never matches after a space. `đổi tên` is that
// case, and it is why the patterns consume their boundaries.
func TestVietnamesePromptsAreRecognisedDespiteASCIIWordBoundaries(t *testing.T) {
	for _, prompt := range []string{
		"tạo thêm một skill mới",
		"sửa lại command doctor",
		"đổi tên cái hook này",
		"xóa reference không dùng",
		"cập nhật router cho agent",
	} {
		if authoringContext(prompt) == "" {
			t.Errorf("no reminder for Vietnamese asset work: %q", prompt)
		}
	}
}

// The script this replaces printed a shape no host reads, so its text reached
// nobody. Whatever else changes, the line has to end up in what a session sees.
func TestANewSessionIsPointedAtTheRouterFlow(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, filepath.FromSlash(MetaSkill))
	if err := os.MkdirAll(filepath.Dir(skill), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("# meta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	text := sessionContext(Request{WorkspaceRoot: root, ToolkitRoot: root})
	if !strings.Contains(text, MetaSkill) {
		t.Errorf("session context does not name the meta-skill: %q", text)
	}

	// A pointer, not the file. The script pasted the whole SKILL.md into every
	// session, which is the copy "one source of truth, referenced" forbids.
	if strings.Contains(text, "# meta") {
		t.Error("session context inlined the meta-skill instead of linking it")
	}
}

func TestAToolkitWithoutTheMetaSkillSaysNothingAboutIt(t *testing.T) {
	if line := metaSkillLine(t.TempDir()); line != "" {
		t.Errorf("named a meta-skill that is not there: %q", line)
	}
	if line := metaSkillLine(""); line != "" {
		t.Errorf("named a meta-skill with no toolkit root: %q", line)
	}
}
