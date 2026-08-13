package verifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/png" // screencap and simctl both emit PNG
	"path/filepath"
	"strings"
	"time"

	state "github.com/ducnd58233/vibe-agent/runtime/internal/run"
)

// Screen proves an app actually rendered.
//
// The gap it fills: `flutter test integration_test` or an Appium suite can exit
// 0 while the app under test shows a white screen or a framework error page,
// because nothing in that exit code is about what reached the display. An
// exit-code verifier is then reporting, truthfully, that a command succeeded,
// and the run reads that as a working app.
//
// So this asks the device three separate questions, and a blank screen fails all
// three:
//
//  1. Did anything crash? The crash log buffer is separate from the main log and
//     holds FATAL EXCEPTION and ANR records.
//  2. Is the expected content in the view hierarchy? This is the only one of the
//     three that says anything about *data*: a screen can be non-blank and
//     crash-free while showing the wrong thing, or nothing but a spinner.
//  3. Is the frame non-blank? A cheap backstop for the case where the hierarchy
//     is unavailable or reports nodes that never painted.
//
// The signals are independent on purpose. Any one of them alone has a failure
// mode the other two do not share.
type Screen struct {
	// Device is injectable so the orchestration can be tested without a phone.
	// Nil means build one from the spec's platform.
	Device Device
}

func (Screen) Kind() string { return "screen" }

// ScreenSpec is what to launch and what must be true afterwards.
//
// It lives here rather than in the check plan because the verifier owns the
// meaning of these fields; the plan only carries them from YAML.
type ScreenSpec struct {
	// Platform selects the toolchain: android or ios.
	Platform string `yaml:"platform"`
	// Device names the target when more than one is attached: an adb serial or a
	// simulator UDID.
	Device string `yaml:"device"`

	// Launch installs and starts the app. It runs before anything is sampled.
	// Without it the verifier measures whatever happens to be on screen.
	Launch     string   `yaml:"launch"`
	LaunchArgs []string `yaml:"launchArgs"`

	// SettleSeconds is how long to wait after launch before sampling. A frame
	// sampled during the splash screen proves nothing either way.
	SettleSeconds int `yaml:"settleSeconds"`

	// ExpectText must all appear in the view hierarchy. This is the assertion
	// that covers data and content rather than mere pixels.
	ExpectText []string `yaml:"expectText"`
	// ExpectResourceIDs must all be present as view identifiers.
	ExpectResourceIDs []string `yaml:"expectResourceIds"`
	// ForbidText must not appear. Framework error screens are not OS-level
	// crashes, so the crash log stays empty for a React Native redbox or a
	// Flutter error widget; naming their text is what catches those.
	ForbidText []string `yaml:"forbidText"`

	// AllowBlank turns off the blank-frame check for a screen that is
	// legitimately one flat colour.
	AllowBlank bool `yaml:"allowBlank"`
}

// DefaultSettle is how long to wait after launch when the spec does not say.
const DefaultSettle = 5 * time.Second

// Settle is the wait after launch before sampling.
func (s ScreenSpec) Settle() time.Duration {
	if s.SettleSeconds <= 0 {
		return DefaultSettle
	}
	return time.Duration(s.SettleSeconds) * time.Second
}

// asserts reports whether the spec asks anything about content. A spec that
// asserts nothing about content is checked for crashes and blankness only, which
// is worth saying out loud in the summary rather than looking like a full pass.
func (s ScreenSpec) asserts() bool {
	return len(s.ExpectText) > 0 || len(s.ExpectResourceIDs) > 0 || len(s.ForbidText) > 0
}

// Device is the small surface the verifier needs from a phone or simulator.
//
// Narrow on purpose: four questions, no general "run this on the device". A
// verifier that could run arbitrary commands on the target would be a way to
// change what it is measuring.
type Device interface {
	// Name identifies the target for the record.
	Name() string
	// ClearCrashLog empties the crash buffer, so what is read afterwards belongs
	// to this launch and not to a previous one.
	ClearCrashLog(ctx context.Context) error
	// Launch installs and starts the app.
	Launch(ctx context.Context, command string, args []string) error
	// CrashLog returns the crash buffer.
	CrashLog(ctx context.Context) (string, error)
	// ViewHierarchy returns the accessibility or view tree as text. The error
	// says why when a platform or app cannot provide one.
	ViewHierarchy(ctx context.Context) (string, error)
	// Screenshot returns PNG bytes.
	Screenshot(ctx context.Context) ([]byte, error)
}

