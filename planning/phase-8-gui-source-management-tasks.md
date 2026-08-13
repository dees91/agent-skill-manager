# Phase 8: GUI Source Management - Task Breakdown

> **Source**: user-approved planning session (2026-08-12). Iterations 1-7 are
> complete. Iteration 8 exposes the existing Git/local lifecycle safely in the
> macOS GUI.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Add one managed-only `Sources` screen; reserve a future `Discover` mode for
  possible `skills.sh` integration.
- Install uses inspect, per-skill/tool matrix review, then apply. Retain a newly
  cloned checkout when the flow is cancelled.
- Give each Claude/Codex matrix column one explicit bulk-selection toggle with
  `ON`, `OFF`, `MIXED`, and `N/A` feedback. It covers every discovered
  non-conflict target even when the text filter hides rows.
- Use native folder selection for local sources and opaque identifiers for all
  lifecycle mutations.
- Source operations are separately confirmed and blocked by pending skill
  toggles. Show phase progress without cancellation.
- Support per-repository Update, deterministic Update All, and typed-confirmed
  whole-source Uninstall.
- Explain source behavior directly in an `Update mode` column: `Managed Git`
  fetches through Update, while `Linked folder` reads changes directly and
  needs no update. Avoid ambiguous `Ready`/`Live` labels.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P8-T01 | Product, interface, and design contract | done | - |
| P8-T02 | Exact-cell planner and GUI lifecycle backend | done | P8-T01 |
| P8-T03 | Wails bindings and Sources interface | done | P8-T02 |
| P8-T04 | Tests, documentation, and packaging | done | P8-T03 |
| P8-T05 | Install-matrix column toggles | done | P8-T03 |

## Task Definitions

### P8-T01: Product, Interface, And Design Contract

- Record accepted ownership, install flow, source-action, progress, and future
  catalog boundaries in authoritative project and design documents.
- Define path-free mutation requests and serializable source/draft/review/result
  projections.

### P8-T02: Exact-Cell Planner And GUI Lifecycle Backend

- Add exact skill/tool cell planning while preserving CLI behavior.
- Add managed source projection, session drafts/reviews, install/update/
  uninstall orchestration, pending/busy guards, progress, and fresh rescans.
- Keep the existing install/update/uninstall services as the mutation layer.

### P8-T03: Wails Bindings And Sources Interface

- Add the native folder picker, progress events, close guard, generated
  bindings, demo backend, and the Sources navigation/view.
- Implement Git/local inspection, matrix review, update actions, typed
  uninstall confirmation, accessible modal behavior, and operational feedback.

### P8-T04: Tests, Documentation, And Packaging

- Add temporary-home/fake-Git backend tests and frontend behavior tests.
- Synchronize README, design contract, planning status, generated bindings, and
  wiki synthesis/log.
- Run the full Go/frontend/design/wiki/build verification and validate the
  ad-hoc-signed ARM64 bundle with an isolated temporary home.

### P8-T05: Install-Matrix Column Toggles

- Replace the implicit clickable Claude/Codex column headings with explicit
  bulk-selection toggles that report `ON`, `OFF`, `MIXED`, or `N/A`.
- Apply each toggle to every non-conflict candidate in its tool column,
  independent of the current text filter, while retaining exact-cell review.
- Cover bulk selection, mixed state, conflicts, filtering, accessibility,
  responsive layout, documentation, packaging, and local installation.
