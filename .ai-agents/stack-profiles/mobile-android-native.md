# Stack profile: Mobile Android native

## Scope

Applies to consumer repositories building native Android applications with Kotlin/Java, Jetpack, Compose or XML views, coroutines/Flow, app architecture, performance, accessibility, and release tooling.

## When to load

- Android screen, ViewModel, repository, navigation, permission, service, or WorkManager work
- Jetpack Compose, XML view, coroutine, Flow, Room, Retrofit/Ktor, Hilt, or Gradle changes
- Android performance, startup, baseline profile, accessibility, offline/cache, or lifecycle decisions

## Detection

- `settings.gradle*`, `build.gradle*`, `gradle/libs.versions.toml`
- `app/src/main/AndroidManifest.xml`, `src/main/java/`, `src/main/kotlin/`
- Dependencies such as `androidx.compose`, `androidx.lifecycle`, `kotlinx-coroutines`, `hilt`, `room`, `workmanager`

## Framework and tooling

- Kotlin-first Android where repo conventions allow; Java only when existing modules require it
- Jetpack Compose or XML views depending on existing UI stack
- Android Architecture Components: ViewModel, Lifecycle, Navigation, Room, WorkManager
- Kotlin coroutines and Flow for async streams
- Android Studio profiler, Macrobenchmark, Baseline Profiles where configured

## Repo layout conventions

- Read Gradle manifests, version catalogs, AndroidManifest, and module boundaries first
- Keep UI state in ViewModel-like presentation layer; keep repositories/data sources behind interfaces
- Keep composables small and stable; isolate side effects with lifecycle-aware APIs
- Keep platform permissions, foreground services, background work, and notifications explicit

## Commands

- `./gradlew test`
- `./gradlew connectedAndroidTest`
- `./gradlew lint`
- `./gradlew assembleDebug`

## Boundaries

- Do not perform network/database work on the main thread
- Do not leak Activity/Context references into long-lived objects
- Do not add Compose to XML-only modules, or XML view patterns to Compose modules, without an explicit migration plan
- Do not ignore lifecycle, configuration changes, process death, or background execution limits

## Security / performance appendix

- Use lifecycle-aware collection, stable Compose models, lazy lists, and baseline profiles for startup/runtime hotspots
- Validate permissions, exported components, deep links, WebViews, secure storage, and network security config
- Benchmark before broad Compose stability or recomposition rewrites

## References

- https://developer.android.com/topic/architecture
- https://developer.android.com/compose
- https://developer.android.com/develop/ui/compose/performance
- https://developer.android.com/kotlin/coroutines
- https://developer.android.com/topic/performance
