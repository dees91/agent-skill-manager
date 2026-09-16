# Phase 23: Install as OFF - Task Breakdown

> **Source**: user-approved planning session (2026-09-16).
> Iterations 1-22 are complete. Iteration 23 lets an install create skills
> directly as OFF, so a source can be added for every tool without any tool
> seeing its skills until the user turns them ON.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Desktop and CLI. The CLI adds `install ... --off`; the desktop install dialog
  adds one **Install as OFF** switch used by both Install source and Install
  new. It applies to the whole selection, not per cell.
- New links are created directly at `~/.skill-manager/disabled/<tool>/<skill>`
  (0700). They never appear in the active skills directory. Creating ON and
  then disabling, as extend does, is not used.
- Each link created OFF gets the disabled record a manual disable would write,
  with `source` and `group` from the scanner's managed classifier. The source
  manifest and all disabled records are saved together.
- Already ON cells stay ON and the result says so; already OFF cells stay OFF.
- Install as OFF adds the conflict "disabled path already exists".
- A failed Git install state save now removes the links it created, matching
  local install and the no-partial-manifest rule. This also applies to ON
  installs.
- The review records the mode, and apply always uses the reviewed mode.
- The Iteration 22 update hint, Discover install, extend, TUI, toggle, update,
  uninstall, and repair are unchanged. Moving extend's create-then-disable onto
  this primitive is a possible follow-up.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P23-T01 | Core OFF planning, link creation, records, and rollback | done | - |
| P23-T02 | CLI `install --off` | done | P23-T01 |
| P23-T03 | GUI review/apply mode and bindings | done | P23-T01 |
| P23-T04 | Desktop Install as OFF switch | done | P23-T03 |
| P23-T05 | AGENTS, CLAUDE, usage, and wiki documentation | done | P23-T02 |

## Task Definitions

### P23-T01: Core OFF planning, link creation, records, and rollback

- `PlanOptions.Off`, `LinkPlan.DisabledPath` and `LinkPath()`, and disabled
  path conflicts in `internal/install/planner.go`.
- Shared `createPlannedLinks`, `addDisabledRecords`, and `rollbackCreated` in
  `internal/install/link_create.go`; Git save failure rolls back.
- `scan.Scanner.ManagedSymlinkClassifier` keeps record labels identical to a
  manual disable.
- Verification: `go test ./internal/install/...`, including parity with a
  manual disable on a non-GitHub origin, audits, uninstall planning, enable,
  local sources, the Skills CLI lock, and rollback.

### P23-T02: CLI `install --off`

- Parse `--off`, print `mode: install as OFF`, OFF dry-run links, the created
  count as OFF with an `enable` hint, and a note for already ON cells.
- Verification: `go test ./internal/cli/...`.

### P23-T03: GUI review/apply mode and bindings

- `ReviewInstall(draftID, selections, off)`, `InstallReview.Off`, and apply
  from the reviewed mode; regenerate Wails bindings.
- Verification: `go test ./internal/gui/...` and `cd desktop && go test ./...`.

### P23-T04: Desktop Install as OFF switch

- Switch above the install matrix in both flows; toggling clears the review;
  summary and apply button show OFF; demo and test backends echo the mode.
- Verification: `npm run typecheck`, `npm test`, and `make gui-build`.

### P23-T05: AGENTS, CLAUDE, usage, and wiki documentation

- Record the decisions in `AGENTS.md`, add the Phase 23 route to `CLAUDE.md`,
  document `--off` and the switch in `docs/usage.md`, and update the wiki.
- Verification: no document describes install as always creating skills ON.
