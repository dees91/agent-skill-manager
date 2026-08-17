# Phase 14: Saved Skill Sets - Task Breakdown

> **Source**: user-approved planning session (2026-08-16).
> Iterations 1-13 are complete. Iteration 14 adds reusable, task-oriented skill
> recipes to the macOS app without changing CLI or TUI behavior.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- `Skill Set` is a user-defined collection of skill names plus an optional
  `When to use` description. It is separate from source-derived `Group`.
- Membership is tool-agnostic. Every use explicitly selects Claude, Codex, or
  both, then stages changes through the existing Pending/Apply model.
- Sets are overlapping smart-toggle recipes, not active profiles and not
  reference-counted owners of skills.
- Missing members remain recorded as unavailable and reconnect when the same
  skill basename is discovered again.
- The first surface is the macOS GUI. CLI and TUI behavior remain unchanged.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P14-T01 | Versioned Skill Set persistence and domain model | done | - |
| P14-T02 | Desktop projection, CRUD, toggle preview, and staging | done | P14-T01 |
| P14-T03 | Skill Sets workspace and contextual creation flows | done | P14-T02 |
| P14-T04 | Documentation, verification, and local build | done | P14-T03 |

## Task Definitions

### P14-T01: Versioned Skill Set Persistence And Domain Model

- Add a separate private `~/.skill-manager/skill-sets.json` store with atomic
  writes, bounded backups, validation, stable opaque IDs, and corruption
  isolation from core skill management.
- Persist unique names, optional descriptions, sorted unique skill basenames,
  and created/updated timestamps without tool selections or filesystem paths.

### P14-T02: Desktop Projection, CRUD, Toggle Preview, And Staging

- Project applied/effective per-tool set status and missing/conflict/read-only
  members through path-free GUI types.
- Add identifier-only CRUD, read-only smart-toggle preview, and scoped staging
  methods that reuse `staging.ToggleBatch`.
- Keep metadata edits independent from Pending, block them during source
  operations, and report non-blocking Skill Set impact during GUI uninstall.

### P14-T03: Skill Sets Workspace And Contextual Creation Flows

- Add a dedicated sidebar workspace with set summaries, member details,
  create/edit/delete dialogs, and an explicit Claude/Codex/Both toggle review.
- Add `Save as Skill Set` from Pending and `Add to Skill Set` from skill
  details while preserving the existing Apply boundary.
- Cover empty, unavailable, error, keyboard, screen-reader, and 1024x720 states.

### P14-T04: Documentation, Verification, And Local Build

- Synchronize AGENTS, README, DESIGN, the wiki, generated bindings, demo data,
  and repository-owned synthetic visual evidence.
- Run backend/frontend tests, vet, vulnerability checks, bindings generation,
  local CLI installation, and the ad-hoc-signed Apple Silicon GUI build.
- Do not tag or publish a release as part of this iteration.
