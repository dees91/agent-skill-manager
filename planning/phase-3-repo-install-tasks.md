# Phase 3: Repository Install - Task Breakdown

> **Source**: user-approved planning session (2026-05-08). Iteration 2 group and bulk-toggle behavior is complete. Iteration 3 adds CLI-first installation of skills from Git repositories using managed checkouts and symlinks.
>
> **Related**: Project-wide agent guidance is in [../AGENTS.md](../AGENTS.md). Phase 1 MVP tasks are in [phase-1-mvp-tasks.md](./phase-1-mvp-tasks.md). Phase 2 group and bulk-toggle tasks are in [phase-2-group-bulk-toggle-tasks.md](./phase-2-group-bulk-toggle-tasks.md).

---

> **MANDATORY FOR ALL AGENTS**
>
> The **Summary Table** at the bottom of this file is the single source of truth for task status. You MUST follow these rules:
>
> 1. **Before starting a task**: set its Status to `in-progress` in the Summary Table.
> 2. **After completing a task**: set its Status to `done` in the Summary Table.
> 3. **After completing a task**: check all tasks that list your completed task in their "Blocked By" column. If ALL blockers are now `done`, change their Status from `blocked` to `todo`.
> 4. **Do not start** a task that is `blocked`.
> 5. **Before marking `done`**: run task-specific verification plus `go test ./...`.
> 6. **After user-visible behavior changes**: run `make dev` so the global `skill-manager` command uses the latest binary.
>
> Status values: `todo` = ready to pick up | `in-progress` = being worked on | `done` = completed | `blocked` = waiting on dependencies

---

## Dependency Graph

```text
Phase 3 Repository Install

  Foundation:
    P3-T01 [Repo URL Normalization + Checkout Paths]
      -> P3-T03 [Git Checkout Service]
      -> P3-T05 [Install Planner + Preflight]

    P3-T02 [Install Manifest State]
      -> P3-T05 [Install Planner + Preflight]
      -> P3-T09 [Read-Only repos Command]

    P3-T04 [Recursive Skill Discovery]
      -> P3-T05 [Install Planner + Preflight]

  Apply:
    P3-T03, P3-T05 -> P3-T06 [Symlink Apply + Rollback]

  CLI:
    P3-T05 -> P3-T07 [install Dry-Run CLI]
    P3-T06, P3-T07 -> P3-T08 [install Apply CLI]
    P3-T09 can proceed after P3-T02

  Quality + docs:
    P3-T01, P3-T04, P3-T06, P3-T08, P3-T09 -> P3-T10 [Tests]
    P3-T08, P3-T09, P3-T10 -> P3-T11 [README/AGENTS Verification]
```

**Max parallelism:** P3-T01, P3-T02, and P3-T04 can start independently. P3-T09 can start after P3-T02. Keep planner and apply logic centralized so CLI, dry-run, and future TUI reuse the same rules.

---

## Product Decisions

- Install is CLI-first. No TUI install screen in Iteration 3.
- `skill-manager install <git-url>` clones repositories into `~/.skill-manager/repos/<host>/<repo-path>`.
- Installed skills are symlinks from `~/.claude/skills` and/or `~/.agents/skills` to directories inside the managed checkout.
- Default install target is `both`; `--tool claude`, `--tool codex`, and `--tool both` are supported.
- If no `--skill` is provided, install all valid skills discovered in the repo.
- `--skill <name>` may be repeated to install selected skills.
- A valid skill is any directory containing `SKILL.md`.
- Discovery is recursive, ignoring heavy/generated directories.
- Skill name is the basename of the directory containing `SKILL.md`.
- Duplicate discovered skill names are preflight conflicts.
- Existing matching checkouts are reused without `git pull`.
- Existing checkout with a different remote is a conflict.
- Existing active symlink to the same repo skill is already installed and OK.
- Existing disabled state entry for the same repo skill is already installed but OFF; install must not enable it.
- Preflight all selected skill/tool targets before creating symlinks.
- If symlink creation fails mid-apply, remove only symlinks created by that failed install operation.
- A clone created during a failed operation remains on disk.
- `install --dry-run` is strict and does not clone missing repositories.
- Install records Skill Manager managed repo metadata in `~/.skill-manager/state.json`.
- `skill-manager repos` is read-only and summarizes managed repo installs.
- Do not edit `.agents/.skill-lock.json`, Claude plugin metadata, Skills CLI lockfiles, or external manager lockfiles.
- Update, uninstall, branch/tag/commit selection, local path installs, shorthand URLs, submodules, sparse checkout, and `--force` are out of scope.

