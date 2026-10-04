# Phase 27: Source Skill Removal - Task Breakdown

> **Source**: user-approved plan (2026-10-04). A managed Git repository can
> remove a skill that Skill Manager recorded as installed. Then the update
> preflight stops with an error, and the update of all repositories stops at
> that repository. Before this phase, the only remedy was to uninstall the full
> source and install it again from the CLI. The desktop app had no remedy.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before you start it. Set it to `done` only after the
> verification is successful.

## Product Decisions

- Add one new operation: remove recorded skills from one source. The
  operation removes only the links and the records of the selected skills.
  The source and its other skills stay installed.
- The operation is available for Git sources and for local sources.
- The operation does not change the managed checkout, the local source
  directory, or the remote refs. It does not fetch. It does not require a
  clean checkout or a recoverable upstream, because it does not remove the
  checkout.
- The operation removes these items for each selected skill:
  - The exact active and disabled links of the skill in each recorded tool.
  - The disabled records of these links.
  - The entry of the skill in `installedSkills` or in the local source entry.
- You can identify a skill by its source name or by its install name.
  Iteration 26 makes sure that each name identifies only one skill of a
  source.
- The operation uses the same reference audit as uninstall. A missing,
  changed, duplicate, or extra link of the source stops the operation before
  it changes a file.
- The operation does not let you remove all the recorded skills of a source.
  To remove all of them, uninstall the source.
- The operation moves the links into `~/.skill-manager/trash/` before it
  writes `state.json`. If an error occurs, it moves the links back. After a
  successful save, it deletes the staging directory. This is the same model as
  uninstall.
- Skill Sets and favorites that use a removed install name stay unchanged. The
  desktop confirmation shows the affected Skill Sets and favorites. They do
  not stop the operation. The CLI does not show them, because the CLI does not
  read these stores. This is the same as for the uninstall of a full source.
- If the checkout contains the skill, the new-skill report of Iteration 22
  shows it again after the removal. This is correct: there is no ignore list.
- The manifest version does not change.

Update failure:

- When the update preflight finds recorded skills that have no regular
  `SKILL.md` at the target commit, it returns a typed error. The error kind is
  `skill-missing-upstream`.
- The typed error gives each affected skill with its source name, its install
  name, and its checkout-relative path. The error message text does not
  change.
- The CLI adds the exact remedy command after the error:
  `skill-manager uninstall <url> --skill <name>...`.
- The desktop update dialog shows the cause and a **Remove from source**
  button. The button opens a confirmation dialog for the new operation, with
  the affected skills selected.
- After the removal, the dialog does not start the update again. You start
  the update again. This keeps each source operation separately confirmed.
- The update of all repositories continues to stop at the first failure.

Rejected alternatives:

- Update that removes missing skills automatically. Skill Manager must not
  remove a skill without a confirmation.
- One combined "remove and update" operation. Two confirmed operations are
  easier to roll back.
- A relink of a skill that moved to a different path in the repository. Update
  does not move links. This case stays out of scope.

## Interfaces

CLI:

```text
skill-manager uninstall <git-url|local-path> [--skill <name>...] [--dry-run]
```

- Without `--skill`, the command removes the full source. This behavior does
  not change.
- With `--skill`, the command removes only the named skills. Each name must
  identify a recorded skill of the source. An unknown name stops the command
  before it changes a file.
- The dry-run output shows each link and record that the command will remove.
- The output recommends a new Claude, Codex, Muse, or Grok session.

Desktop:

- A **Remove skills** source action opens a dialog with a checklist of the
  recorded skills of the source. The dialog shows each skill by its install
  name, and by its source name when the names are different.
- The confirmation shows the links that the operation will remove and the
  affected Skill Sets and favorites. It does not ask you to type the group
  name, because the source and its checkout stay installed.
- The confirmation button stays disabled when no skill is selected or when all
  the skills of the source are selected.
- The action is a source operation in the exclusive source lane. Pending
  Skills toggles stop it, as for the other source operations.
- The frontend sends an opaque source identifier and skill names. No
  filesystem path crosses the desktop bridge.

