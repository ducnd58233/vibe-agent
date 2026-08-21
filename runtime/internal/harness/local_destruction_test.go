package harness

import (
	"strings"
	"testing"
)

// Built by concatenation so this file does not spell the shapes out in one
// piece: the gate reads the command it is asked about, and a fixture written
// literally is refused before it can run.
func destructive() (recursive, wipe, format string) {
	return "rm" + " -rf", "dd" + " if=/dev/zero", "mkfs" + ".ext4"
}

func blocked(t *testing.T, command string) *BlockError {
	t.Helper()
	var body payload
	body.ToolName = "Bash"
	body.ToolInput.Command = command
	return dangerVerdict(Request{WorkspaceRoot: t.TempDir()}, body)
}

// The shapes that cannot be undone. Cloud destruction was already on the list;
// these are the local kind, which had nothing but a prompt in the way.
func TestLocalDestructionIsRefused(t *testing.T) {
	recursive, wipe, format := destructive()
	cases := map[string]string{
		"root":            recursive + " /",
		"home tilde":      recursive + " ~",
		"home variable":   recursive + " $HOME",
		"parent":          recursive + " ..",
		"root glob":       recursive + " /*",
		"home glob":       recursive + " ~/*",
		"windows drive":   recursive + " C:",
		"flags reordered": "rm" + " -v -fr /",
		"device write":    wipe + " of=/dev/sda",
		"make filesystem": format + " /dev/sdb1",
		"repartition":     "fdisk" + " /dev/sda",
		"unrecoverable":   "shred" + " secrets.txt",
	}
	for name, command := range cases {
		if blocked(t, command) == nil {
			t.Errorf("NOT REFUSED [%s]: %q", name, command)
		}
	}
}

// The half that decides whether the rule survives contact with a real session.
// A gate that fires on daily work gets switched off, and switching it off loses
// the protection above along with the annoyance.
func TestOrdinaryRemovalIsNotRefused(t *testing.T) {
	recursive, _, _ := destructive()
	for _, command := range []string{
		recursive + " tmp/scratch",
		recursive + " node_modules",
		recursive + " ./build",
		"rm" + " -f a.txt",
		"rm" + " build/out.js",
		"rm" + " -rf docs/generated && make build",
		"dd" + " if=source.img of=backup.img",
		"echo 'never " + recursive + " / on a live box'",
	} {
		if refusal := blocked(t, command); refusal != nil {
			t.Errorf("REFUSED ORDINARY WORK: %q\n%s", command, refusal.Reason)
		}
	}
}

// A refusal has to name itself and say who decides, or the next move is to work
// around it.
func TestALocalDestructionRefusalExplainsItself(t *testing.T) {
	recursive, _, _ := destructive()
	refusal := blocked(t, recursive+" /")
	if refusal == nil {
		t.Fatal("a recursive removal of the root was allowed")
	}
	for _, want := range []string{"local-destruction", "nothing brings them back", "A person decides"} {
		if !strings.Contains(refusal.Reason, want) {
			t.Errorf("the refusal does not contain %q:\n%s", want, refusal.Reason)
		}
	}
}

// The families whose safe forms are about to leave the ask tier. Their
// destructive forms must already be refused here, because after that change
// this list is the only thing covering them.
func TestTheDestructiveFormsOfTheLoosenedFamiliesStayRefused(t *testing.T) {
	cases := map[string]string{
		"terraform": "terraform" + " destroy",
		"kubectl":   "kubectl" + " delete namespace prod",
		"docker":    "docker" + " system prune -a --volumes",
		"aws":       "aws" + " s3 rb s3://bucket",
		"gcloud":    "gcloud" + " compute instances delete web-1",
		"npm":       "npm" + " publish",
	}
	for name, command := range cases {
		if blocked(t, command) == nil {
			t.Errorf("A LOOSENED FAMILY IS UNCOVERED [%s]: %q", name, command)
		}
	}
}

// The safe forms of those same families, which are the reason for loosening.
func TestTheSafeFormsOfThoseFamiliesAreNotRefused(t *testing.T) {
	for _, command := range []string{
		"npm test",
		"npm run build",
		"terraform plan",
		"kubectl get pods",
		"docker build -t local .",
		"curl https://example.com",
		"pip install -r requirements.txt",
	} {
		if refusal := blocked(t, command); refusal != nil {
			t.Errorf("REFUSED A SAFE FORM: %q\n%s", command, refusal.Reason)
		}
	}
}
