# Stack profile: Mobile iOS native

## Scope

<routing>

Applies to consumer repositories building native iOS applications with Swift, SwiftUI or UIKit, Swift concurrency, Observation/Combine, accessibility, performance, and App Store release workflows.

## When to load

- iOS screen, view model, navigation, persistence, networking, concurrency, or platform integration work
- SwiftUI, UIKit, async/await, TaskGroup, actors, Observation, Combine, Core Data/SwiftData, or Xcode project changes
- iOS performance, accessibility, lifecycle, permission, deep-link, push, or release decisions
</routing>

## Detection

<context>

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
</context>

## Commands

<procedure>

- `xcodebuild test -scheme <Scheme> -destination <Destination>`
- `xcodebuild build -scheme <Scheme> -destination <Destination>`
- `swift test`
- `swift build`
</procedure>

## Render verification (MUST)

<required>

`xcodebuild test` exiting 0 means its assertions passed, not that a screen rendered. Where a task claims a screen works, verify it - see [`mobile-ui-verification.md`](../references/mobile-ui-verification.md).

`simctl` covers two of the three signals; the third has to come from inside the app:

```sh
xcrun simctl io booted screenshot frame.png              # assert not blank
xcrun simctl spawn booted log show --last 2m   --predicate "eventType == 'faultEvent'"                # assert no fault
```

- There is **no `uiautomator` equivalent on iOS**. Assert expected content with XCUITest queries on accessibility identifiers, which is also what makes the assertions survive a view rename.
- At least one assertion must name **expected content**. A crash-free, non-blank screen can still show the wrong values.
- The simulator's crash reports are cumulative with no clearable buffer, so bound the query by time or a stale report will outlive the run that produced it.
- An unreadable signal is a failure, not a skip.

## Boundaries

- Do not block the main actor/thread with network, file, decoding, image processing, or CPU-heavy work
- Do not mix SwiftUI and UIKit migration patterns without checking the existing architecture
- Do not create detached tasks without cancellation, ownership, and lifecycle reasoning
- Do not bypass privacy prompts, entitlement requirements, or App Store review constraints

## Security / performance appendix

- Profile startup, scrolling, memory, and main-thread stalls with Instruments
- Validate accessibility labels, dynamic type, VoiceOver order, reduced motion, and color contrast
- Protect secrets with Keychain/secure storage; validate deep links, pasteboard, WebViews, and ATS assumptions
</required>

## References

<references>

- https://developer.apple.com/documentation/swiftui
- https://developer.apple.com/documentation/swift/concurrency
- https://developer.apple.com/documentation/Observation
- https://developer.apple.com/accessibility/
- https://developer.apple.com/documentation/xcode/understanding-and-improving-swiftui-performance
</references>