Example (synthetic): the source `example/agent-skills` records the skill
`beta` with the install name `beta-example`. A new commit removes `beta`. The
update stops with `skill-missing-upstream`. The CLI prints
`skill-manager uninstall https://github.com/example/agent-skills --skill beta`.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P27-T01 | Domain: skill removal service for Git and local sources | done | - |
| P27-T02 | Typed `skill-missing-upstream` update error | done | - |
| P27-T03 | CLI `uninstall --skill`, dry-run output, remedy command | done | P27-T01, P27-T02 |
| P27-T04 | GUI backend: removal preview, apply, failure kind | done | P27-T01, P27-T02 |
| P27-T05 | Desktop: Remove skills dialog and update remedy button | done | P27-T04 |
| P27-T06 | AGENTS, usage, DESIGN, and wiki documentation | done | P27-T03, P27-T05 |

## Task Definitions

### P27-T01: Domain: skill removal service

- Add a removal service in `internal/install/` with `Plan` and `Apply`, next
  to `UninstallService` and the local uninstall.
- Resolve each name to one recorded entry by source name or install name.
  Stop on an unknown name, on a duplicate name, or when the selection contains
  all the recorded skills.
- Run the full reference audit of the source before the first change.
- Stage the exact links in a `remove-skill-<operation-id>` directory in the
  trash. Remove the disabled records. Save the manifest one time. Delete the
  staging directory after the save.
- Roll back the staged links when the save is not successful. Keep the
  staging directory and name it in the error when the rollback is not
  complete.
- Verification: `go test ./internal/install/...`. Include these tests:
  - An aliased skill, a disabled link, and a local source.
  - An unknown name and the selection of all skills.
  - A rollback after a save error.
  - A skill that has no `SKILL.md` at the remote target commit.

### P27-T02: Typed update error

- Replace the `fmt.Errorf` in `preflightInstalledSkillsAtCommit` with a typed
  error that contains each affected skill. Keep the message text.
- Give the error the kind `skill-missing-upstream` , for `errors.As` in
  the callers.
- Verification: `go test ./internal/install/ -run Update`.

### P27-T03: CLI

- Parse a repeatable `--skill` option for `uninstall`. Keep the full-source
  behavior when the option is absent.
- Print the dry-run plan.
- After a `skill-missing-upstream` update error, print the exact remedy
  command with shell-quoted arguments.
- Update the help text.
- Verification: `go test ./internal/cli/`.

### P27-T04: GUI backend

- Add a removal preview and an apply method to the GUI service and to the
  desktop `App`. The preview accepts a source identifier. The apply method
  accepts a source identifier and skill names.
- The preview gives each recorded skill with its link counts, its favorite
  flag, and its Skill Set names. The dialog adds these values, so a change of
  the selection does not call the backend.
- Add `Kind`, `Cause`, `Remedy`, and the affected install names to
  `SourceMutationFailure` for `skill-missing-upstream`. Use the same
  classification path as the checkout conflicts of Iteration 21.
- Return a fresh snapshot after each apply, as for the other source
  mutations.
- Verification: `go test ./internal/gui/`, `make gui-bindings`, and
  `go test ./... && go vet ./...` in `desktop`.

### P27-T05: Desktop

- Add the **Remove skills** source action, the checklist dialog, and the
  confirmation with the link and impact preview.
- In the update dialog, show the cause and the **Remove from source** button
  for `skill-missing-upstream`. Open the confirmation with the affected skills
  selected.
- Add the operation to the demo backend.
- Verification: `make gui-test`. Include a test for the remedy flow from the
  update dialog of all repositories.

### P27-T06: Documentation

- Update `docs/usage.md` for `uninstall --skill` and the remedy command.
- Update `DESIGN.md` for the dialog and the remedy button.
- Update the wiki topic pages and append an entry to `docs/wiki/log.md`.
- Verification: the wiki relative links resolve, `git diff --check` is clean,
  and each example uses synthetic sources.

## Non-Goals

- A relink of a skill that moved to a different path in the repository.
- An ignore list for skills that you do not want reported.
- An automatic removal during update.
- An option to continue the update of all repositories after a failure.
- A TUI surface for the new operation.
