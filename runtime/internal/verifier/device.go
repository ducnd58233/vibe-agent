package verifier

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Supported platform values for a screen check.
const (
	PlatformAndroid = "android"
	PlatformIOS     = "ios"
)

// NewDevice builds the toolchain adapter for a spec.
func NewDevice(spec ScreenSpec, workspaceRoot string) (Device, error) {
	switch spec.Platform {
	case PlatformAndroid:
		return android{serial: spec.Device, dir: workspaceRoot}, nil
	case PlatformIOS:
		return simulator{udid: spec.Device, dir: workspaceRoot}, nil
	case "":
		return nil, fmt.Errorf("screen check needs a platform: %s or %s", PlatformAndroid, PlatformIOS)
	default:
		return nil, fmt.Errorf("unknown screen platform %q; use %s or %s", spec.Platform, PlatformAndroid, PlatformIOS)
	}
}

// AndroidSDKEnv are the variables that name an SDK location, in the order the
// Android tooling itself prefers them.
var AndroidSDKEnv = []string{"ANDROID_HOME", "ANDROID_SDK_ROOT"}

// adbCommand resolves the adb executable.
//
// Installing Android Studio does not put platform-tools on PATH, which is the
// common case rather than a broken one: the SDK sets ANDROID_HOME and leaves PATH
// alone. Calling "adb" by name therefore failed on an ordinary developer machine
// with a running emulator, and the failure looked like a missing tool rather than
// a lookup this code never tried.
//
// PATH comes first so an explicitly installed adb wins over whatever an SDK
// happens to ship.
func adbCommand() string {
	if resolved, err := exec.LookPath("adb"); err == nil {
		return resolved
	}
	var roots []string
	for _, key := range AndroidSDKEnv {
		if value := os.Getenv(key); value != "" {
			roots = append(roots, value)
		}
	}
	if found := ToolInSDK("adb", roots); found != "" {
		return found
	}
	// Fall back to the bare name so the error names the tool the reader expects,
	// rather than an empty command.
	return "adb"
}

// ToolInSDK looks for a platform-tools executable under the given SDK roots.
//
// Exported so the lookup can be tested without an SDK installed, which is the
// only way it gets tested in CI.
func ToolInSDK(name string, roots []string) string {
	names := []string{name}
	if runtime.GOOS == "windows" {
		// A direct file check does not consult PATHEXT the way LookPath does.
		names = []string{name + ".exe", name}
	}
	for _, root := range roots {
		for _, candidate := range names {
			path := filepath.Join(root, "platform-tools", candidate)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}
	return ""
}

// capture runs a command and returns stdout, with stderr folded into the error
// so a failure says what the tool actually printed.
func capture(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(errs.String())
		if detail == "" {
			detail = strings.TrimSpace(out.String())
		}
		if detail != "" {
			return out.Bytes(), fmt.Errorf("%s: %w: %s", name, err, detail)
		}
		return out.Bytes(), fmt.Errorf("%s: %w", name, err)
	}
	return out.Bytes(), nil
}

// android drives an emulator or a handset over adb.
type android struct {
	serial string
	dir    string
}

func (a android) Name() string {
	if a.serial == "" {
		return "android (default device)"
	}
	return "android " + a.serial
}

// adb prefixes the serial when one was given, so a machine with two devices
// attached does not fail with adb's "more than one device" error.
func (a android) adb(args ...string) []string {
	if a.serial == "" {
		return args
	}
	return append([]string{"-s", a.serial}, args...)
}

func (a android) ClearCrashLog(ctx context.Context) error {
	_, err := capture(ctx, a.dir, adbCommand(), a.adb("logcat", "-b", "crash", "-c")...)
	return err
}

func (a android) Launch(ctx context.Context, command string, args []string) error {
	_, err := capture(ctx, a.dir, command, args...)
	return err
}

// CrashLog reads the crash buffer, which is separate from the main log and holds
// only FATAL EXCEPTION, ANR, and tombstone records. Reading the main buffer would
// mean sifting a working app's own logging for words like "error".
func (a android) CrashLog(ctx context.Context) (string, error) {
	out, err := capture(ctx, a.dir, adbCommand(), a.adb("logcat", "-b", "crash", "-d")...)
	return string(out), err
}