Future repository-management work is planned after Iteration 3:

- `skill-manager update`
- `skill-manager uninstall`
- `skill-manager repo remove`
- `skill-manager install --branch`
- `skill-manager install --force`

---

## Task Definitions

### P3-T01: Repo URL Normalization + Checkout Paths

| Field | Value |
|---|---|
| Description | Parse supported Git URLs into stable repo identities and managed checkout paths. |
| Blocked By | -- |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New package, e.g. `internal/install`
- `internal/model/model.go` if shared types are needed
- `internal/paths/paths.go`

**Acceptance Criteria:**
1. HTTPS URL `https://github.com/addyosmani/agent-skills` normalizes to host `github.com`, repo path `addyosmani/agent-skills`, and group `addyosmani/agent-skills`.
2. HTTPS URL with `.git` normalizes to the same identity.
3. SSH URL `git@github.com:addyosmani/agent-skills.git` normalizes to the same identity.
4. Checkout path resolves under `~/.skill-manager/repos/github.com/addyosmani/agent-skills`.
5. Unsupported inputs such as local paths and GitHub shorthand are rejected with clear errors.
6. Path resolution cannot escape `~/.skill-manager/repos`.

**Completion Notes:**
- Added `internal/install` repository identity normalization for supported HTTPS and `git@host:path.git` SSH URLs.
- Added managed checkout path resolution under `~/.skill-manager/repos`, plus `paths.Paths.ReposDir`.
- Hardened URL/path validation for local paths, shorthand inputs, unsafe path segments, explicit HTTPS ports, and containment escapes.
- Verified with `go test ./internal/install ./internal/paths` and `go test ./...`.

---

### P3-T02: Install Manifest State

| Field | Value |
|---|---|
| Description | Extend `state.json` to track Skill Manager managed repository installs alongside disabled entries. |
| Blocked By | -- |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | State |
| Source | Planning |

**Files to modify:**
- `internal/state/store.go`
- `internal/state/store_test.go`
- `internal/model/model.go` if shared manifest types are needed

**Acceptance Criteria:**
1. Existing `disabled` state remains backward compatible.
2. State can store repo URL, normalized host, normalized repo path, checkout path, group label, installed skills, install timestamp, and last seen commit.
3. Each installed skill entry stores name, relative path inside checkout, and tools.
4. Repository entries can be upserted without duplicating the same repo identity.
5. Tool lists are deterministic and sorted.
6. Existing backups still work before state mutation.

**Completion Notes:**
- Extended `state.Manifest` with managed repository entries and installed skill metadata while preserving disabled-entry compatibility.
- Added repository upsert/get helpers keyed by normalized host and repo path, with deterministic repository, skill, and tool ordering.
- Normalized state on load/save, including empty repository arrays and duplicate repository identity coalescing for malformed manifests.
- Verified with `go test ./internal/state` and `go test ./...`.

---

### P3-T03: Git Checkout Service

| Field | Value |
|---|---|
| Description | Add the service that clones missing repositories or reuses existing matching checkouts without pulling. |
| Blocked By | P3-T01 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New package, e.g. `internal/install`
- Tests using temp directories and a fake command runner or local git fixture

**Acceptance Criteria:**
1. Missing checkout plans and performs `git clone <url> <checkoutPath>` for real install.
2. Existing checkout with matching remote is reused.
3. Existing checkout with mismatched remote fails as a conflict.
4. Existing non-git directory at checkout path fails as a conflict.
5. No automatic `git pull` happens.
6. Last seen commit is collected when available.
7. Dry-run mode does not clone missing repositories.
8. Tests do not depend on network access.

