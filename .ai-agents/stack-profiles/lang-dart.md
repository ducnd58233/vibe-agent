# Stack profile: Language Dart

## Scope

Applies to consumer repositories writing Dart at the language level: sound null safety, the async and isolate model, type system, and pub packaging. Independent of any framework; compose with [`mobile-flutter.md`](mobile-flutter.md) for widget, navigation, and platform-channel work.

## When to load

- Writing or reviewing Dart in packages, CLIs, servers, or shared business logic
- Null-safety modeling, generics, or type-system problems
- `Future`, `Stream`, isolate, or concurrency design
- `pubspec.yaml`, dependency constraint, or package-publishing changes
- Code generation and build_runner pipelines

## Detection

- `pubspec.yaml`, `pubspec.lock`, `analysis_options.yaml`
- `lib/`, `bin/`, `test/` directories; `*.dart` sources
- `.dart_tool/`, generated `*.g.dart` or `*.freezed.dart` files
- SDK constraints in `pubspec.yaml` distinguishing a Dart-only package from a Flutter package

## Framework and tooling

Non-exhaustive examples. Any tool here may be renamed, deprecated, or replaced. Detect what the repo actually uses and verify current commands and flags against official docs ([`source-driven-development`](../skills/source-driven-development/SKILL.md)) before running anything.

- SDK and package manager: the Dart SDK and `dart pub`; a Flutter package uses `flutter pub` instead
- Analysis and format: `dart analyze`, `dart format`, lint rule sets configured in `analysis_options.yaml`
- Test: `package:test`; for example mocktail or mockito for test doubles
- Code generation: for example build_runner with json_serializable, freezed
- Compilation targets: for example native AOT, JIT, JavaScript, WASM

## Repo layout conventions

- Read `pubspec.yaml` first: the SDK constraint, dependency bounds, and whether Flutter is a dependency at all
- Public API lives under `lib/`; anything under `lib/src/` is internal and should be re-exported deliberately
- Executables belong in `bin/`; tests mirror the source tree under `test/`
- Generated files are build output — regenerate them, do not hand-edit

## Commands

Use repo-documented commands first. Typical examples:

- `dart pub get`
- `dart analyze`
- `dart format --output=none --set-exit-if-changed .`
- `dart test`
- Regenerate code with the repo's build_runner invocation when generated sources are stale

## Boundaries

- Null safety is sound only if you do not defeat it. The `!` operator asserts non-null and throws when wrong; prefer narrowing, `?.`, `??`, or restructuring the type
- Prefer `late` only when initialization genuinely happens before first read; a `late` field read too early throws at runtime rather than failing to compile
- Dart is single-threaded per isolate. CPU-bound work blocks that isolate's event loop — move it to a separate isolate
- Isolates do not share mutable memory; data crosses by message. Design for that boundary instead of trying to share state
- Distinguish a single-value `Future` from a multi-value `Stream`, and always cancel stream subscriptions you own to avoid leaks
- Unawaited futures swallow errors silently. Await them, or mark the intent explicitly
- Keep a package Flutter-free unless it genuinely needs Flutter; depending on Flutter makes the package unusable on the server and in plain Dart CLIs
- Do not widen dependency constraints to resolve a conflict without checking what the widened range actually allows

## Security / performance appendix

- Validate data decoded from JSON or platform channels; a decoded `Map` is `dynamic` and defeats static checking
- Bound network calls with timeouts and handle cancellation
- Prefer `const` constructors and immutable value types where the API allows it
- Profile before optimizing; async scheduling and serialization usually dominate over raw computation

## References

- https://dart.dev/language
- https://dart.dev/null-safety
- https://dart.dev/language/concurrency
- https://dart.dev/tools/pub/pubspec
