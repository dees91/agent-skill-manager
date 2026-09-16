# Phase 22: New Skill Discovery for Managed Sources - Task Breakdown

> **Source**: user-approved planning session (2026-09-16).
> Iterations 1-21 are complete. Iteration 22 makes skills that a managed Git
> repository gained after installation visible, without changing the rule that
> `update` never installs them. The trigger was an upstream repository adding
> skills after install: `update` fetched them, but nothing told the user they
> existed.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- A new skill is a skill that `DiscoverSkills` finds in the managed checkout
  whose name is not recorded in the repository's `installedSkills`. Matching is
  by name, so a recorded skill that moved to another path is not reported.
- There is no dismiss or ignore list and no new state. A skill stops being new
  only when it is installed.
- `update` still never installs, links, or records new skills. After a
  successful update it lists the new skills per repository and prints the exact
  `skill-manager install <url> --skill <name>...` command. The command adds
  `--tool <tool>` only when the repository records exactly one tool.
- A discovery failure after a successful update (for example a duplicate skill
  name) is reported as a warning and does not turn the update into a failure.
- `update --dry-run` lists new skills already present in the local checkout. It
  still does not fetch, so skills that exist only upstream are not shown.
- GUI source health (`InspectSources`) carries `newSkills` for sources whose
  health is `ok`. Health stays an explicit read-only inspection.
- The Sources row shows the new skill count and names plus an `Install new`
  action. It reuses the Git install flow on the recorded URL, which reuses the
  checkout without pulling. The matrix preselects new skills for tools the
  source already uses and never preselects conflict or already-installed cells.
  The user can change the selection before review.
- Update result messages mention how many new skills are available.
- Toggle, uninstall, extend, and repair semantics do not change.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P22-T01 | Core `NewSkills` and update result reporting | done | - |
| P22-T02 | CLI update and dry-run new skill output | done | P22-T01 |
| P22-T03 | GUI health `newSkills` and update message | done | P22-T01 |
| P22-T04 | Desktop row badge and `Install new` preselection | done | P22-T03 |
| P22-T05 | AGENTS, CLAUDE, usage, and wiki documentation | done | P22-T02 |

## Task Definitions

### P22-T01: Core `NewSkills` and update result reporting

- Add `NewSkills(repository state.RepositoryEntry)` in
  `internal/install/new_skills.go`.
- Add `NewSkills` and `NewSkillsError` to `UpdateResult`, filled after the
  successful state save in `UpdateService.Apply`.
- Verification: `go test ./internal/install/...` covering name matching, moved
  recorded skills, duplicate-name errors, and update still not installing.

### P22-T02: CLI update and dry-run new skill output

- Print new skills and the install command after each update result and in the
  dry-run loop; print a warning when discovery fails.
- Verification: `go test ./internal/cli/...` for apply and dry-run output.

### P22-T03: GUI health `newSkills` and update message

- Add `NewSkills []string` to `SourceHealth`, fill it in
  `inspectRepositoryHealth` for `ok` sources, and extend the update message.
- Regenerate Wails models.
- Verification: `go test ./internal/gui/...` and `cd desktop && go test ./...`.

### P22-T04: Desktop row badge and `Install new` preselection

- Render the new skill note and `Install new` action on healthy rows.
- Let `InstallDialog` start from a recorded URL, inspect immediately, and
  preselect only new skills for the source's tools.
- Update demo and test backends.
- Verification: `npm run typecheck`, `npm test`, and `make gui-build`.

### P22-T05: AGENTS, CLAUDE, usage, and wiki documentation

- Amend the Iteration 4 update rule and Sources screen decisions in `AGENTS.md`,
  add the Phase 22 routing step to `CLAUDE.md`, document the output in
  `docs/usage.md`, and update the wiki topic and source pages plus `log.md`.
- Verification: no document still says new skills are silently ignored.