**Completion Notes:**
- Added a checkout service with injectable Git runner support for network-free tests and real `git` execution in production.
- Implemented clone, dry-run would-clone, existing checkout reuse, remote identity comparison, checkout-root validation, and conflict handling.
- Collected last seen commit when available without failing checkout reuse/clone if commit lookup fails.
- Verified with `go test ./internal/install` and `go test ./...`.

---

### P3-T04: Recursive Skill Discovery

| Field | Value |
|---|---|
| Description | Discover installable skills inside a checkout by recursively finding `SKILL.md`. |
| Blocked By | -- |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New package, e.g. `internal/install`
- Discovery tests with temp checkout trees

**Acceptance Criteria:**
1. A directory containing `SKILL.md` is discovered as a valid skill.
2. Nested skills such as `skills/foo/SKILL.md` are discovered.
3. Skill name is the basename of the directory containing `SKILL.md`.
4. Invalid directories without `SKILL.md` are skipped.
5. `.git`, `node_modules`, `.venv`, `vendor`, `build`, and `dist` are ignored.
6. Duplicate discovered basenames fail preflight with a clear duplicate-name error.
7. Discovered relative paths are deterministic and sorted by skill name.

**Completion Notes:**
- Added recursive install discovery for root and nested skill directories containing `SKILL.md`.
- Skipped invalid directories and ignored generated/heavy directories, while keeping symlinked directories untraversed.
- Added deterministic sorting and duplicate basename errors that include the duplicate relative paths.
- Verified with `go test ./internal/install` and `go test ./...`.

---

### P3-T05: Install Planner + Preflight

| Field | Value |
|---|---|
| Description | Build install plans from repo identity, discovered skills, selected tools, selected names, current filesystem, and current state. |
| Blocked By | P3-T01, P3-T02, P3-T04 |
| Wave | foundation |
| Execution | Main |
| Effort | L |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New package, e.g. `internal/install`
- `internal/paths/paths.go` if helper paths are needed

**Acceptance Criteria:**
1. `--tool` defaults to `both`.
2. `claude`, `codex`, and `both` are accepted; invalid tool values fail.
3. No `--skill` means all discovered valid skills.
4. Repeated `--skill` selects only named skills.
5. Requested missing skills fail before any symlink is created.
6. Target path free means link can be planned.
7. Existing active symlink to the same repo skill is marked already installed and OK.
8. Existing disabled state entry pointing to the same repo skill is marked already installed but OFF and is not enabled.
9. Existing target path with any other file, directory, or symlink is a conflict.
10. Any conflict fails the whole plan before apply.
11. Planner does not edit lockfiles or external manager metadata.

**Completion Notes:**
- Added side-effect-free install planning with target parsing, skill selection, symlink link plans, already-installed ON/OFF detection, and structured preflight errors.
- Added deterministic missing-skill and conflict reporting, path-equivalent symlink comparison, and direct input validation for tools, checkout paths, and discovered skills.
- Covered default both-tool planning, selected skills, active/disabled idempotency, conflict preflight, and no lockfile/state mutation in tests.
- Verified with `go test ./internal/install` and `go test ./...`.

---

### P3-T06: Symlink Apply + Rollback

| Field | Value |
|---|---|
| Description | Apply clean install plans by creating symlinks, rolling back only links created by the failed operation, and updating the install manifest only on success. |
| Blocked By | P3-T03, P3-T05 |
| Wave | apply |
| Execution | Main |
| Effort | L |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New package, e.g. `internal/install`
- `internal/state/store.go`

**Acceptance Criteria:**
1. Clean plans create symlinks in `~/.claude/skills` and/or `~/.agents/skills`.
2. Symlinks point to discovered skill directories inside the managed checkout.
3. Existing same symlinks are left untouched.
4. Disabled same-skill entries are left disabled.
5. If symlink creation fails mid-apply, newly created symlinks from that operation are removed.
6. Rollback does not remove pre-existing symlinks.
7. Rollback does not remove the managed checkout.
8. State manifest is updated only after successful symlink creation.
9. State manifest records installed skill paths, tools, group, URL, checkout path, and commit.
10. Operation rescans cleanly through existing scan/group logic after success.

