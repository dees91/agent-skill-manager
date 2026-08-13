# Desktop GUI

## Scope And Stack

- `implemented`: Phase 6 adds a local macOS interface without changing CLI/TUI
  source-lifecycle semantics.
- `desktop/` is a nested Go 1.25 module using Wails 2.13. The root CLI module
  retains its existing Go 1.22 directive.
- The frontend uses React 19, TypeScript, Vite, Lucide outline icons, generated
  Wails bindings, and plain CSS tokens. There is no router, external state
  store, Tailwind, or chart library.
- The dark-only app contains Dashboard, Skills, a managed-only Sources screen,
  and experimental Discover. Iteration 8 exposes Git/local lifecycle actions;
  Iteration 9 adds skills.sh browse/search and selected-skill install through
  the same Go domain services as the CLI.
- `implemented`: Iteration 7 adds a read-only global skill-catalog context
  panel to Dashboard without expanding the source-management surface.
- `implemented`: Iteration 10 replaces the flat Skills table with an
  active-first workspace and source-grouped inactive accordions without
  changing toggle or Apply semantics.

## Security And State Boundary

The Wails-bound `App` accepts skill/tool/group identifiers, opaque source,
draft, and review IDs, and read-only visibility. Git inspection additionally
accepts a Git URL; a local source path enters through Wails' native directory
picker. It delegates to `internal/gui.Service`; the frontend never supplies
active/disabled paths or planned filesystem operations.

Desktop pending changes are owned by a mutex-protected Go session and use the
shared `internal/staging` rules. Frontend projections show effective state,
but only Apply enters `ops.PlanBatch`/`ops.Apply`. A close dialog prevents
accidental loss unless the user explicitly discards pending work.

Scoped Skills bulk calls accept only exact skill/group names and tool names.
Go validates and deduplicates the scope before calling the existing
`staging.ToggleBatch`; legacy both-tool group/visible methods remain wrappers.

Source operations have a separate exclusive lane. They refuse to start while
toggle changes are pending, block other mutations and app close while active,
and emit phase progress. Install revalidates exact matrix selections before
apply. Update and uninstall resolve opaque IDs against the current manifest;
uninstall additionally requires an exact group-name confirmation.

Discover reads go through `internal/skillssh` and a versioned normalized cache.
The Wails adapter accepts catalog session/skill IDs and selected tool names;
it never accepts catalog-derived paths or repository URLs from React. Install
requires fresh detail revalidation, resolves a fixed GitHub source in Go, and
then enters the same exclusive source-operation lane and exact-cell planner.

## Scan And Apply Behavior

- Scan at startup, on explicit refresh, when read-only entries are requested,
  and after Apply.
- Do not watch or poll filesystem paths.
- Apply order and partial failure match the TUI: disables, enables, tool, skill;
  stop after the first failure and preserve/report the completed prefix.
- Structured failures identify preflight, apply, or rescan stage. Remaining
  pending entries stay available for review.

## Active-First Skills Workspace

- Conflict rows appear in `Needs attention`, applied ON rows in always-expanded
  `Active now`, and applied OFF rows in collapsed `Available by source`
  accordions. Read-only rows form a separate grouped region after opt-in.
- Section placement uses applied state, so pending transitions remain stable
  until Apply/rescan. Each skill appears in one section.
- Search, availability chips, and tool chips compose with advanced Group,
  Source, and Read only filters. Search temporarily opens matching groups.
- Group expansion lives in React App state for the process lifetime; navigating
  away and back preserves it, while restart clears it.
- Both Claude and Codex columns remain visible. The tool chip scopes
  classification and row/group/results smart-toggle targets.
- Result bulk includes filtered rows in collapsed groups. Group bulk ignores
  filtering and uses the complete loaded group. Eligible cell counts make both
  impacts explicit.

## Global Skill Context Budget

- [`internal/contextbudget`](../../../internal/contextbudget/) measures startup
  catalog metadata; it does not count full skill bodies loaded after a skill is
  selected.
- Codex attempts a timeout-bounded, fixed-argument `debug prompt-input`
  measurement in a neutral temporary directory and compares it with an
  expanded-budget catalog to count shortened descriptions and omitted skills.
  Model window evidence comes from local Codex diagnostics or its model cache.
- Claude estimates personal skills, legacy commands, and locally enumerable
  enabled plugin skills. It honors local listing overrides where detectable,
  defaults to a 1% listing budget, and labels the 200k unresolved-window
  fallback and opaque provider catalogs.
- Token figures are explicitly approximate at four characters per token.
  Current pressure is requested catalog characters divided by the provider
  budget, so the percentage may exceed 100 while the progress bar clamps.
- `gui.Service` caches the applied report at scan boundaries. Staging, undo,
  clear, group, and visible-row actions project managed-cell deltas in memory
  and return an `After Apply` report without rerunning diagnostics.
- Provider diagnostics inherit the injected home directory and are disabled
  from ambient `PATH` resolution in temporary-home tests. Failures degrade to
  labeled filesystem estimates and never fail the main skill scan.

## Visual Contract

[`../../../DESIGN.md`](../../../DESIGN.md) describes the implemented interface
and points to repository-owned Dashboard and Discover screenshots generated
from synthetic demo data. The visual system uses a dark palette, flat panel
hierarchy, persistent sidebar, dense tables, cyan informational accents, and
one warm primary action per region.

The implementation uses semantic controls, visible focus, reduced-motion
handling, an icon-only compact sidebar at 1024×720, scrollable wide tables,
loading/error/empty feedback, status announcements, and review/details drawers.
The context panel uses two stable tool rows, accessible progressbar semantics,
accuracy badges, model/window evidence, warm near-limit status, red overflow,
and a pending projection marker.

The Sources screen uses a dense manifest-owned source table and centered
workflow dialogs. Install includes Git/local selection, discovery, a scrollable
Claude/Codex matrix, review, and apply. Each tool column has an explicit bulk
selection toggle whose `ON`, `OFF`, `MIXED`, or `N/A` state reflects every
non-conflict discovered target, independent of the row filter. Dialogs trap and
restore focus, announce progress/errors, remain usable at the 1024×720
minimum, and use typed confirmation for destructive removal.

Discover uses the same dense table/surface language for rankings and search,
adds compact activity sparklines and agent state badges, and opens details in a
right-side drawer. Offline cache, unsupported well-known sources, external-only
audit, and the third-party-instruction risk are explicit states rather than
hidden failures.

The source table names behavior rather than health: its `Update mode` column
shows `Managed Git` with `Use Update to fetch changes.`, or `Linked folder`
with `Changes are read directly; no update needed.` The earlier standalone
`Ready` and `Live` labels were removed because they required remembered context.

## Build And Verification

```bash
make gui-dev
make gui-bindings
make gui-test
make gui-build
```

`make gui-build` produces a locally ad-hoc-signed Apple Silicon app. Developer
ID signing, notarization, universal packaging, publishing, and update delivery
are deferred. The build retains Wails' ad-hoc signature because it is required
for reliable launch on the supported macOS target. Verification uses an
isolated temporary home and synthetic frontend data; it must not inspect or
publish the developer's actual provider directories.

See [architecture-and-data-flow.md](architecture-and-data-flow.md) for the
service flow, [interfaces.md](interfaces.md) for controls, and
[testing-development-and-roadmap.md](testing-development-and-roadmap.md) for
verification scope.
