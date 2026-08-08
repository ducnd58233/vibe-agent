# Mobile UI Verification Reference

How to prove a mobile app **rendered the right thing**, not merely that a test command exited 0. Use alongside [`qa-testing-strategy.md`](qa-testing-strategy.md) and the mobile profiles in [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).

**Tool names here are non-exhaustive examples.** Any of them can be renamed, deprecated, or replaced. Detect what the repo actually uses from its manifests and config, and verify a tool's current commands and flags against its official docs before relying on them ([`source-driven-development`](../skills/source-driven-development/SKILL.md)).

## Table of Contents

- [The failure this exists to prevent](#the-failure-this-exists-to-prevent)
- [Three signals, and why three](#three-signals-and-why-three)
- [Android: collecting each signal](#android-collecting-each-signal)
- [iOS: what is and is not available](#ios-what-is-and-is-not-available)
- [Framework-specific limits](#framework-specific-limits)
- [Running a device in CI](#running-a-device-in-ci)
- [Choosing a driver](#choosing-a-driver)
- [Agent exploration is not evidence](#agent-exploration-is-not-evidence)
- [Checklist](#checklist)

## The failure this exists to prevent

An app launches on an emulator, shows a white screen or a framework error page, and the verification step reports a pass.

Nothing lied. `flutter test integration_test` or an Appium suite exits 0 when its assertions pass, and none of those assertions were about what reached the display. A command's exit code is evidence that the command succeeded. It is not evidence that a user would see anything.

Two habits produce this, and both look reasonable at the time:

| Habit | Why it passes on a blank screen |
|-------|--------------------------------|
| Treating a test command's exit code as the render check | The suite can pass without asserting on any rendered content |
| Saving a screenshot and asserting the file exists | A blank PNG is a file with content in it |

The fix is not a better command. It is **assertions about the device's own reported state**, collected by the runtime rather than chosen per run.

## Three signals, and why three

| Signal | Question | Blind spot on its own |
|--------|----------|----------------------|
| Crash and ANR records | Did the process die or hang? | A crash-free app can render nothing at all |
| Expected content in the view hierarchy | Is the right data on screen? | A hierarchy can list nodes that never painted |
| Frame is not blank | Did anything paint? | A busy screen can show entirely wrong values |

Collect all three. Each one covers a case the other two miss, and a blank screen fails all three at once — which is what makes the combination worth the setup cost.

**Only the second signal says anything about data.** Crash-free and non-blank together still pass a screen showing `Total: 0.00` where `Total: 42.00` belongs. If a check asserts no expected content, say so in the record rather than letting it read as a full pass.

**A framework error page is not a crash.** A React Native redbox or a Flutter error widget leaves the crash buffer clean and the screen busy. Name the framework's own error text as forbidden content; that is the only one of the three that catches it.

**An unreadable signal is not a passing signal.** A dropped device connection makes the crash log unavailable, and treating "could not read" as "nothing to report" turns a broken harness into a healthy app. Record it as a failure.

## Android: collecting each signal

```sh
# 1. Crash and ANR. The crash buffer is separate from the main log and holds only
#    FATAL EXCEPTION, ANR, and tombstone records, so there is no sifting a working
#    app's own logging for the word "error".
adb logcat -b crash -c              # clear first, so what follows is this run
# ... launch the app, wait for it to settle ...
adb logcat -b crash -d              # read

# 2. Content. uiautomator writes to a file on the device; there is no stdout form.
adb shell uiautomator dump /sdcard/window_dump.xml
adb shell cat /sdcard/window_dump.xml

# 3. Frame. exec-out rather than shell, or line-ending translation corrupts the PNG.
adb exec-out screencap -p > frame.png
```

Two things to get right:

- **Clear the crash buffer before launching.** Reading a buffer that still holds yesterday's stack trace fails every run, and a check that always fails gets switched off.
- **Wait for the app to settle before sampling.** A frame captured during the splash screen proves nothing either way. A fixed wait is crude but honest; a wait until an expected element appears is better where the driver supports it.

Match crash log lines rather than testing whether the buffer is empty: it opens with a `--------- beginning of crash` banner even when nothing has crashed.

For the blank-frame test, prefer measuring **how much visual variation the frame carries** over comparing pixels to the frame's average colour. The average-based form fails on the commonest real case: a spinner on white pulls the average away from white, so no pixel is near the average and the emptiest screen in the set scores as the busiest. Counting distinct quantised colours does not have that failure. Keep the threshold conservative — anti-aliased text alone puts a real screen far past "almost no colours" — because a check that fails working screens costs more than the cases it catches.

## iOS: what is and is not available

`xcrun simctl` covers two of the three signals:

```sh
xcrun simctl io booted screenshot frame.png
xcrun simctl spawn booted log show --last 2m --predicate "eventType == 'faultEvent'"
```

There is **no `uiautomator` equivalent**. Content on iOS has to be asserted from inside the app: an XCUITest, or a framework-level integration test that queries its own widget tree. Plan for the content signal to come from a different mechanism per platform rather than assuming one recipe covers both.

The simulator's crash reports are cumulative with no clearable buffer, so a stale report can outlive the run that produced it. Bound the query by time.

## Framework-specific limits

| Framework | View hierarchy via `uiautomator`? | Where content assertions belong |
|-----------|-----------------------------------|--------------------------------|
| Native Android (Views, Compose) | Yes | Either side |
| React Native | Yes — renders native views | Either side; set `testID` for stable identifiers |
| Flutter | **No** — one canvas, not accessible unless the app enables semantics | Inside the app: `integration_test`, or a driver that speaks to the Flutter engine |
| Games, WebView-heavy screens | Usually not | Inside the app or its web context |

Flutter is the case most likely to produce a false pass, because the dump *succeeds* and returns a hierarchy with essentially one node. Absence of expected content then looks identical to absence of measurement. Distinguish them: report "the dump is not a usable hierarchy" rather than "the expected text is missing", and get the content signal from an in-app assertion instead.

Enabling semantics is possible (`SemanticsBinding.instance.ensureSemantics()`), but semantics are opt-in for performance reasons, so treat it as a deliberate change to the app rather than a given.

Framework harnesses can themselves produce the blank screen being investigated. Before concluding the app is broken, check the driver's own issue tracker for the version in use — a white screen during a test run is a known failure mode of more than one mobile test harness.

## Running a device in CI

- **Android:** a maintained emulator action can boot a headless AVD, wait for it, run a script, and shut down, with a matrix over API levels. Hardware acceleration is the constraint: macOS runners generally have it, and Linux needs a runner where KVM is available, which hosted Linux runners typically are not. Verify the current situation for the runner in use rather than assuming.
- **iOS:** `simctl` on a macOS runner.
- Budget for boot time. An emulator that has not finished booting produces exactly the blank frame this reference is about, so gate sampling on boot completion, not on a sleep.

## Choosing a driver

Cross-platform mobile E2E drivers differ mainly in flakiness and maintenance cost. Published comparisons put YAML-flow drivers ahead of WebDriver-based ones on both, but most of those numbers come from vendors comparing themselves to competitors — treat them as directional, not measured.

One point the same sources agree on, and it matters more than the ranking: **every driver locates elements by test identifier, accessibility label, or XPath, so all of them break when components are renamed.** A flakiness advantage does not include rename resilience. Stable `testID` / `resource-id` values are what actually reduce churn, whichever driver is chosen.

Pick on what the repo already uses. Introducing a second driver alongside an existing one doubles the maintenance surface for no new signal.

## Agent exploration is not evidence

Accessibility-tree-driven MCP servers let an agent drive a simulator or handset directly — list elements, tap, type, read crashes — without a vision model. That is genuinely useful for **investigating** a failure: it is the mobile counterpart of a browser automation MCP server.

Keep the boundary explicit:

| Use for | Do not use for |
|---------|----------------|
| Reproducing a bug, exploring a flow, reading a crash | Producing the evidence a gate depends on |

An agent that both drives the device and decides what to record has the same shape as the failure at the top of this file: the thing being measured and the thing reporting are the same actor. The gate's evidence should come from a fixed collection path the runtime owns. See [`tool-safety-and-permissions.md`](tool-safety-and-permissions.md).

## Checklist

Before trusting a mobile render check:

- [ ] The crash buffer is cleared before launch and read after
- [ ] Crash detection matches specific markers, not "buffer is non-empty"
- [ ] The check waits for the app to settle before sampling
- [ ] At least one assertion names **expected content**, not just "not blank"
- [ ] Framework error text is listed as forbidden content
- [ ] An unreadable signal is recorded as a failure, not skipped
- [ ] For Flutter, content is asserted inside the app, and a canvas-only dump is reported as unmeasured rather than as missing content
- [ ] The blank-frame threshold has been checked against a real screen from this app, not only against a synthetic one
- [ ] The command that collects all of this is declared in the repo, not chosen per run

---

Runtime support for these signals: the `screen` verifier in [`runtime/README.md`](../../runtime/README.md), configured per check in `vibe-checks.yaml`.