**Completion Notes:**
- Added install apply service for symlink creation, stale-plan revalidation, rollback of newly created symlinks, and repository manifest persistence.
- Merged repository manifest entries across repeated installs while preserving installed timestamp and updating commit/metadata.
- Covered successful rescan, already-installed ON/OFF entries, state-load-before-write, manifest merge, rollback behavior, and backup-once behavior in tests.
- Verified with `go test ./internal/install` and `go test ./...`.

---

### P3-T07: `install` Dry-Run CLI

| Field | Value |
|---|---|
| Description | Add `skill-manager install ... --dry-run` planning output without cloning, symlinking, or mutating state. |
| Blocked By | P3-T05 |
| Wave | cli |
| Execution | Main |
| Effort | M |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

**Acceptance Criteria:**
1. CLI accepts `install <git-url> [--tool claude|codex|both] [--skill name...] [--dry-run]`.
2. Dry-run for missing checkout prints that the repo would be cloned.
3. Dry-run for missing checkout does not attempt discovery.
4. Dry-run for existing checkout runs discovery and preflight.
5. Dry-run prints planned symlinks when known.
6. Dry-run prints conflicts when detected.
7. Dry-run does not create checkout directories, symlinks, or state changes.
8. Help text documents `install` and its flags.

**Completion Notes:**
- Added `install <git-url> ... --dry-run` CLI parsing, help text, checkout validation, discovery, and preflight output.
- Kept missing-checkout dry-run strict and side-effect free, while existing checkouts go through dry-run checkout validation before discovery.
- Added parser, missing-checkout, existing-checkout, selection, conflict, and checkout-conflict tests.
- Verified with `go test ./internal/cli ./internal/install`, `go test ./...`, and `make dev`.

---

### P3-T08: `install` Apply CLI

| Field | Value |
|---|---|
| Description | Wire real CLI install behavior through checkout, discovery, preflight, apply, state update, and final summary. |
| Blocked By | P3-T06, P3-T07 |
| Wave | cli |
| Execution | Main |
| Effort | L |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

**Acceptance Criteria:**
1. Real install clones missing checkout.
2. Real install reuses existing matching checkout without pull.
3. Real install fails on checkout remote mismatch.
4. Real install installs all discovered skills by default.
5. Real install respects repeated `--skill`.
6. Real install creates symlinks for selected tools.
7. Default install target is both tools.
8. Already installed same symlinks are reported as already installed and do not fail.
9. Already disabled same skills are reported as installed but OFF and are not enabled.
10. Conflicts fail before any symlink is created.
11. Partial symlink apply failure rolls back new symlinks.
12. Successful install prints a concise summary.
13. Successful install updates `state.json`.

**Completion Notes:**
- Wired real `install` CLI through checkout clone/reuse, discovery, planner preflight, symlink apply, state update, and concise summary output.
- Added network-free real clone coverage using Git URL rewrite, reuse-without-pull coverage, selection, idempotent ON/OFF, conflict, remote mismatch, and clone-failure tests.
- Real install now reports already installed ON/OFF entries and the active-session restart caveat.
- Verified with `go test ./internal/cli ./internal/install`, `go test ./...`, and `make dev`.

---

### P3-T09: Read-Only `repos` Command

| Field | Value |
|---|---|
| Description | Add `skill-manager repos` to inspect Skill Manager managed repository installs without mutating files. |
| Blocked By | P3-T02 |
| Wave | cli |
| Execution | Main |
| Effort | M |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

**Acceptance Criteria:**
1. `skill-manager repos` prints one row per managed repo manifest entry.
2. Output includes group, URL, checkout path, last seen commit, installed skill count, and tools.
3. Output is deterministic and sorted by group or repo identity.
4. Empty manifest prints a clear empty state.
5. Command does not mutate files or state.
6. Help text documents `repos`.

**Completion Notes:**
- Added read-only `repos` CLI output over the managed repository manifest, including empty-state handling, URL fallbacks, commit/checkouts, installed skill counts, and aggregated tool labels.
- Covered empty manifests, populated deterministic output, no state mutation, corrupt state errors, argument rejection, and help text in CLI tests.
- Verified with `go test ./internal/cli ./internal/state`, `go test ./...`, and `make dev`.

