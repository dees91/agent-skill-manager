# Phase 4: Repository Update and Uninstall - Task Breakdown

> **Source**: user-approved planning session (2026-08-11). Iterations 1-3 are complete. Iteration 4 adds CLI-first fast-forward repository updates and strict whole-repository uninstall.
>
> **Related**: Product decisions are canonical in [../AGENTS.md](../AGENTS.md). Phase 3 install behavior is in [phase-3-repo-install-tasks.md](./phase-3-repo-install-tasks.md).

---

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to `in-progress` before starting it, set it to `done` only after its verification passes, and unblock dependents when all blockers are done. Do not start blocked tasks.

## Product Decisions

- Commands are CLI-only in this iteration:
  - `skill-manager update [<git-url>] [--dry-run]`
  - `skill-manager uninstall <git-url> [--dry-run]`
- `update` without a URL updates every repository recorded in Skill Manager state; a URL selects one normalized repository identity.
- Updates require a clean current branch tracking `origin/*` and use fetch plus fast-forward-only merge.
- Detached HEAD, missing/non-origin upstream, local-only commits, divergence, worktree changes, manifest/link drift, and remote removal/movement of an installed `SKILL.md` block update.
- New repository skills are not installed automatically.
- Update-all stops on the first failure and preserves the completed prefix.
- Update dry-run performs no fetch and explicitly reports that remote preflight is unavailable until real update.
- Uninstall removes the whole recorded repository: active and disabled skill symlinks, matching disabled state, repository state, and the managed checkout.
- Uninstall requires an explicit Git URL and has no `--all` or `--force` mode.
- Dirty checkouts, local-only commits, missing/changed recorded symlinks, and extra managed-directory symlinks into the checkout block uninstall.
- Uninstall stages exact validated paths under `~/.skill-manager/trash/`, saves state atomically, then deletes the staging directory; failures before state save roll back staged paths.
- `uninstall` supersedes the previously deferred `repo remove` command.
- Manifest version remains 1. No TUI repository actions are added.

## Task Definitions

### P4-T01: Product Contract and Plan

- Record the approved Iteration 4 semantics in this file and `AGENTS.md`.
- Add Phase 4 routing to `CLAUDE.md` during the documentation pass.
- Verification: documentation is internally consistent and the Summary Table is current.

### P4-T02: State, Paths, and Repository Reference Audit

- Add trash/staging path support and state helpers for repository removal.
- Add a shared, side-effect-free audit of manifest skill/tool cells against active and disabled symlinks.
- Reject malformed paths, duplicate cells, changed/missing expected symlinks, and extra managed symlinks into a checkout.
- Verification: focused state/path/audit tests with temporary homes.

### P4-T03: Repository Update Backend

- Add local dry-run validation and real fetch/preflight/fast-forward update services.
- Preflight every installed relative path in the target Git tree before merge.
- Persist `lastSeenCommit` per successfully updated repository and stop update-all on the first error.
- Verification: fake-runner unit tests and local-bare-repository integration tests.

### P4-T04: Update CLI

- Parse no-URL all-repository mode, single-URL mode, and strict `--dry-run`.
- Print deterministic per-repository results and clear local-only dry-run limitations.
- Verification: CLI tests for selection, output, no-op, failures, and no dry-run mutation.

### P4-T05: Transactional Whole-Repository Uninstall

- Plan exact active/disabled symlink, state, and checkout removal.
- Require a clean recoverable checkout and a clean reference audit.
- Stage, save, clean up, and roll back before-save failures without touching blockers or unrelated repositories.
- Verification: planner/apply tests including rollback and cleanup failure behavior.

### P4-T06: Uninstall CLI

- Parse one required repository URL plus optional `--dry-run`.
- Print planned/removed ON/OFF links and checkout summary.
- Verification: CLI dry-run, success, conflict, and read-only external-state tests.

### P4-T07: Full Regression Tests

- Cover update and uninstall edge cases described above without network or real home-directory mutation.
- Run `go test ./...`.

### P4-T08: README, Agent Guidance, Wiki, and Global Build

- Update README, AGENTS, CLAUDE, repository workflow/safety/interface/testing wiki pages, wiki source routing, and append-only log.
- Remove update/uninstall/repo-remove items from deferred work while retaining branch/force and other deferred features.
- Run `go test ./...`, `make dev`, and verify global CLI help.

## Summary Table

| ID | Title | Blocked By | Status |
|---|---|---|---|
| P4-T01 | Product Contract and Plan | -- | done |
| P4-T02 | State, Paths, and Repository Reference Audit | P4-T01 | done |
| P4-T03 | Repository Update Backend | P4-T02 | done |
| P4-T04 | Update CLI | P4-T03 | done |
| P4-T05 | Transactional Whole-Repository Uninstall | P4-T02 | done |
| P4-T06 | Uninstall CLI | P4-T05 | done |
| P4-T07 | Full Regression Tests | P4-T03, P4-T04, P4-T05, P4-T06 | done |
| P4-T08 | README, Agent Guidance, Wiki, and Global Build | P4-T07 | done |
