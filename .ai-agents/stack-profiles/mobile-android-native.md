# Stack profile: Mobile Android native

## Scope

<routing>

Applies to consumer repositories building native Android applications with Kotlin/Java, Jetpack, Compose or XML views, coroutines/Flow, app architecture, performance, accessibility, and release tooling.

## When to load

- Android screen, ViewModel, repository, navigation, permission, service, or WorkManager work
- Jetpack Compose, XML view, coroutine, Flow, Room, Retrofit/Ktor, Hilt, or Gradle changes
- Android performance, startup, baseline profile, accessibility, offline/cache, or lifecycle decisions
</routing>

## Detection

<context>

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
</context>

## Commands

<procedure>

- `./gradlew test`
- `./gradlew connectedAndroidTest`
- `./gradlew lint`
- `./gradlew assembleDebug`
</procedure>

## Render verification (MUST)

<required>

`./gradlew connectedAndroidTest` exiting 0 means its assertions passed, not that a user would see anything. Where a task claims a screen works, verify it from the device — see [`mobile-ui-verification.md`](../references/mobile-ui-verification.md).

```sh
adb logcat -b crash -c                                   # before launch
adb shell uiautomator dump /sdcard/window_dump.xml       # after it settles
adb shell cat /sdcard/window_dump.xml                    # assert expected content
adb exec-out screencap -p > frame.png                    # assert not blank
adb logcat -b crash -d                                   # assert no crash or ANR
```

- Clear the crash buffer before launching, or a stale trace fails every run. Match `FATAL EXCEPTION` and `ANR in` rather than testing whether the buffer is empty: it opens with a banner even when nothing crashed.
- Give asserted elements stable `resource-id` values. In Compose, `Modifier.testTag` needs `testTagsAsResourceId` enabled before it appears in the dump.
- Wait for the app to settle before sampling. A frame from the splash screen proves nothing either way, and an emulator that has not finished booting produces exactly the blank frame under investigation.
- An unreadable signal is a failure, not a skip: a dropped `adb` connection otherwise looks like a healthy app.
- **The view tree outlives the display.** With the screen asleep, `uiautomator dump` still lists every element while `screencap` returns one flat colour. A hierarchy-only check passes that screen; keep the frame check.
- `adb` is usually not on `PATH` after an Android Studio install; resolve it from `ANDROID_HOME`/`ANDROID_SDK_ROOT` + `platform-tools`. In Git Bash, `/sdcard/...` is rewritten to a Windows path — use `MSYS_NO_PATHCONV=1` or `//sdcard/...`.

## Boundaries

- Do not perform network/database work on the main thread
- Do not leak Activity/Context references into long-lived objects
- Do not add Compose to XML-only modules, or XML view patterns to Compose modules, without an explicit migration plan
- Do not ignore lifecycle, configuration changes, process death, or background execution limits

## Security / performance appendix

- Use lifecycle-aware collection, stable Compose models, lazy lists, and baseline profiles for startup/runtime hotspots
- Validate permissions, exported components, deep links, WebViews, secure storage, and network security config
- Benchmark before broad Compose stability or recomposition rewrites
</required>

## References

<references>

- https://developer.android.com/topic/architecture
- https://developer.android.com/compose
- https://developer.android.com/develop/ui/compose/performance
- https://developer.android.com/kotlin/coroutines
- https://developer.android.com/topic/performance
</references>