func (s Screen) Verify(ctx context.Context, req Request) (Result, error) {
	if req.Screen == nil {
		return Result{}, errors.New("screen verifier needs a screen block in the check plan")
	}
	spec := *req.Screen

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultCommandTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	device := s.Device
	if device == nil {
		built, err := NewDevice(spec, req.WorkspaceRoot)
		if err != nil {
			return Result{}, err
		}
		device = built
	}

	var log strings.Builder
	record := func(format string, args ...any) {
		fmt.Fprintf(&log, format+"\n", args...)
	}
	record("device %s", device.Name())

	// Clearing first is what makes the crash signal about this launch. Reading a
	// buffer that already held yesterday's stack trace would fail every run, and
	// a verifier that always fails gets turned off.
	if err := device.ClearCrashLog(ctx); err != nil {
		record("could not clear the crash log: %v", err)
	}

	if spec.Launch != "" {
		if err := device.Launch(ctx, spec.Launch, spec.LaunchArgs); err != nil {
			// A launch that failed is not an app that rendered wrongly; it is an app
			// that is not running. Recording it as a render failure would send the
			// reader looking at the UI.
			return s.result(req, spec, device, log.String()+
				fmt.Sprintf("launch failed: %v\n", err), []string{"the app did not launch"}), nil
		}
		record("launched %s", strings.TrimSpace(spec.Launch+" "+strings.Join(spec.LaunchArgs, " ")))
	}

	settle := spec.Settle()
	select {
	case <-ctx.Done():
		return s.result(req, spec, device, log.String()+"timed out while settling\n",
			[]string{"the run was cut short before the app settled"}), nil
	case <-time.After(settle):
	}
	record("settled for %s", settle)

	var failures []string

	crashes, err := device.CrashLog(ctx)
	if err != nil {
		// An unreadable crash log is not an absent crash. Treating it as clean
		// would make a broken adb connection look like a healthy app.
		failures = append(failures, fmt.Sprintf("could not read the crash log: %v", err))
	} else if found := CrashFindings(crashes); len(found) > 0 {
		failures = append(failures, found...)
		record("crash log:\n%s", crashes)
	} else {
		record("crash log clean")
	}

	hierarchy, err := device.ViewHierarchy(ctx)
	switch {
	case err != nil && spec.asserts():
		failures = append(failures, fmt.Sprintf("could not read the view hierarchy, so the expected content is unproven: %v", err))
	case err != nil:
		record("no view hierarchy: %v", err)
	default:
		if found := ContentFindings(hierarchy, spec); len(found) > 0 {
			failures = append(failures, found...)
		} else if spec.asserts() {
			record("every expected element is present in the view hierarchy")
		}
	}

	shot, err := device.Screenshot(ctx)
	switch {
	case err != nil:
		failures = append(failures, fmt.Sprintf("could not take a screenshot: %v", err))
	default:
		if path, writeErr := writeScreenshot(req, shot); writeErr == nil && path != "" {
			record("screenshot %s", filepath.ToSlash(path))
		}
		measured, blankErr := Blank(shot)
		switch {
		case blankErr != nil:
			failures = append(failures, fmt.Sprintf("could not read the screenshot: %v", blankErr))
		case measured.Blank && !spec.AllowBlank:
			failures = append(failures, "the screen is blank: "+measured.Describe())
		default:
			record("screen is not blank (%s)", measured.Describe())
		}
	}

	if !spec.asserts() {
		record("note: the plan asserts no expected content, so this proves the app " +
			"rendered something, not that it rendered the right thing")
	}

	return s.result(req, spec, device, log.String(), failures), nil
}

func (s Screen) result(req Request, spec ScreenSpec, device Device, detail string, failures []string) Result {
	if len(failures) > 0 {
		detail += "\nfailures:\n"
		for _, failure := range failures {
			detail += "  - " + failure + "\n"
		}
	}
	logPath, _ := writeLog(Request{
		Slug: req.Slug, LogDir: req.LogDir, Check: req.Check,
		WorkspaceRoot: req.WorkspaceRoot,
	}, []byte(detail))

	result := Result{
		Check: state.Check{
			Passed: len(failures) == 0,
			// The evidence is a set of file and process observations from the
			// device, not an exit code: no single command's status decides this.
			Source: state.SourceFileAssert,
			At:     time.Now().UTC(),
		},
		Detail:  detail,
		LogPath: logPath,
	}
	if logPath != "" {
		result.Check.Ref = filepath.ToSlash(logPath)
	}

	if len(failures) == 0 {
		result.Summary = fmt.Sprintf("%s rendered on %s: no crash, %s, not blank",
			spec.Platform, device.Name(), contentPhrase(spec))
		return result
	}
	result.Summary = fmt.Sprintf("%s did not render correctly on %s: %s",
		spec.Platform, device.Name(), strings.Join(failures, "; "))
	return result
}

func contentPhrase(spec ScreenSpec) string {
	if !spec.asserts() {
		return "no content asserted"
	}
	return fmt.Sprintf("%d expected element(s) present",
		len(spec.ExpectText)+len(spec.ExpectResourceIDs))
}

