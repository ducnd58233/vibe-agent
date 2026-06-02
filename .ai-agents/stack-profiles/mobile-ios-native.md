# Stack profile: Mobile iOS native

## Scope

Applies to consumer repositories building native iOS applications with Swift, SwiftUI or UIKit, Swift concurrency, Observation/Combine, accessibility, performance, and App Store release workflows.

## When to load

- iOS screen, view model, navigation, persistence, networking, concurrency, or platform integration work
- SwiftUI, UIKit, async/await, TaskGroup, actors, Observation, Combine, Core Data/SwiftData, or Xcode project changes
- iOS performance, accessibility, lifecycle, permission, deep-link, push, or release decisions

## Detection

- `.xcodeproj`, `.xcworkspace`, `Package.swift`, `Podfile`, `Cartfile`
- `Sources/`, `App/`, `*.swift`, `Info.plist`, `Assets.xcassets`
- Dependencies or imports such as `SwiftUI`, `UIKit`, `Combine`, `Observation`, `SwiftData`, `CoreData`

## Framework and tooling

- Swift and Xcode
- SwiftUI or UIKit depending on existing UI stack
- Swift concurrency (`async`/`await`, `Task`, actors, task groups) for async work
- Observation / Combine depending on deployment target and existing patterns
- XCTest, XCUITest, Instruments, MetricKit where configured

## Repo layout conventions

- Read project/workspace/package files, `Info.plist`, app entrypoint, and module boundaries first
- Keep view rendering separate from networking, persistence, and business orchestration
- Use `@MainActor` deliberately for UI state; keep heavy work off the main actor
- Keep platform capabilities, entitlements, permissions, background modes, and deep links explicit

## Commands

- `xcodebuild test -scheme <Scheme> -destination <Destination>`
- `xcodebuild build -scheme <Scheme> -destination <Destination>`
- `swift test`
- `swift build`

## Boundaries

- Do not block the main actor/thread with network, file, decoding, image processing, or CPU-heavy work
- Do not mix SwiftUI and UIKit migration patterns without checking the existing architecture
- Do not create detached tasks without cancellation, ownership, and lifecycle reasoning
- Do not bypass privacy prompts, entitlement requirements, or App Store review constraints

## Security / performance appendix

- Profile startup, scrolling, memory, and main-thread stalls with Instruments
- Validate accessibility labels, dynamic type, VoiceOver order, reduced motion, and color contrast
- Protect secrets with Keychain/secure storage; validate deep links, pasteboard, WebViews, and ATS assumptions

## References

- https://developer.apple.com/documentation/swiftui
- https://developer.apple.com/documentation/swift/concurrency
- https://developer.apple.com/documentation/Observation
- https://developer.apple.com/accessibility/
- https://developer.apple.com/documentation/xcode/understanding-and-improving-swiftui-performance
