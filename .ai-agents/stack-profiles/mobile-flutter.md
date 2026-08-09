# Stack profile: Mobile Flutter

## Scope

<routing>

Applies to consumer repositories building cross-platform mobile applications with Flutter and Dart, including widgets, navigation, state management, platform channels, accessibility, testing, and release performance.

Compose with [`lang-dart.md`](lang-dart.md) for language-level concerns: null safety, futures and streams, isolates, pub packaging.

## When to load

- Flutter widget, screen, state, navigation, or platform integration work
- Dart package, `pubspec.yaml`, build runner, generated code, or app release changes
- Mobile performance, animation, accessibility, offline/cache, or platform-channel decisions
</routing>

## Detection

<context>

- `pubspec.yaml` includes `flutter`
- `lib/main.dart`, `android/`, `ios/`, `test/`, `integration_test/`
- Dependencies such as `go_router`, `riverpod`, `provider`, `bloc`, `freezed`, `json_serializable`, `dio`

## Framework and tooling

- Flutter SDK and Dart
- Widget tree / declarative UI with repo-pinned state management
- Optional: Provider, Riverpod, Bloc, go_router, Dio, freezed/json_serializable
- Flutter DevTools for performance, memory, layout, and network inspection

## Repo layout conventions

- Read `pubspec.yaml`, `analysis_options.yaml`, `lib/main.dart`, and generated-code conventions first
- Keep features vertical (`lib/features/<feature>/...`) when existing layout supports it
- Keep widgets focused; move async orchestration into controllers/notifiers/blocs/services
- Keep platform channels and native code behind adapter interfaces
- Keep generated files generated; edit source annotations/models instead
</context>

## Commands

<procedure>

- `flutter analyze`
- `flutter test`
- `flutter test integration_test`
- `flutter build apk`
- `flutter build ios`
</procedure>

## Render verification (MUST)

<required>

`flutter test integration_test` exits 0 whether the app painted a list or a white rectangle. Its exit code is not a render check, and neither is the existence of a screenshot file: a blank PNG is a non-empty file. See [`mobile-ui-verification.md`](../references/mobile-ui-verification.md) for the full contract.

Flutter is the hardest case for the usual device-side approach. The app renders to a **single canvas**, and that canvas is not accessible unless the app enables semantics, so `adb shell uiautomator dump` returns a hierarchy with essentially one node. The dump *succeeds*, which means "the expected text is absent" and "nothing was measured" look identical from outside.

So content assertions belong **inside the app**:

- Assert on rendered widgets and their values in `integration_test`, for example that a total reads `42.00`, not merely that the screen built.
- Where an out-of-app driver is used, prefer one that speaks to the Flutter engine rather than to the platform accessibility tree.
- Enabling semantics (`SemanticsBinding.instance.ensureSemantics()`) makes the tree visible, but semantics are opt-in for performance reasons; treat it as a deliberate app change, not a given.

Device-side signals still apply and do not depend on the canvas:

- `adb logcat -b crash -c` before launch, `-d` after, matched against `FATAL EXCEPTION` and `ANR in`.
- A screenshot checked for carrying almost no visual variation.
- A Flutter **error widget** is not a crash: the buffer stays clean and the screen is busy. Assert its text is absent.

A white screen during a test run can also be the harness rather than the app. Check the driver's issue tracker for the version in use before concluding the app is broken.

## Boundaries

- Do not introduce a new state-management package when the repo has a clear existing pattern
- Do not perform blocking or CPU-heavy work on the UI isolate; use isolates or platform/native work where needed
- Do not mix persistence, network, and widget rendering logic inside large stateful widgets
- Do not edit generated files directly

## Security / performance appendix

- Profile jank with Flutter DevTools; optimize rebuild scopes before broad rewrites
- Use const widgets, stable keys, lazy lists, image caching, and isolate offload where evidence supports it
- Validate permissions, secure storage, TLS assumptions, and platform-specific lifecycle behavior
</required>

## References

<references>

- https://docs.flutter.dev/
- https://docs.flutter.dev/data-and-backend/state-mgmt/intro
- https://docs.flutter.dev/perf
- https://docs.flutter.dev/testing
- https://docs.flutter.dev/accessibility-and-internationalization/accessibility
</references>
