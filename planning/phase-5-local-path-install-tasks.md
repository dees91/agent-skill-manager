# Phase 5: Local Path Install - Task Breakdown

> **Source**: user-approved planning session (2026-08-11). Iterations 1-4 are complete. Iteration 5 adds link-in-place installation and whole-source uninstall for local folders.
>
> **Related**: Product decisions are canonical in [../AGENTS.md](../AGENTS.md). Git install/update/uninstall behavior remains defined by the Phase 3 and Phase 4 plans.

---

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to `in-progress` before starting it, set it to `done` only after its verification passes, and unblock dependents when all blockers are done. Do not start blocked tasks.

## Product Decisions

- `skill-manager install <local-path>` accepts absolute, explicit relative, home-relative, and existing bare-relative directories.
- Local sources use link-in-place semantics. Skill Manager never copies, updates, moves, or deletes their source directories.
- A root containing a regular `SKILL.md` is one skill and is not scanned below that root. Otherwise discovery is recursive with the existing ignored-directory and duplicate-basename rules.
- Existing `--tool`, repeated `--skill`, and `--dry-run` options apply unchanged.
- Local installs use source label `local path` and the source root basename as their group.
- Local source identity is its canonical absolute path after symlink resolution. Reinstalling the same source is idempotent and may add newly discovered skills.
- Protected Skill Manager, Claude, Codex, disabled, system, and plugin paths must not overlap a local source in either direction.
- `skill-manager uninstall <local-path> [--dry-run]` removes the full recorded local installation: exact active/disabled symlinks and matching state only. The source is never staged or removed.
- Local uninstall may clean recorded broken symlinks after the source is moved or deleted, while still rejecting changed or extra managed references.
- Update-all ignores local sources. A targeted local update reports that link-in-place sources are already live and do not require update.
- `repos` remains Git-only. No TUI repository/source actions or separate local-source summary command are added.
- `state.json` advances to version 2 with a `localSources` collection. Version 1 loads as an empty-local-source manifest and is written as version 2 on the next mutation; unknown newer versions are rejected.

## Task Definitions

### P5-T01: Product Contract and State Foundation

- Record the accepted local-source semantics in this file and `AGENTS.md`.
- Add manifest v2 migration, local-source records, deterministic normalization, and lookup/update/remove helpers.
- Verification: state migration and persistence tests.

### P5-T02: Local Path Resolution and Discovery

- Detect local inputs without changing supported Git URL behavior.
- Expand home/relative paths, canonicalize symlinks, validate directories and protected-path overlap, and implement single-skill-root discovery.
- Verification: path, containment, and discovery tests.

### P5-T03: Local Install Planning and Apply

- Reuse selection and target preflight semantics while enforcing single-source ownership.
- Create exact symlinks, support ON/OFF idempotency, back up state, roll back failed applies, and persist local-source ownership only after success.
- Verification: planner/apply and strict dry-run tests.

### P5-T04: Local Reference Audit and Uninstall

- Audit active/disabled local-source links without requiring the source to still exist.
- Reuse transactional trash staging for links only; preserve blockers and source directories and retain recovery data after incomplete rollback.
- Verification: active/OFF/missing-source/conflict/rollback/cleanup tests.

### P5-T05: CLI and Scan Integration

- Dispatch install/uninstall inputs between Git and local sources.
- Add local-path output, targeted-update guidance, manifest-backed `local path` source/group classification, and unchanged Git-only `repos` behavior.
- Verification: CLI and scanner tests.

### P5-T06: Regression, Documentation, Wiki, and Global Build

- Run the full suite and vet, update README/agent guidance/wiki/log, and remove local installs from deferred work.
- Run `make dev` and verify global help and local dry-run behavior using temporary sources only.

## Summary Table

| ID | Title | Blocked By | Status |
|---|---|---|---|
| P5-T01 | Product Contract and State Foundation | -- | done |
| P5-T02 | Local Path Resolution and Discovery | P5-T01 | done |
| P5-T03 | Local Install Planning and Apply | P5-T02 | done |
| P5-T04 | Local Reference Audit and Uninstall | P5-T03 | done |
| P5-T05 | CLI and Scan Integration | P5-T03, P5-T04 | done |
| P5-T06 | Regression, Documentation, Wiki, and Global Build | P5-T05 | done |
