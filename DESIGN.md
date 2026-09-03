---
schema: design-md/v1
name: "Skill Manager macOS GUI"
sources:
  - type: screenshot
    path: "docs/images/dashboard.png"
    platform: desktop
    dimensions: "1440x960"
  - type: screenshot
    path: "docs/images/discover.png"
    platform: desktop
    dimensions: "1440x960"
  - type: screenshot
    path: "docs/images/skill-sets.png"
    platform: desktop
    dimensions: "1440x960"
confidence:
  overall: high
  colors: high
  typography: high
  spacing: high
  components: high
  responsive: medium
tokens:
  colors:
    canvas: "#161617"
    chrome: "#212123"
    surface: "#212123"
    surface-deep: "#0D0D0E"
    surface-subtle: "#262627"
    text-primary: "#F2F2F2"
    accent-info: "#50B0E0"
    accent-secondary: "#6090E0"
    accent-action: "#E08050"
  typography:
    body:
      family: "-apple-system, BlinkMacSystemFont, system-ui, sans-serif"
      size: "14px"
      weight: 400
      lineHeight: "1.45"
    numeric:
      family: "ui-monospace, SFMono-Regular, Menlo, monospace"
      size: "13px"
      weight: 400
      lineHeight: "1.4"
  spacing:
    compact: "8px"
    control: "12px"
    panel: "20px"
    region: "28px"
  radii:
    control: "8px"
    panel: "12px"
  shadows:
    panel: "none; use a low-contrast border"
---

## Design Intent

Skill Manager is a dense, operational desktop workspace. It uses a quiet dark
foundation, compact controls, stable columns, and restrained accents so users
can compare skill state without losing context. Tables and grouped lists are the
primary work surfaces; decoration is secondary to state, ownership, and safety.

This file describes the implemented interface. The repository-owned screenshots
under `docs/images/` are generated from synthetic demo data and are safe to use
in public documentation.

## Foundations

- `#161617` is the canvas, `#212123` is the primary panel and navigation
  surface, and `#0D0D0E` provides deeper selected or summary regions.
- Thin low-contrast borders separate panels and rows. Surface changes provide
  hierarchy without pronounced shadows.
- Near-white text identifies headings, important values, and active items.
  Muted gray carries descriptions, paths, timestamps, and secondary metadata.
- Cyan is informational, blue supports charts, and warm orange is reserved for
  primary actions and attention states. Conflict/destructive states use the
  dedicated error treatment.
- UI text uses the macOS system sans-serif stack. Paths, versions, commits, and
  numerical diagnostics use a system monospace stack.

## Components

- **Navigation rail:** persistent product navigation with compact icon/label
  rows, a deep selected state, connection status, and last-scan metadata.
- **Native About:** the standard macOS application menu exposes the app icon,
  product name, current build version, and short description without repeating
  release metadata inside the workspace.
- **Summary cards:** equal-width dashboard cards with a muted label and one
  prominent value.
- **Panels and tables:** full-width dark surfaces with compact headers, aligned
  columns, dividers, keyboard focus, and explicit empty/error/loading states.
- **Buttons and badges:** orange filled primary actions, bordered secondary
  actions, cyan links, and compact state badges that never rely on color alone.
- **Context budget:** one row per provider with an approximate token ratio,
  model metadata, a visible percentage, an optional post-Apply projection, and
  an explicit action for running local provider diagnostics.
- **Active-first Skills workspace:** conflicts first, then always-expanded
  active skills, collapsed available-by-source groups, and explicitly opted-in
  read-only groups. Managed rows expose accessible favorite stars; a Favorites
  chip narrows the same hierarchy and favorite rows/sources sort first. Pending
  changes stay in their applied section until Apply.
- **Saved Skill Sets:** task-oriented recipes show member count, applied and
  post-Apply state per tool, unavailable members, and expandable member detail.
  Create/edit supports one tool-agnostic member selection plus an optional
  `When to use` note; every toggle opens an explicit Claude/Codex/Muse/All preview.
- **Managed Sources:** Git repositories and linked folders with unambiguous
  update modes, counts, locations, and separately confirmed lifecycle actions.
- **Install workflow:** inspect, matrix selection, review, and apply. Tool-column
  bulk selectors cover all discovered candidates, including filtered-out rows.
- **Discover (dormant):** its experimental design and implementation history are
  retained, but the `v0.4.1` public preview has no navigation or bound catalog
  surface.
- **Dialogs and progress:** centered panels over a dimmed canvas. Mutating source
  operations announce their phase and prevent overlapping changes.

## Layout and Responsive Behavior

The desktop layout combines a fixed navigation rail with a vertically scrolling
content region. Dashboard summaries use a multi-column grid, while operational
tables span the available width. The Skills screen keeps Claude, Codex, and
Muse columns stable while filters change scope.

At narrower supported widths, summary grids wrap before tables lose essential
columns. Tables and matrices may scroll horizontally inside their own panels.
The navigation rail and primary actions remain reachable without covering the
current work surface.

## Interaction and Motion

Skill toggles are staged locally. Apply is the only action that persists those
pending changes. Source lifecycle actions are immediate but separately reviewed
and confirmed. Focus, hover, disabled, loading, conflict, and offline states must
all have textual or structural cues.

Skill Set metadata changes are immediate local edits, while using a set only
stages ordinary skill toggles. Sets may overlap; they do not own or reference
count active skills. Unavailable member names remain visible and reconnect by
basename when the skill returns.

Favorite metadata changes are immediate and independent from staged toggles.
The favorite filter temporarily expands matching available groups without
changing the user's session expansion choices. Missing basenames remain saved
and reconnect when a managed skill returns.

Motion is short and functional. Reduced-motion preferences remove nonessential
transitions. Long-running source operations show phase progress and do not imply
that closing or cancellation is safe.

## Accessibility

- Use semantic controls and landmarks, complete keyboard navigation, visible
  focus rings, and accessible names for icon-only actions.
- Announce pending counts, apply results, progress phases, errors, and offline
  catalog state to assistive technology.
- Keep text and meaningful controls at WCAG AA contrast. Do not communicate ON,
  OFF, conflict, read-only, or update mode through color alone.
- Preserve logical focus after filters, accordion changes, dialogs, and rescans.
- Keep compact controls large enough for reliable pointer and keyboard use.

## Reproduction Guidelines

1. Start with the dark canvas, lighter navigation/panel surfaces, and border-led
   separation defined in the token block.
2. Preserve information hierarchy: summary, operational action, then grouped
   detail. Keep one warm primary action per region.
3. Use synthetic data for demos, screenshots, and fixtures. Never capture a real
   home path, username, email address, installed-skill inventory, or provider
   state in repository assets.
4. Validate desktop changes with type checking, component tests, a production
   frontend build, backend tests, and screenshots at the documented viewport.
