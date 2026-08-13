package verifier

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Installing Android Studio sets ANDROID_HOME and leaves PATH alone, so calling
// adb by name failed on an ordinary developer machine with a running emulator.
// The failure read as a missing tool rather than as a lookup this code never
// tried, which is the kind of thing only running it on real hardware finds.
func TestAdbIsFoundUnderAnSDKRootWhenNotOnPath(t *testing.T) {
	root := t.TempDir()
	tools := filepath.Join(root, "platform-tools")
	if err := os.MkdirAll(tools, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	name := "adb"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\n"), 0o750); err != nil {
		t.Fatalf("write: %v", err)
	}

	found := ToolInSDK("adb", []string{root})
	if found == "" {
		t.Fatal("adb under an SDK root was not found")
	}
	if !strings.HasPrefix(found, root) {
		t.Errorf("found %q, which is not under the SDK root given", found)
	}

	if ToolInSDK("adb", []string{t.TempDir()}) != "" {
		t.Error("an SDK root with no platform-tools reported a hit")
	}
	if ToolInSDK("adb", nil) != "" {
		t.Error("no roots reported a hit")
	}
}

// attachedDevice returns the first adb serial, or skips.
//
// The only test here that touches real hardware. Everything else about the
// screen verifier runs against a fake, which proves the logic and not the
// plumbing; adb's actual output shape is the part a fake cannot check.
func attachedDevice(t *testing.T) string {
	t.Helper()
	out, err := exec.Command(adbCommand(), "devices").Output()
	if err != nil {
		t.Skipf("no adb available: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "device" {
			return fields[0]
		}
	}
	t.Skip("no device or emulator attached")
	return ""
}

// The plumbing test. It asserts that real device output survives the parsing,
// which is where a fake cannot help: the fixture XML in screen_test.go is
// whatever shape this file's author imagined.
func TestScreenVerifierAgainstAnAttachedDevice(t *testing.T) {
	serial := attachedDevice(t)
	device := android{serial: serial, dir: t.TempDir()}
	ctx := t.Context()

	tree, err := device.ViewHierarchy(ctx)
	if err != nil {
		t.Fatalf("ViewHierarchy: %v", err)
	}
	if !strings.Contains(tree, "<hierarchy") || !strings.Contains(tree, "<node") {
		t.Errorf("the dump does not look like a view hierarchy: %.200q", tree)
	}

	shot, err := device.Screenshot(ctx)
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	measured, err := Blank(shot)
	if err != nil {
		t.Fatalf("Blank on a real screenshot: %v", err)
	}
	t.Logf("%s: %d bytes, %s", serial, len(shot), measured.Describe())

	// Content that cannot be on any screen must fail. This is the assertion a
	// fake cannot make honestly, because the fixture was written to contain what
	// the test looks for.
	spec := ScreenSpec{
		Platform:   PlatformAndroid,
		Device:     serial,
		ExpectText: []string{"vibe-agent-string-that-is-on-no-screen"},
	}
	result, err := Screen{Device: device}.Verify(ctx, Request{
		Check: "e2e", WorkspaceRoot: t.TempDir(), Screen: &spec,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Check.Passed {
		t.Errorf("content that is on no screen was reported present: %s", result.Summary)
	}
}