// ViewHierarchy dumps the uiautomator tree.
//
// The dump goes to a file on the device and is then read back, because that is
// the only form uiautomator offers. Two things it cannot see, both worth knowing
// before trusting a pass:
//
//   - A Flutter app renders to a single canvas, and that canvas is not accessible
//     unless the app has semantics enabled. The dump then shows one node and no
//     content, so a Flutter check needs its assertions inside the app instead.
//   - A screen mid-animation can make uiautomator report that it never became
//     idle, which surfaces here as an error rather than as missing content.
func (a android) ViewHierarchy(ctx context.Context) (string, error) {
	const remote = "/sdcard/window_dump.xml"
	if _, err := capture(ctx, a.dir, adbCommand(), a.adb("shell", "uiautomator", "dump", remote)...); err != nil {
		return "", err
	}
	out, err := capture(ctx, a.dir, adbCommand(), a.adb("shell", "cat", remote)...)
	if err != nil {
		return "", err
	}
	dump := string(out)
	if !strings.Contains(dump, "<hierarchy") {
		return "", fmt.Errorf("the dump is not a view hierarchy, which is what a canvas-rendered app looks like here: %.120q", dump)
	}
	return dump, nil
}

// Screenshot uses exec-out rather than shell, so the PNG arrives as bytes without
// adb's line-ending translation corrupting it.
func (a android) Screenshot(ctx context.Context) ([]byte, error) {
	return capture(ctx, a.dir, adbCommand(), a.adb("exec-out", "screencap", "-p")...)
}

// simulator drives an iOS simulator over xcrun simctl.
type simulator struct {
	udid string
	dir  string
}

func (s simulator) Name() string {
	if s.udid == "" {
		return "ios (booted simulator)"
	}
	return "ios " + s.udid
}

func (s simulator) target() string {
	if s.udid == "" {
		return "booted"
	}
	return s.udid
}

// ClearCrashLog is a no-op: simctl exposes no equivalent of a clearable crash
// buffer. CrashLog reads the diagnostic reports directory instead, which is
// cumulative, so a stale report can outlive the run that produced it.
func (s simulator) ClearCrashLog(context.Context) error { return nil }

func (s simulator) Launch(ctx context.Context, command string, args []string) error {
	_, err := capture(ctx, s.dir, command, args...)
	return err
}

func (s simulator) CrashLog(ctx context.Context) (string, error) {
	out, err := capture(ctx, s.dir, "xcrun", "simctl", "spawn", s.target(),
		"log", "show", "--last", "2m", "--predicate", "eventType == 'faultEvent'")
	return string(out), err
}

// ViewHierarchy is not available. simctl has no uiautomator equivalent, so the
// content assertion has to come from inside the app on iOS: an XCUITest, or a
// framework-level integration test that queries its own widget tree.
//
// Returning an error rather than an empty string is deliberate. Empty would read
// as "the screen has no expected content", which is a claim; this is an absence
// of measurement, and Verify treats the two differently.
func (s simulator) ViewHierarchy(context.Context) (string, error) {
	return "", fmt.Errorf("simctl exposes no view hierarchy; assert content from inside the app on %s", PlatformIOS)
}

func (s simulator) Screenshot(ctx context.Context) ([]byte, error) {
	// simctl writes to a file rather than stdout, so it needs a real path.
	file, err := os.CreateTemp("", "simctl-*.png")
	if err != nil {
		return nil, fmt.Errorf("create screenshot file: %w", err)
	}
	path := file.Name()
	file.Close()
	defer os.Remove(path)

	if _, err := capture(ctx, s.dir, "xcrun", "simctl", "io", s.target(), "screenshot", path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// writeScreenshot keeps the frame beside the log, so a failure can be looked at
// rather than only read about.
func writeScreenshot(req Request, png []byte) (string, error) {
	if req.Slug == "" || req.LogDir == "" || len(png) == 0 {
		return "", nil
	}
	dir := filepath.Join(req.WorkspaceRoot, "tmp", req.Slug, req.LogDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := req.Check
	if name == "" {
		name = "screen"
	}
	path := filepath.Join(dir, name+".png")
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(req.WorkspaceRoot, path)
	if err != nil {
		return path, nil
	}
	return relative, nil
}
