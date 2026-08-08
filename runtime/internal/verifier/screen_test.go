package verifier

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/ducnd58233/vibe-agent/runtime/internal/state"
)

// fakeDevice stands in for a phone. Every field is what that call returns, so a
// test describes a device state rather than a sequence of commands.
type fakeDevice struct {
	crash     string
	crashErr  error
	hierarchy string
	treeErr   error
	shot      []byte
	shotErr   error
	launchErr error

	cleared  bool
	launched bool
}

func (f *fakeDevice) Name() string { return "fake" }
func (f *fakeDevice) ClearCrashLog(context.Context) error {
	f.cleared = true
	return nil
}
func (f *fakeDevice) Launch(context.Context, string, []string) error {
	f.launched = true
	return f.launchErr
}
func (f *fakeDevice) CrashLog(context.Context) (string, error) { return f.crash, f.crashErr }
func (f *fakeDevice) ViewHierarchy(context.Context) (string, error) {
	return f.hierarchy, f.treeErr
}
func (f *fakeDevice) Screenshot(context.Context) ([]byte, error) { return f.shot, f.shotErr }

// solid renders a single-colour PNG, which is what a failed launch leaves on a
// display.
func solid(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

// busy renders a PNG with enough variation that no single colour dominates.
func busy(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: uint8((x + y) * 2), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

const goodTree = `<?xml version='1.0'?><hierarchy rotation="0">
  <node resource-id="com.example:id/total" text="Total: 42.00" />
  <node resource-id="com.example:id/rows" text="3 items" />
</hierarchy>`

func screenSpec() ScreenSpec {
	return ScreenSpec{
		Platform:          PlatformAndroid,
		SettleSeconds:     1,
		ExpectText:        []string{"Total: 42.00"},
		ExpectResourceIDs: []string{"com.example:id/rows"},
		ForbidText:        []string{"Unhandled JS Exception"},
	}
}

func verifyScreen(t *testing.T, device Device, spec ScreenSpec) Result {
	t.Helper()
	result, err := Screen{Device: device}.Verify(context.Background(), Request{
		Check: "e2e", WorkspaceRoot: t.TempDir(), Screen: &spec,
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return result
}

// The reported bug, as a test. A white screen is what the user saw while the
// agent reported a pass. All three signals have to notice it.
func TestABlankScreenFailsEveryWay(t *testing.T) {
	white := &fakeDevice{
		// A launched-but-dead app has no crash record and an empty hierarchy.
		hierarchy: `<?xml version='1.0'?><hierarchy rotation="0" />`,
		shot:      solid(t, color.White),
	}
	result := verifyScreen(t, white, screenSpec())

	if result.Check.Passed {
		t.Fatalf("a blank white screen passed: %s", result.Summary)
	}
	for _, want := range []string{"Total: 42.00", "com.example:id/rows", "blank"} {
		if !strings.Contains(result.Summary, want) {
			t.Errorf("the summary does not mention %q: %s", want, result.Summary)
		}
	}
}

func TestARenderedScreenWithTheExpectedContentPasses(t *testing.T) {
	good := &fakeDevice{hierarchy: goodTree, shot: busy(t)}
	result := verifyScreen(t, good, screenSpec())

	if !result.Check.Passed {
		t.Fatalf("a correctly rendered screen failed: %s", result.Summary)
	}
	if !good.cleared {
		t.Error("the crash log was not cleared before launch, so a stale crash would fail this run")
	}
	if result.Check.Source != state.SourceFileAssert {
		t.Errorf("source = %q; a screen result is a set of device observations, not one exit code",
			result.Check.Source)
	}
}

// The case an exit-code verifier cannot see: the app is up, nothing crashed, the
// screen is busy, and it is showing the wrong numbers.
func TestAScreenShowingTheWrongDataFails(t *testing.T) {
	wrong := &fakeDevice{
		hierarchy: strings.Replace(goodTree, "Total: 42.00", "Total: 0.00", 1),
		shot:      busy(t),
	}
	result := verifyScreen(t, wrong, screenSpec())

	if result.Check.Passed {
		t.Fatal("a screen with the wrong total passed; content is the only signal that covers data")
	}
	if !strings.Contains(result.Summary, "Total: 42.00") {
		t.Errorf("the summary does not name the missing content: %s", result.Summary)
	}
}

// A framework error page is not an OS-level crash, so the crash log stays clean
// and the screen is busy. ForbidText is what catches it.
func TestAFrameworkErrorScreenFails(t *testing.T) {
	redbox := &fakeDevice{
		hierarchy: `<hierarchy><node text="Unhandled JS Exception: undefined is not an object" /></hierarchy>`,
		shot:      busy(t),
	}
	result := verifyScreen(t, redbox, screenSpec())

	if result.Check.Passed {
		t.Fatal("an error screen passed; it is not a crash, so only forbidden text catches it")
	}
	if !strings.Contains(result.Summary, "Unhandled JS Exception") {
		t.Errorf("the summary does not name the forbidden text: %s", result.Summary)
	}
}

// An unreadable signal is not a passing signal. A broken adb connection would
// otherwise look exactly like a healthy app.
func TestAnUnreadableSignalIsNotAPass(t *testing.T) {
	cases := []struct {
		name   string
		device *fakeDevice
		want   string
	}{
		{
			name:   "crash log unreadable",
			device: &fakeDevice{crashErr: errors.New("device offline"), hierarchy: goodTree, shot: busy(t)},
			want:   "crash log",
		},
		{
			name:   "hierarchy unreadable while content is asserted",
			device: &fakeDevice{treeErr: errors.New("not a view hierarchy"), shot: busy(t)},
			want:   "view hierarchy",
		},
		{
			name:   "screenshot unavailable",
			device: &fakeDevice{hierarchy: goodTree, shotErr: errors.New("screencap failed")},
			want:   "screenshot",
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := verifyScreen(t, test.device, screenSpec())
			if result.Check.Passed {
				t.Fatalf("an unmeasured signal passed: %s", result.Summary)
			}
			if !strings.Contains(result.Summary, test.want) {
				t.Errorf("the summary does not say what could not be read: %s", result.Summary)
			}
		})
	}
}

// A launch that failed is a different problem from a screen that rendered
// wrongly, and saying so saves the reader looking at the UI.
func TestAFailedLaunchIsReportedAsSuch(t *testing.T) {
	dead := &fakeDevice{launchErr: errors.New("Error: Activity not started"), shot: solid(t, color.Black)}
	spec := screenSpec()
	spec.Launch = "adb"
	spec.LaunchArgs = []string{"shell", "am", "start", "-n", "com.example/.Main"}

	result := verifyScreen(t, dead, spec)
	if result.Check.Passed {
		t.Fatal("a failed launch passed")
	}
	if !strings.Contains(result.Summary, "did not launch") {
		t.Errorf("the summary does not say the app never started: %s", result.Summary)
	}
}

// A spec that asserts nothing about content still checks crashes and blankness,
// but it must not read as a full pass: it proves the app rendered something.
func TestASpecWithNoContentAssertionSaysSo(t *testing.T) {
	bare := &fakeDevice{hierarchy: goodTree, shot: busy(t)}
	result := verifyScreen(t, bare, ScreenSpec{Platform: PlatformAndroid, SettleSeconds: 1})

	if !result.Check.Passed {
		t.Fatalf("a rendered screen with no assertions failed: %s", result.Summary)
	}
	if !strings.Contains(result.Summary, "no content asserted") {
		t.Errorf("the summary overstates what was proven: %s", result.Summary)
	}
	if !strings.Contains(result.Detail, "not that it rendered the right thing") {
		t.Errorf("the log does not warn that content went unchecked:\n%s", result.Detail)
	}
}

// The crash buffer opens with a banner even when nothing has crashed, so
// "non-empty" is the wrong test. This is the parsing that reads untrusted text
// off a device, which makes it the part most likely to be wrong.
func TestCrashFindingsIgnoreTheEmptyBufferBanner(t *testing.T) {
	if found := CrashFindings("--------- beginning of crash\n"); len(found) > 0 {
		t.Errorf("an empty crash buffer was read as a crash: %v", found)
	}

	fatal := `--------- beginning of crash
01-01 00:00:00.000  E AndroidRuntime: FATAL EXCEPTION: main
01-01 00:00:00.000  E AndroidRuntime: java.lang.NullPointerException`
	if found := CrashFindings(fatal); len(found) == 0 {
		t.Error("a FATAL EXCEPTION was not reported")
	}

	anr := "01-01 00:00:00.000  E ActivityManager: ANR in com.example (com.example/.Main)"
	if found := CrashFindings(anr); len(found) == 0 {
		t.Error("an ANR was not reported")
	}
}

func TestBlankDetection(t *testing.T) {
	cases := []struct {
		name  string
		png   []byte
		blank bool
	}{
		{"white", solid(t, color.White), true},
		{"black", solid(t, color.Black), true},
		{"gradient", busy(t), false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			measured, err := Blank(test.png)
			if err != nil {
				t.Fatalf("Blank: %v", err)
			}
			if measured.Blank != test.blank {
				t.Errorf("blank = %t, want %t (%s)", measured.Blank, test.blank, measured.Describe())
			}
		})
	}
}

// A screen that is mostly one colour with a small amount of content is the
// hardest real case: a loading spinner on white. It must read as blank, because
// that is what the user is looking at.
func TestAMostlyEmptyScreenReadsAsBlank(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.White)
		}
	}
	// A 20x20 dark square: 4% of the frame, well under the 80% threshold.
	for y := 40; y < 60; y++ {
		for x := 40; x < 60; x++ {
			img.Set(x, y, color.RGBA{A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}

	measured, err := Blank(buf.Bytes())
	if err != nil {
		t.Fatalf("Blank: %v", err)
	}
	if !measured.Blank {
		t.Errorf("a spinner on white did not read as blank (%s)", measured.Describe())
	}
}

// The other half of the same judgement, and the one that matters more in
// practice. A real screen is often mostly one background colour, and a verifier
// that failed those would be switched off within a day.
func TestAScreenWithRealContentIsNotBlank(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.White)
		}
	}
	// A card, a heading, a divider, and anti-aliased edges: the shapes a list
	// screen is made of. Still 85% white background.
	for y := 10; y < 40; y++ {
		for x := 10; x < 190; x++ {
			img.Set(x, y, color.RGBA{R: 32, G: 44, B: 60, A: 255})
		}
	}
	for y := 50; y < 150; y++ {
		for x := 10; x < 190; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 242, B: 245, A: 255})
		}
	}
	for i := 0; i < 180; i++ {
		img.Set(10+i, 160, color.RGBA{R: uint8(60 + i/3), G: uint8(70 + i/3), B: uint8(80 + i/3), A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}

	measured, err := Blank(buf.Bytes())
	if err != nil {
		t.Fatalf("Blank: %v", err)
	}
	if measured.Blank {
		t.Errorf("a screen with real content read as blank (%s); this would fail working apps",
			measured.Describe())
	}
}

func TestBlankRejectsSomethingThatIsNotAnImage(t *testing.T) {
	if _, err := Blank([]byte("this is not a png")); err == nil {
		t.Error("Blank accepted bytes that are not an image")
	}
}

func TestNewDeviceRejectsAnUnknownPlatform(t *testing.T) {
	if _, err := NewDevice(ScreenSpec{Platform: "windows-phone"}, t.TempDir()); err == nil {
		t.Error("an unknown platform was accepted")
	}
	if _, err := NewDevice(ScreenSpec{}, t.TempDir()); err == nil {
		t.Error("a missing platform was accepted")
	}
}

func TestScreenNeedsAScreenBlock(t *testing.T) {
	if _, err := (Screen{}).Verify(context.Background(), Request{Check: "e2e"}); err == nil {
		t.Error("Verify ran without a screen block")
	}
}
