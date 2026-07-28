# Stack profile: Mobile Flutter

## Scope

Applies to consumer repositories building cross-platform mobile applications with Flutter and Dart, including widgets, navigation, state management, platform channels, accessibility, testing, and release performance.

Compose with [`lang-dart.md`](lang-dart.md) for language-level concerns: null safety, futures and streams, isolates, pub packaging.

## When to load

- Flutter widget, screen, state, navigation, or platform integration work
- Dart package, `pubspec.yaml`, build runner, generated code, or app release changes
- Mobile performance, animation, accessibility, offline/cache, or platform-channel decisions

## Detection

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

## Commands

- `flutter analyze`
- `flutter test`
- `flutter test integration_test`
- `flutter build apk`
- `flutter build ios`

## Boundaries

- Do not introduce a new state-management package when the repo has a clear existing pattern
- Do not perform blocking or CPU-heavy work on the UI isolate; use isolates or platform/native work where needed
- Do not mix persistence, network, and widget rendering logic inside large stateful widgets
- Do not edit generated files directly

## Security / performance appendix

- Profile jank with Flutter DevTools; optimize rebuild scopes before broad rewrites
- Use const widgets, stable keys, lazy lists, image caching, and isolate offload where evidence supports it
- Validate permissions, secure storage, TLS assumptions, and platform-specific lifecycle behavior

## References

- https://docs.flutter.dev/
- https://docs.flutter.dev/data-and-backend/state-mgmt/intro
- https://docs.flutter.dev/perf
- https://docs.flutter.dev/testing
- https://docs.flutter.dev/accessibility-and-internationalization/accessibility
