# Accessibility Checklist

Quick reference for WCAG 2.1 AA compliance. Use alongside the [`frontend-ui-engineering`](../skills/frontend-ui-engineering/SKILL.md) skill.

**Repository-specific UI stack** for this codebase: see frontend-related rows in [`stack-profiles/ROUTER.md`](../stack-profiles/ROUTER.md).

## Table of Contents

- [Essential Checks](#essential-checks)
- [Common HTML Patterns](#common-html-patterns)
- [Testing Tools](#testing-tools)
- [Quick Reference: ARIA Live Regions](#quick-reference-aria-live-regions)
- [Common Anti-Patterns](#common-anti-patterns)

## Essential Checks

### Keyboard Navigation

- [ ] All interactive elements focusable via Tab key
- [ ] Focus order follows visual/logical order
- [ ] Focus is visible (outline/ring on focused elements)
- [ ] Custom widgets have keyboard support (Enter to activate, Escape to close)
- [ ] No keyboard traps (user can always Tab away from a component)
- [ ] Skip-to-content link at top of page — visible (at least) on keyboard focus
- [ ] Modals trap focus while open, return focus on close

### Screen Readers

- [ ] All images have `alt` text (or `alt=""` for decorative images)
- [ ] All form inputs have associated labels (`<label htmlFor>` or `aria-label`)
- [ ] Buttons and links have descriptive text (not "Click here")
- [ ] Icon-only buttons have `aria-label`
- [ ] Page has one `<h1>` and headings do not skip levels
- [ ] Dynamic content changes announced (`aria-live` regions)
- [ ] Tables have `<th>` headers with scope

### Visual

- [ ] Text contrast ≥ 4.5:1 (normal text) or ≥ 3:1 (large text, 18px+)
- [ ] UI components contrast ≥ 3:1 against background
- [ ] Color is not the only way to convey information
- [ ] Text resizable to 200% without breaking layout
- [ ] No content that flashes more than 3 times per second

### Forms

- [ ] Every input has a visible label
- [ ] Required fields indicated (not by color alone)
- [ ] Error messages specific and associated with the field
- [ ] Error state visible by more than color (icon, text, border)
- [ ] Form submission errors summarized and focusable
- [ ] Known fields use autocomplete (for example `type="email" autoComplete="email"`)

### Content

- [ ] Language declared (`<html lang="...">`)
- [ ] Page has a descriptive `<title>`
- [ ] Links distinguish from surrounding text (not by color alone)
- [ ] Touch targets ≥ 44×44px on mobile
- [ ] Meaningful empty states (not blank screens)

## Common HTML Patterns

### Buttons vs. Links

```tsx
// Use <button> for actions
<button type="button" onClick={handleDelete}>Delete Task</button>

// Use <a href> or your framework’s navigation component for in-app routes
<a href="/tasks/123">View Task</a>

// NEVER use div/span as buttons
<div onClick={handleDelete}>Delete</div>  // BAD
```

### Form Labels

```tsx
// Explicit label association
<label htmlFor="email">Email address</label>
<input id="email" type="email" required />

// Hidden label (visible label preferred)
<input type="search" aria-label="Search tasks" />
```

### ARIA Roles

```html
<nav aria-label="Main navigation">...</nav>
<div role="status" aria-live="polite">Task saved</div>
<div role="alert">Error: Title is required</div>
<dialog aria-modal="true" aria-labelledby="dialog-title">
  <h2 id="dialog-title">Confirm Delete</h2>
</dialog>
<div aria-busy="true" aria-label="Loading tasks">
  <Spinner />
</div>
```

## Testing Tools

```bash
npx @axe-core/cli
npx pa11y https://localhost:3000
```

In Chrome DevTools: Lighthouse → Accessibility; Elements → Accessibility tree.

Screen readers: macOS VoiceOver (Cmd+F5); Windows NVDA or JAWS.

## Quick Reference: ARIA Live Regions

| Value | Behavior | Use For |
|-------|----------|---------|
| `aria-live="polite"` | Announced at next pause | Status updates, saved confirmations |
| `aria-live="assertive"` | Announced immediately | Errors, time-sensitive alerts |
| `role="status"` | Same as `polite` | Status messages |
| `role="alert"` | Same as `assertive` | Error messages |

## Common Anti-Patterns

| Anti-Pattern | Problem | Fix |
|--------------|---------|-----|
| `div` as button | Not focusable, no keyboard support | Use `<button>` |
| Missing `alt` text | Images invisible to screen readers | Add descriptive `alt` |
| Color-only states | Invisible to color-blind users | Add icons, text, or patterns |
| Custom dropdown with no ARIA | Unusable by keyboard/screen reader | Use a robust Select primitive or proper listbox |
| Removing focus outlines | Users cannot see where they are | Style outlines, do not remove |
| Empty links/buttons | "Link" announced with no description | Add text or `aria-label` |
| `tabindex > 0` | Breaks natural tab order | Use `tabindex="0"` or `-1` only |

---

Repository-specific component library notes: document in the appropriate profile and link it from [`ROUTER.md`](../stack-profiles/ROUTER.md).
