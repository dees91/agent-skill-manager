# Phase 10: Active-First Skills Workspace - Task Breakdown

> **Source**: user-approved planning session (2026-08-13).
> Iterations 1-9 are complete. Iteration 10 restructures the macOS Skills
> screen for catalogs where only a small fraction of installed skills is active.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Show conflicts first, active skills in an always-expanded section, and OFF
  skills in source-grouped accordions that start collapsed.
- Keep each skill in exactly one section. Pending changes remain in the section
  determined by applied state until Apply and rescan.
- Keep search plus state/tool chips on the first line; move Group, Source, and
  Read only into an advanced Filters panel.
- Keep both Claude and Codex columns visible. The selected tool chip controls
  classification and bulk mutation scope.
- Global bulk covers all filtered results, including collapsed rows. Group bulk
  covers the complete group. Both show eligible cell counts and remain pending.
- Preserve group expansion while navigating inside one app session and reset it
  on application restart.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P10-T01 | Scoped smart-toggle backend and Wails bindings | done | - |
| P10-T02 | Active-first grouped Skills workspace | done | P10-T01 |
| P10-T03 | Behavior, accessibility, and responsive coverage | done | P10-T02 |
| P10-T04 | Documentation, packaging, and local installation | done | P10-T03 |

## Task Definitions

### P10-T01: Scoped Smart-Toggle Backend And Wails Bindings

- Accept exact skill/group identifiers plus selected tool names only.
- Validate and deduplicate identifiers in Go, then reuse `staging.ToggleBatch`.
- Retain the existing both-tool methods as compatibility wrappers.

### P10-T02: Active-First Grouped Skills Workspace

- Add `Needs attention`, `Active now`, `Available by source`, and optional
  `Read only` sections without duplicating skills.
- Add compact state/tool chips, advanced filters, global search expansion, and
  source headers with whole-group actions and counts.
- Keep pending rows stable until Apply and keep group expansion in App session
  state.

### P10-T03: Behavior, Accessibility, And Responsive Coverage

- Cover classification, collapsed groups, search expansion, navigation memory,
  pending placement, conflicts, exact tool scope, and complete group scope.
- Verify keyboard semantics, contrast, axe-core, and the 1024×720 minimum.

### P10-T04: Documentation, Packaging, And Local Installation

- Synchronize AGENTS, README, DESIGN, wiki synthesis/log, generated bindings,
  and desktop version `0.4.0`.
- Run all Go/frontend tests, vet, typecheck/build, dependency audit, production
  packaging, signature/architecture checks, installation, and launch.
