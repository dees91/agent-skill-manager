# Interfaces

## CLI Surface

- `skill-manager tui` and bare `skill-manager` launch the Bubble Tea UI.
- `version` and `--version` print the release version, tagged Go module version,
  or `dev` for an unversioned repository build.
- `list` shows discovered skill rows and tool states. `list --json` adds the
  stable path-free `apiVersion: 1` inventory. Optional `--available-for
  claude|codex|muse` keeps only toggleable OFF cells for one host; repeated
  case-insensitive `--query` terms preserve the legacy OR-substring match over
  name, description, group, and source.
- `status` summarizes `ON`, `OFF`, `CONFLICT`, and `RO` cells.
- `groups` summarizes group rows, per-tool state counts, and sources.
- `repos` summarizes repositories recorded in the Skill Manager manifest.
- `enable` and `disable` mutate one named tool/skill cell and support dry-run.
- `install <git-url|local-path>` supports strict dry-run, tool targeting, and
  repeated skill selection; Git uses a managed checkout and local paths link in
  place.
- `update [<git-url|local-path>] [--dry-run]` fast-forwards one managed Git
  repository or all recorded repositories when omitted; targeted local sources
  report that no update is required.
- `uninstall <git-url|local-path> [--dry-run]` removes one complete audited
  installation and always requires an explicit source. Local source data is
  preserved.
- `extend --tool <tool> [--dry-run]` links every managed source to one more
  tool in manifest order, mirroring ON/OFF state through the shared
  install/audit machinery. It stops at the first blocked source and never
  hardcodes a tool name.
- `advisor activate --tool claude|codex|muse --skill <name>... [--dry-run]
  [--json]` preflights 1-5 exact names and creates one opaque receipt for cells
  temporarily enabled or shared.
- `advisor search --tool claude|codex|muse --query <text> [--limit 1-50] [--json]`
  ranks that host's toggleable ON/OFF cells with deterministic local weighted
  BM25F, phrase bonuses, and bounded fuzzy matching. Its path-free result omits
  the query, reasons, and scores; the default limit is 20.
- `advisor cleanup --receipt <id> [--dry-run] [--json]` releases one exact
  receipt and restores cells whose final lease claim is removed.
- `advisor status [--tool claude|codex|muse] [--json]` lists outstanding receipts
  without exposing filesystem paths. JSON status advertises
  `ranked_search_v1` under API version 1.

The first-party skill invokes exact-receipt cleanup itself before each normal
final response. The CLI remains receipt-specific; it does not infer stale
receipts or expose bulk/expiry cleanup.

There is no CLI uninstall-all, force mode, separate `repo remove`, local-source
listing command, or TUI source-management screen. `repos` intentionally lists
Git repositories only.

## macOS Desktop Interface

- `implemented`: a Wails 2.14 `darwin/arm64` app under `desktop/`, with React,
  TypeScript, Vite, generated bindings, native dark appearance, inset macOS
  titlebar, standard app/edit menus, and pending-close confirmation.
- Dashboard shows effective per-tool counts, source groups, visibility
  distribution, context-budget pressure, and restore conflict readiness.
- Skills is active-first: `Needs attention` precedes an always-expanded `Active
  now` list, while OFF and opted-in read-only skills use collapsed source-group
  accordions. Search plus state/tool chips stay visible; Group, Source, and Read
  only live under `Filters`. All three tool columns remain visible while the tool
  chip scopes classification and bulk staging. Global bulk covers all filtered
  rows including collapsed results; group bulk covers the complete group.
  Pending rows stay in their applied-state section until Apply/rescan.
- Skill Sets lists task-oriented recipes independently from source Groups.
  Create/edit stores a name, optional `When to use` description, and
  tool-agnostic skill names. Every use requires Claude, Codex, Muse, or All, shows a
  smart-toggle preview, and stages through Pending. Member detail exposes
  unavailable names and current per-tool state. Pending offers **Save as set**;
  skill details offers **Add to Skill Set…**. Recipe deletion never changes
  skill state or clears Pending.
- Eligible Skills rows and details expose favorite stars. `Favorites N` is a
  fourth availability chip; it preserves active-first placement, composes with
  the existing filters, and temporarily expands matching available groups.
  Favorite metadata changes are immediate and do not alter Pending.