---

### P3-T10: Tests

| Field | Value |
|---|---|
| Description | Add focused and integration tests for repo install planning, checkout behavior, discovery, apply rollback, CLI output, and state persistence. |
| Blocked By | P3-T01, P3-T04, P3-T06, P3-T08, P3-T09 |
| Wave | quality |
| Execution | Main |
| Effort | L |
| Scope | Test |
| Source | Planning |

**Files to modify:**
- New install package tests
- `internal/state/store_test.go`
- `internal/cli/cli_test.go`
- Existing scan/ops tests only if needed for integration

**Acceptance Criteria:**
1. Tests cover HTTPS URL normalization with and without `.git`.
2. Tests cover SSH GitHub URL normalization.
3. Tests cover unsupported URL rejection.
4. Tests cover checkout path containment.
5. Tests cover clone, reuse, remote mismatch, no-pull behavior, and dry-run no-clone.
6. Tests cover recursive discovery, ignored dirs, invalid dirs, and duplicate basenames.
7. Tests cover default both-tool install and explicit single-tool install.
8. Tests cover selected `--skill` installs and missing selected skill failure.
9. Tests cover active same symlink idempotency.
10. Tests cover disabled same-skill idempotency without enabling.
11. Tests cover conflict preflight before apply.
12. Tests cover rollback of only newly created symlinks.
13. Tests cover install manifest persistence.
14. Tests cover `install --dry-run`.
15. Tests cover `skill-manager repos`.
16. `go test ./...` passes.

**Completion Notes:**
- Added CLI integration tests for missing selected skills during strict dry-run and unsupported install URLs without filesystem/state mutation.
- Coverage evidence:
  - Criteria 1-4: `TestNormalizeGitURLAcceptsSupportedForms`, `TestNormalizeGitURLRejectsUnsupportedInputs`, `TestCheckoutPathResolvesUnderManagedReposDir`, `TestCheckoutPathDefendsAgainstEscapingRepoPath`, `TestCheckoutPathDefendsAgainstUnsafeHost`.
  - Criterion 5: `TestEnsureCheckoutClonesMissingCheckout`, `TestEnsureCheckoutReusesExistingMatchingCheckout`, `TestEnsureCheckoutRejectsRemoteMismatch`, `TestEnsureCheckoutMissingDryRunDoesNotMutateOrRunGit`, `TestEnsureCheckoutDryRunValidatesExistingCheckout`, `TestRunInstallReusesExistingCheckoutWithoutPull`.
  - Criterion 6: `TestDiscoverSkillsFindsRootSkill`, `TestDiscoverSkillsFindsNestedSkillsAndSkipsInvalidDirs`, `TestDiscoverSkillsIgnoresHeavyGeneratedDirectories`, `TestDiscoverSkillsRejectsDuplicateSkillBasenames`.
  - Criteria 7-11: `TestPlanInstallDefaultsToBothToolsAndAllSkills`, `TestPlanInstallSupportsSingleToolAndSelectedSkills`, `TestPlanInstallReportsMissingSelectedSkills`, `TestPlanInstallTreatsSameActiveSymlinkAsAlreadyInstalled`, `TestPlanInstallTreatsSameDisabledSymlinkAsInstalledOff`, `TestPlanInstallPreflightConflicts`.
  - Criteria 12-13: `TestApplyRollsBackOnlyCreatedSymlinksOnFailure`, `TestApplyCreatesSymlinksUpdatesManifestAndRescans`, `TestApplyMergesRepositoryManifestAcrossInstalls`, `TestSaveLoadRepositoryManifest`, `TestRepositoryManifestHelpers`.
  - Criteria 14-15: `TestRunInstallDryRunMissingCheckoutDoesNotMutate`, `TestRunInstallDryRunExistingCheckoutPlansSymlinks`, `TestRunInstallDryRunReportsMissingSelectedSkillWithoutMutation`, `TestRunReposEmptyManifestDoesNotCreateState`, `TestRunReposPrintsManagedRepositoriesReadOnly`.
