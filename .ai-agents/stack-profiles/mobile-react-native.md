# Stack profile: Mobile React Native

## Scope

Applies to consumer repositories building iOS and Android applications with React Native or Expo, including navigation, native modules, mobile performance, offline state, accessibility, and release validation.

## When to load

- React Native screen, component, navigation, or native bridge work
- Expo, Metro, Hermes, New Architecture, native module, or platform-specific changes
- Mobile performance, gesture, animation, list virtualization, offline/cache, or app-store release concerns

## Detection

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

## Commands

- `npm run lint`
- `npm run typecheck`
- `npm run test`
- `npm run android`
- `npm run ios`

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

## References

- https://reactnative.dev/docs/getting-started
- https://reactnative.dev/docs/performance
- https://reactnative.dev/docs/accessibility
- https://docs.expo.dev/
- https://docs.expo.dev/guides/new-architecture/
