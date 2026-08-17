# Desktop GUI

## Scope And Stack

- `implemented`: Phase 6 adds a local macOS interface without changing CLI/TUI
  source-lifecycle semantics.
- `desktop/` is a nested Go 1.26.6 module using Wails 2.14. The root CLI module
  also requires Go 1.26.6 so patched standard-library and runtime support
  modules can be used
  consistently across release artifacts.
- The frontend uses React 19, TypeScript, Vite, Lucide outline icons, generated
  Wails bindings, and plain CSS tokens. There is no router, external state
  store, Tailwind, or chart library. Frontend development requires Node.js
  22.12 or newer and npm 10 or newer.
- The dark-only desktop source build contains Dashboard, Skills, saved Skill
  Sets, and a managed-only Sources screen. Iteration 9's experimental skills.sh
  Go adapter and domain tests remain in the repository, but Discover has no
  public Wails binding or React navigation in the `v0.5.0` preview.
- `implemented`: Iteration 7 adds a read-only global skill-catalog context
  panel to Dashboard without expanding the source-management surface.
- `implemented`: Iteration 10 replaces the flat Skills table with an
  active-first workspace and source-grouped inactive accordions without
  changing toggle or Apply semantics.
- `implemented`: Iteration 14 adds tool-agnostic saved Skill Sets, contextual
  creation, scoped smart-toggle preview/staging, and unavailable-member
  retention without changing CLI/TUI behavior.
- `implemented`: Iteration 16 adds tool-agnostic managed-skill favorites,
  favorite-first ordering, and a Favorites availability filter without
  changing CLI/TUI behavior.

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

Skill Set calls accept opaque set IDs, skill basenames, and explicit tool names.
Go owns recipe validation and resolves every name against the current scan.
Preview uses a copy of Pending; confirmed use stages through the same
`staging.ToggleBatch` engine. Recipe CRUD writes only the separate private
metadata file and never applies, clears, or owns skill state.

Favorite calls accept one skill basename and desired boolean state. Go checks
managed-row eligibility before add, persists only basename metadata, and
returns the complete path-free favorite list. Favorite changes do not touch
Pending or filesystem visibility.

Source operations have a separate exclusive lane. They refuse to start while
toggle changes are pending, block other mutations and app close while active,
and emit phase progress. Install revalidates exact matrix selections before
apply. Update and uninstall resolve opaque IDs against the current manifest;
uninstall additionally requires an exact group-name confirmation.

Dormant Discover reads go through `internal/skillssh` and a versioned normalized
cache. Search terms/results are memory-only and legacy cache queries are
removed on first desktop launch. The public `App` adapter intentionally exposes
none of the catalog methods.

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
- Eligible managed rows and details expose favorite stars. Favorites remain in
  their applied-state section, sort before other rows/groups, and can be
  isolated with a fourth availability chip that temporarily opens matching
  available groups.

## Global Skill Context Budget

- [`internal/contextbudget`](../../../internal/contextbudget/) estimates startup
  catalog metadata; it does not count full skill bodies loaded after a skill is
  selected.
- Ordinary scan/refresh uses only filesystem and settings evidence. An explicit
  Dashboard action may run timeout-bounded fixed-argument Codex
  `debug prompt-input` diagnostics and Claude `plugin list --json`.
- Codex measurement uses a neutral temporary directory and compares it with an
  expanded-budget catalog to count shortened descriptions and omitted skills.
  Model window evidence comes only from its local config/model cache.
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
- Diagnostics inherit the injected home and only PATH/temp/locale/terminal
  variables. Credentials, provider overrides, tokens, and proxies are omitted.
  Their argument allowlist is exact. Failures degrade to labeled filesystem
  estimates and never fail the main skill scan.

## Saved Skill Sets Workspace

- The workspace lists each recipe once with optional `When to use` copy,
  member count, unavailable count, and applied/effective Claude and Codex
  summaries. Expanded rows show current member source and per-tool state.
- Create/edit uses one searchable tool-agnostic member list. Existing missing
  members remain editable, while arbitrary new unavailable names are rejected.
- Toggle always requires an explicit Claude, Codex, or Both selection and a
  read-only impact preview before staging. Apply remains the only skill-state
  filesystem mutation.
- **Save as set** seeds unique skill names from Pending. Skill details expose
  **Add to Skill Set…** for an existing or new recipe.
- Set deletion removes metadata only and preserves Pending. Source uninstall
  warns about affected sets but retains their now-unavailable basenames.
- Corrupt recipe metadata is a page-local warning; Dashboard, Skills, Sources,
  and their mutations remain available.

Favorite metadata uses the same isolated-warning principle. Corruption disables
only favorite controls, and source uninstall reports retained favorite names as
a separate non-blocking impact.

## Visual Contract

[`../../../DESIGN.md`](../../../DESIGN.md) describes the implemented interface
and points to repository-owned Dashboard, Skill Sets, and dormant Discover
screenshots generated from synthetic demo data. The visual system uses a dark
palette, flat panel hierarchy, persistent sidebar, dense tables, cyan
informational accents, and one warm primary action per region.

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

The Skill Sets screen uses the same compact table language, expandable member
detail, and centered dialogs. Dialog focus is contained, action scope is named
in text, unavailable members remain explicit, and the layout remains usable at
1024×720. The synthetic 1440×960 capture is
[`../../images/skill-sets.png`](../../images/skill-sets.png).

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
make release-package RELEASE_VERSION=0.5.0
```

`make gui-build` produces a local ad-hoc-signed Apple Silicon app.
`make release-package` requires a clean Apple Silicon macOS checkout, runs the
full local verification suite, verifies regenerated dependency notices, builds the versioned desktop ZIP and CLI
tarball, re-extracts them, checks their signatures/metadata/architecture and an
isolated-home launch, and writes a SHA-256 manifest under ignored
`dist/release/`. Developer ID signing, notarization, DMG and universal
packaging, SBOM/provenance, automatic publishing, and update delivery remain deferred. Release
verification must not inspect or publish the developer's actual provider
directories.

See [architecture-and-data-flow.md](architecture-and-data-flow.md) for the
service flow, [interfaces.md](interfaces.md) for controls, and
[testing-development-and-roadmap.md](testing-development-and-roadmap.md) for
verification scope.
