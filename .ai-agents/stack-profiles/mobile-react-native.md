# Stack profile: Mobile React Native

## Scope

<routing>

Applies to consumer repositories building iOS and Android applications with React Native or Expo, including navigation, native modules, mobile performance, offline state, accessibility, and release validation.

Compose with [`lang-typescript.md`](lang-typescript.md) for type-system concerns.

## When to load

- React Native screen, component, navigation, or native bridge work
- Expo, Metro, Hermes, New Architecture, native module, or platform-specific changes
- Mobile performance, gesture, animation, list virtualization, offline/cache, or app-store release concerns
</routing>

## Detection

<context>

- `package.json` includes `react-native` or `expo`
- `metro.config.*`, `app.json`, `app.config.*`, `ios/`, `android/`
- Dependencies such as `@react-navigation/*`, `react-native-reanimated`, `react-native-gesture-handler`, `expo-*`

## Framework and tooling

- React Native with TypeScript where configured
- Expo or bare React Native CLI depending on manifest
- Hermes, Metro, React Native DevTools, Flipper-compatible tooling where present
- Optional: React Navigation, Reanimated, Gesture Handler, MMKV/AsyncStorage, TanStack Query, Zustand/Redux

## Repo layout conventions

- Read `package.json`, `app.json/app.config.*`, `metro.config.*`, `babel.config.*`, and native project files before changing mobile setup
- Keep screens feature-scoped; keep reusable native-safe components in shared UI packages
- Keep platform-specific code explicit with `.ios.*`, `.android.*`, or native module boundaries
- Keep JS-thread hot paths small; use native/animated worklets where appropriate for gestures/animations
</context>

## Commands

<procedure>

- `npm run lint`
- `npm run typecheck`
- `npm run test`
- `npm run android`
- `npm run ios`
</procedure>

## Render verification (MUST)

<required>

`npm run test` and `npm run android` say nothing about what reached the display, and a saved screenshot proves only that a file exists. Verify the render from the device's own reported state — see [`mobile-ui-verification.md`](../references/mobile-ui-verification.md).

React Native renders **native views**, so the platform view hierarchy works here and is the strongest signal available:

```sh
adb logcat -b crash -c                                   # before launch
adb shell uiautomator dump /sdcard/window_dump.xml       # after it settles
adb shell cat /sdcard/window_dump.xml                    # assert expected content
adb exec-out screencap -p > frame.png                    # assert not blank
adb logcat -b crash -d                                   # assert no crash or ANR
```

- Set `testID` on the elements a check asserts on. Every driver locates elements by identifier, label, or XPath, so a rename breaks the check whichever driver is in use; stable identifiers are what reduce that churn.
- At least one assertion must name **expected content**, not just "not blank". Crash-free and non-blank together still pass a screen showing the wrong numbers.
- A **redbox is not a crash**: an unhandled JS exception leaves the crash buffer clean and the screen busy. Assert that the redbox's own text is absent.
- On iOS there is no `uiautomator` equivalent; assert content from an XCUITest or in-app test and use `xcrun simctl` for the screenshot and fault log.

## Boundaries

- Do not assume web DOM APIs are available
- Do not introduce native dependency changes without checking Expo/bare compatibility and config plugins
- Do not block the JS thread with synchronous heavy computation, large JSON parsing, or unvirtualized lists
- Do not ignore platform differences in permissions, file access, background execution, push, and deep links

## Security / performance appendix

- Test slow-device list scrolling, app startup, image loading, and navigation transitions
- Prefer FlatList/FlashList-style virtualization for long lists
- Validate permissions, secure storage, network TLS assumptions, and deep-link handling
- Check dependency compatibility with the active React Native / Expo SDK and New Architecture posture
</required>

## References

<references>

- https://reactnative.dev/docs/getting-started
- https://reactnative.dev/docs/performance
- https://reactnative.dev/docs/accessibility
- https://docs.expo.dev/
- https://docs.expo.dev/guides/new-architecture/
</references>
