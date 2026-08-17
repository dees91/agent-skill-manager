# Phase 16: Skill Favorites - Task Breakdown

> **Source**: user-approved planning session (2026-08-17).
> Iterations 1-15 are complete. Iteration 16 adds private, tool-agnostic
> favorites to the macOS Skills workspace without changing CLI or TUI behavior.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- A favorite is one managed skill basename, independent of tool and ON/OFF
  state. Conflicted managed rows remain eligible; read-only-only rows do not.
- Persist favorites separately in `~/.skill-manager/favorites.json` with
  atomic owner-only writes and bounded independent backups.
- Missing favorites remain saved and reconnect when the same basename returns.
  Source uninstall warns but never rewrites favorite metadata.
- Add stars to Skills rows/details, a `Favorites` availability chip, and
  favorite-first ordering while preserving active-first placement and Pending.
- The first surface is the macOS GUI. CLI, TUI, list JSON, and Skill Advisor
  behavior remain unchanged.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P16-T01 | Versioned favorites persistence and safety boundary | done | - |
| P16-T02 | Desktop projection, mutation API, and uninstall impact | done | P16-T01 |
| P16-T03 | Skills favorites UX and demo integration | done | P16-T02 |
| P16-T04 | Documentation, verification, build, and local installation | done | P16-T03 |

## Task Definitions

### P16-T01: Versioned Favorites Persistence And Safety Boundary

- Add a separate private favorites file with normalized unique basenames,
  atomic replacement, independent backups, and corruption isolation.
- Add focused temporary-home tests for format, safety, permissions, retention,
  and idempotent mutations.

### P16-T02: Desktop Projection, Mutation API, And Uninstall Impact

- Project favorite state and page-local warnings through path-free GUI types.
- Add one identifier-only idempotent mutation and preserve Pending state.
- Warn about favorites affected by source uninstall without blocking or pruning.

### P16-T03: Skills Favorites UX And Demo Integration

- Add accessible row/detail stars, a Favorites availability chip, favorite-first
  ordering, temporary source-group expansion, and tailored empty/error states.
- Update the demo backend, fixtures, generated Wails bindings, and frontend
  behavior/accessibility coverage.

### P16-T04: Documentation, Verification, Build, And Local Installation

- Synchronize AGENTS, README, DESIGN, privacy/security docs, wiki, and synthetic
  visual evidence.
- Run all backend/frontend quality gates, build the CLI and ad-hoc-signed
  Apple Silicon app, install it in `/Applications`, launch-check, and commit.
- Do not tag, publish, push, or change the release version.