// crashMarkers are the lines that mean a process died or hung.
//
// Matched rather than "the buffer is non-empty", because the crash buffer opens
// with a "beginning of crash" banner even when nothing has crashed, and treating
// that as a failure would fail every state.
var crashMarkers = []struct {
	needle string
	says   string
}{
	{"FATAL EXCEPTION", "an unhandled exception killed a thread"},
	{"ANR in ", "the app stopped responding"},
	{"Process crashed", "the process crashed"},
	{"signal 11 (SIGSEGV)", "the process died on a segmentation fault"},
	{"signal 6 (SIGABRT)", "the process aborted"},
	{"*** *** *** *** ***", "a native crash produced a tombstone"},
}

// CrashFindings reports what a crash log says went wrong, once per kind.
//
// Exported so the parsing can be tested without a device, which is the only way
// it gets tested at all: this reads untrusted text from a phone, and the shape of
// that text is the thing most likely to be wrong.
func CrashFindings(log string) []string {
	var findings []string
	for _, marker := range crashMarkers {
		if strings.Contains(log, marker.needle) {
			findings = append(findings, marker.says+" ("+marker.needle+")")
		}
	}
	return findings
}

// ContentFindings reports which expected elements are missing from a view
// hierarchy, and which forbidden ones are present.
//
// The hierarchy is XML from uiautomator or a plist-ish dump from a simulator,
// and it is searched as text rather than parsed. Parsing would let a malformed
// dump become an error where "the content is not there" is the honest answer,
// and the attribute layouts differ per platform and per framework version.
func ContentFindings(hierarchy string, spec ScreenSpec) []string {
	var findings []string
	for _, want := range spec.ExpectText {
		if !strings.Contains(hierarchy, want) {
			findings = append(findings, fmt.Sprintf("expected text %q is not on screen", want))
		}
	}
	for _, want := range spec.ExpectResourceIDs {
		if !strings.Contains(hierarchy, want) {
			findings = append(findings, fmt.Sprintf("expected element %q is not on screen", want))
		}
	}
	for _, forbidden := range spec.ForbidText {
		if strings.Contains(hierarchy, forbidden) {
			findings = append(findings, fmt.Sprintf("forbidden text %q is on screen", forbidden))
		}
	}
	return findings
}

// Blank-frame thresholds.
//
// The first shape tried here was the one described in US patent 7536078: count
// the pixels within a tolerance of the frame's average colour. It does not
// survive the most common real case. A loading spinner on white is 96% white and
// 4% dark, which drags the average to about 245, and then the white pixels are
// themselves 10 units from the average and nothing counts as near it. The measure
// reported 0% for the emptiest screen in the set.
//
// So the question is asked differently: how much visual information does the
// frame carry at all. Colours are quantised into buckets, and a frame is blank
// when it occupies almost no buckets, or when one bucket holds nearly everything.
//
// Deliberately conservative. Anti-aliased text alone puts a real screen well past
// BlankDistinctMax, so this fires on "nothing rendered" rather than on "rendered
// badly" — and a verifier that failed working screens would be switched off,
// which costs more than the cases it would catch. Whether the right thing
// rendered is the content assertion's job, not this one's.
const (
	// BlankTolerance is the quantisation step, in 8-bit channel units. Two
	// colours within it land in the same bucket.
	BlankTolerance = 8
	// BlankDistinctMax is how many buckets a frame may occupy and still count as
	// carrying nothing. White is one; white with a spinner is two or three.
	BlankDistinctMax = 4
	// BlankDominantShare catches a frame with sensor or compression noise spread
	// across many buckets while one colour still holds essentially all of it.
	BlankDominantShare = 0.995
)

// Blankness is what a frame measured.
type Blankness struct {
	// Blank is the verdict.
	Blank bool
	// Distinct is how many quantised colour buckets the frame occupies.
	Distinct int
	// Dominant is the share of pixels in the largest bucket.
	Dominant float64
}

// Describe renders the measurement for a log or a summary.
func (b Blankness) Describe() string {
	return fmt.Sprintf("%d distinct colour(s), largest covering %.0f%%", b.Distinct, b.Dominant*100)
}

// Blank reports whether a PNG screenshot carries essentially nothing.
func Blank(encoded []byte) (Blankness, error) {
	decoded, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return Blankness{}, fmt.Errorf("decode screenshot: %w", err)
	}

	bounds := decoded.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return Blankness{}, errors.New("screenshot has no pixels")
	}

	const step = BlankTolerance + 1
	buckets := map[[3]uint8]int{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b := rgb8(decoded, x, y)
			buckets[[3]uint8{r / step, g / step, b / step}]++
		}
	}

	largest := 0
	for _, count := range buckets {
		if count > largest {
			largest = count
		}
	}

	measured := Blankness{
		Distinct: len(buckets),
		Dominant: float64(largest) / float64(total),
	}
	measured.Blank = measured.Distinct <= BlankDistinctMax ||
		measured.Dominant >= BlankDominantShare
	return measured, nil
}

// rgb8 drops the alpha and the low byte. At() returns 16-bit channels, and the
// tolerance above is in 8-bit units because that is what the source describes.
func rgb8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8((r >> 8) & 0xff), uint8((g >> 8) & 0xff), uint8((b >> 8) & 0xff)
}