- Verified with `go test ./internal/cli ./internal/install ./internal/state` and `go test ./...`.

---

### P3-T11: README/AGENTS Verification

| Field | Value |
|---|---|
| Description | Update documentation after implementation so install/repo behavior matches actual code, and mark this phase complete. |
| Blocked By | P3-T08, P3-T09, P3-T10 |
| Wave | docs |
| Execution | Main |
| Effort | S |
| Scope | Docs |
| Source | Planning |

**Files to modify:**
- `README.md`
- `AGENTS.md`
- `CLAUDE.md`
- `planning/phase-3-repo-install-tasks.md`

**Acceptance Criteria:**
1. README documents implemented `install` examples and `repos`.
2. README documents active-session restart caveat.
3. AGENTS records final install semantics in present tense.
4. This task file has accurate statuses and completion notes.
5. `go test ./...` passes.
6. `make dev` has been run after implementation.

**Completion Notes:**
- Updated README from planned repository-install wording to implemented `install` and `repos` usage, including supported HTTPS/SSH inputs, out-of-scope inputs/options, strict dry-run behavior, checkout reuse, symlink install behavior, manifest state, `repos` output fields, and active-session restart caveat.
- Updated AGENTS CLI wording to present tense for Iteration 3 repository install commands and widened CLAUDE guidance to include repository-install documentation work.
- Verified with `go test ./...` and `make dev`.

---

## Summary Table

> **Status legend:** `todo` = ready to pick up | `in-progress` = being worked on | `done` = completed | `blocked` = waiting on dependencies

| ID | Title | Blocked By | Wave | Execution | Effort | Scope | Source | Status |
|---|---|---|---|---|---|---|---|---|
| P3-T01 | Repo URL Normalization + Checkout Paths | -- | foundation | Main | M | Core | Planning | done |
| P3-T02 | Install Manifest State | -- | foundation | Main | M | State | Planning | done |
| P3-T03 | Git Checkout Service | P3-T01 | foundation | Main | M | Core | Planning | done |
| P3-T04 | Recursive Skill Discovery | -- | foundation | Main | M | Core | Planning | done |
| P3-T05 | Install Planner + Preflight | P3-T01, P3-T02, P3-T04 | foundation | Main | L | Core | Planning | done |
| P3-T06 | Symlink Apply + Rollback | P3-T03, P3-T05 | apply | Main | L | Core | Planning | done |
| P3-T07 | `install` Dry-Run CLI | P3-T05 | cli | Main | M | CLI | Planning | done |
| P3-T08 | `install` Apply CLI | P3-T06, P3-T07 | cli | Main | L | CLI | Planning | done |
| P3-T09 | Read-Only `repos` Command | P3-T02 | cli | Main | M | CLI | Planning | done |
| P3-T10 | Tests | P3-T01, P3-T04, P3-T06, P3-T08, P3-T09 | quality | Main | L | Test | Planning | done |
| P3-T11 | README/AGENTS Verification | P3-T08, P3-T09, P3-T10 | docs | Main | S | Docs | Planning | done |

## Effort Summary

| Category | Tasks | Total Effort |
|---|---|---|
| Foundation | P3-T01, P3-T02, P3-T03, P3-T04, P3-T05 | M + M + M + M + L |
| Apply | P3-T06 | L |
| CLI | P3-T07, P3-T08, P3-T09 | M + L + M |
| Quality | P3-T10 | L |
| Docs | P3-T11 | S |

## Implementation Notes

- Keep install backend independent of CLI formatting so a future TUI install screen can reuse it.
- Do not test against real global skill directories.
- Do not rely on network in tests; use fakes or local git fixtures.
- Keep `install --dry-run` side-effect free.
- Do not add update or uninstall behavior in this phase.
- Keep future `update`, `uninstall`, `repo remove`, `install --branch`, and `install --force` work out of Iteration 3 implementation.
- Do not edit external lockfiles.
- Preserve existing toggle semantics for installed symlinks.
- Run `make dev` after user-visible implementation changes so the global command is updated.