- Sources lists only manifest-owned Git/local sources. Install supports Git URL
  inspection or native local-folder selection, then an exact per-skill
  Claude/Codex/Muse matrix, review, and Apply. One toggle per tool column selects or
  clears every discovered non-conflict target regardless of the text filter;
  it exposes `ON`, `OFF`, `MIXED`, or `N/A`. Git repositories expose Update
  and Update all; both source kinds expose typed-confirmed whole-source
  Uninstall. **Extend to tool** links every recorded source to one tool radio
  after a per-source link preview that surfaces blocked sources; confirm
  stays disabled while any source is blocked or no new links are planned,
  and the batch stops at the first failure with a fresh snapshot.
- Discover is excluded from the `v0.5.0` public preview navigation and public
  Wails binding. Its experimental Go adapter/domain remains in the repository.
- Dashboard context metrics are filesystem estimates by default; **Run provider
  diagnostics** is the only UI action that executes local Claude/Codex
  diagnostic subprocesses. Muse is always a labeled filesystem estimate.
- Its `Update mode` column explicitly describes `Managed Git` sources as
  updateable and `Linked folder` sources as direct/live links that need no
  update; it does not encode this distinction as generic health states.
- Source actions are unavailable with pending toggles and become an exclusive
  non-cancellable operation while phase progress is shown. App close is
  blocked until that operation completes.
- `Cmd+F` focuses installed-skill search, `Cmd+R` refreshes, `Cmd+Enter` applies, and
  `Escape` closes transient drawers/dialogs when no source operation is active.
- Skill Set dialogs trap focus and support Escape when no mutation is active.
- At the 1024×720 minimum size, the sidebar collapses to icons and wide tables
  scroll horizontally. The design is dark-only.
- Source rows use opaque backend identifiers. Frontend mutations never submit
  filesystem paths or prebuilt operations; local paths enter only through the
  native picker.
- Skills scoped mutations likewise send only exact skill/group and tool
  identifiers. Search-expanded accordions restore manual session state when
  search is cleared, and manual expansion resets at app restart.

The visual contract and evidence confidence live in
[`../../../DESIGN.md`](../../../DESIGN.md); the preserved source raster is under
[`../../design/references/`](../../design/references/).

Exact command examples and flag placement belong in the user-facing
[`../../../README.md`](../../../README.md).

Skill Set CRUD and use are GUI-only in Phase 14; no CLI commands or TUI keys
are added.

Skill Advisor is CLI-only. Its standard public skill lives under
[`../../../skills/skill-advisor`](../../../skills/skill-advisor/) and uses the
existing install/update/uninstall source lifecycle plus Phase 18's ranked
search capability.

## TUI Model

- One row per skill with Claude, Codex, and Group columns.
- The active tool column determines the target of `Space` and `u`.
- Read-only rows are hidden until requested.
- Details show description, row group/source, tool-specific state and paths,
  entry type, symlink target, repository metadata, disabled path, pending state,
  and conflict information when available.
- Rescan reconstructs rows from disk and the manifest.

## Navigation And Actions

- `Tab`: change active tool column.
- `Up`/`Down` or `k`/`j`: move the selected row.
- `Space`: stage/unstage the active cell.
- `b`: stage/unstage every existing toggleable cell in the row.
- `g`: smart-toggle the selected group across all tools.
- `A`: smart-toggle all currently visible toggleable rows.
- `a` or `Enter`: plan and apply pending changes.
- `u` / `U`: remove the active pending change / clear all pending changes.
- `/`, `s`, `G`: edit text filter / cycle source filter / cycle group filter.
- `o`: show or hide read-only rows.
- `d`: show or hide details.
- `r`: rescan.
- `q`: require confirmation when pending changes exist.

## Filter And Bulk Scope

- Filters compose as text, then source, then group.
- Group choices reflect the rows remaining after text and source filters.
- `g` intentionally uses all loaded rows in the selected group, not merely the
  currently visible filtered subset.
- `A` intentionally uses only currently visible rows.
- Neither action creates a missing skill on the other tool side.
- Bulk status messages include updated/removed pending counts and skipped
  read-only/missing/conflict counts.

See [toggle-and-state-lifecycle.md](toggle-and-state-lifecycle.md) for the smart
target and apply semantics.
