# Phase 1: Skill Manager MVP - Task Breakdown

> **Source**: user-approved planning session (2026-05-08). The product scope, safety model, UX, and CLI shape were agreed before implementation.
>
> **Related**: Project-wide agent guidance is in [../AGENTS.md](../AGENTS.md). Claude Code entrypoint is [../CLAUDE.md](../CLAUDE.md).

---

> **MANDATORY FOR ALL AGENTS**
>
> The **Summary Table** at the bottom of this file is the single source of truth for task status. You MUST follow these rules:
>
> 1. **Before starting a task**: set its Status to `in-progress` in the Summary Table.
> 2. **After completing a task**: set its Status to `done` in the Summary Table.
> 3. **After completing a task**: check all tasks that list your completed task in their "Blocked By" column. If ALL blockers are now `done`, change their Status from `blocked` to `todo`.
> 4. **Do not start** a task that is `blocked`.
> 5. **Before marking `done`**: run task-specific verification plus the project verification available at that point. Once Go exists, this means `go test ./...` at minimum.
>
> Status values: `todo` = ready to pick up | `in-progress` = being worked on | `done` = completed | `blocked` = waiting on dependencies

---

## Dependency Graph

```text
Phase 1 MVP

  Foundation:
    P1-T01 [Go Module + Command Skeleton]
      -> P1-T02 [Domain Model + Path Defaults]
      -> P1-T03 [Skill Scanning: Managed Sources]
      -> P1-T04 [Metadata + Source Classification]
      -> P1-T05 [Read-Only Source Scanning]

  State + operations:
    P1-T02, P1-T03 -> P1-T06 [State Store + Disabled Layout]
    P1-T06 -> P1-T07 [Toggle Planner + Executor]
    P1-T07 -> P1-T08 [Conflict Handling]

  CLI:
    P1-T03, P1-T04, P1-T07 -> P1-T09 [CLI list/status/enable/disable]
    P1-T09 -> P1-T10 [CLI dry-run]

  TUI:
    P1-T03, P1-T04 -> P1-T11 [TUI Table Model]
    P1-T07, P1-T11 -> P1-T12 [TUI Pending Changes + Apply]
    P1-T05, P1-T11 -> P1-T13 [Read-Only Toggle + Filters]
    P1-T04, P1-T08, P1-T11 -> P1-T14 [Details Panel]

  Quality:
    P1-T07 -> P1-T15 [Backend Tests with Temp Homes]
    P1-T12 -> P1-T16 [TUI Model Tests]
    P1-T09, P1-T12, P1-T15 -> P1-T17 [README Verification Pass]
```

**Max parallelism:** After P1-T01 and P1-T02, scanning, state work, and CLI/TUI model work can be split, but file ownership should be coordinated to avoid churn in shared packages.

---

## Task Definitions

### P1-T01: Go Module + Command Skeleton

| Field | Value |
|---|---|
| Description | Initialize the Go project with a minimal command entrypoint. `go run .` should launch the same path as `skill-manager tui` once the TUI exists. No real scanning or mutation yet. |
| Blocked By | -- |
| Wave | foundation |
| Execution | Main |
| Effort | S |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New: `go.mod`
- New: `main.go`
- New: `internal/cli/cli.go`

**Acceptance Criteria:**
1. `go run .` compiles.
2. `skill-manager help` or equivalent usage output exists.
3. Subcommands are stubbed: `tui`, `list`, `status`, `enable`, `disable`.
4. Unknown commands return a non-zero exit with a useful message.
5. No filesystem mutation is implemented in this task.

**Completion Notes (2026-05-08):**
- Added a minimal Go module, `main.go`, and `internal/cli` command skeleton.
- `go run .` defaults to the `tui` stub; `help`, `tui`, `list`, `status`, `enable`, and `disable` are stubbed.
- Unknown commands return a non-zero exit with a usage hint.
- Added CLI skeleton tests and verified with `go test ./...`.

---

### P1-T02: Domain Model + Path Defaults

| Field | Value |
|---|---|
| Description | Define core types for tools, skill entries, states, source labels, planned operations, and default `$HOME`-based paths. Keep config file support out of MVP. |
| Blocked By | P1-T01 |
| Wave | foundation |
| Execution | Main |
| Effort | S |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New: `internal/model/model.go`
- New: `internal/paths/paths.go`

**Acceptance Criteria:**
1. Tool enum covers `claude` and `codex`.
2. Skill state can represent `ON`, `OFF`, `RO`, missing, pending, and conflict.
3. Source labels include `symlink repo`, `Skills CLI`, `local`, `Codex system`, `Claude plugin`, `unknown`.
4. Default paths are derived from `os.UserHomeDir()`.
5. No `config.toml` support is added.

**Completion Notes (2026-05-08):**
- Added domain enums and structs in `internal/model` for tools, skill states, sources, entry types, conflicts, and planned operations.
- Added `$HOME`-derived MVP paths in `internal/paths` with helper methods for tool-specific active and disabled directories.
- Added focused tests for tool parsing, required labels, and derived path layout.
- Verified with `go test ./...`.

---

### P1-T03: Skill Scanning - Managed Sources

| Field | Value |
|---|---|
| Description | Scan toggleable user skill directories: `~/.claude/skills` and `~/.agents/skills`. Include only entries with `SKILL.md`. Preserve symlink vs directory information. |
| Blocked By | P1-T02 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New: `internal/scan/scan.go`
- `internal/model/model.go`

**Acceptance Criteria:**
1. Scanner reads direct children of Claude and Codex managed skill directories.
2. Symlink entries are recorded as symlinks without dereferencing for mutation purposes.
3. Directory entries are recorded as directories.
4. Entries without `SKILL.md` are hidden.
5. Missing base directories are treated as empty, not fatal.
6. The same skill name can appear in both Claude and Codex state.

**Completion Notes (2026-05-08):**
- Added `internal/scan` scanner for managed Claude and Codex user skill directories.
- Scanner records symlink entries as symlinks, preserves raw symlink targets, records directory entries, and hides entries without a regular `SKILL.md`.
- Missing managed base directories scan as empty; other base directory errors are returned.
- Added row grouping for shared skill names across Claude and Codex.
- Added temp-home scanner tests and verified with `go test ./...`.

---

### P1-T04: Metadata + Source Classification

| Field | Value |
|---|---|
| Description | Read optional metadata for display and classify source type. Use `SKILL.md` frontmatter opportunistically and `~/.agents/.skill-lock.json` for Skills CLI labels. Detect git repo origin and commit for symlink targets when available. |
| Blocked By | P1-T03 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/scan/scan.go`
- New: `internal/metadata/metadata.go`

**Acceptance Criteria:**
1. Missing or malformed frontmatter does not block scanning.
2. Skill name falls back to directory name.
3. Description is shown when easy to parse.
4. Skills listed in `~/.agents/.skill-lock.json` are labeled `Skills CLI`.
5. Symlinks into git repositories are labeled `symlink repo`.
6. Local non-symlink managed skills are labeled `local`.
7. Git origin and short commit are collected when available, but failures are non-fatal.

**Completion Notes (2026-05-08):**
- Added tolerant `SKILL.md` frontmatter parsing for display name and description.
- Added Skills CLI lockfile name detection from `~/.agents/.skill-lock.json`.
- Managed scanner now labels Skills CLI, local directory, symlink repo, and unknown sources.
- Git origin and short commit are collected opportunistically for symlink targets without failing scans.
- Added metadata and scanner tests and verified with `go test ./...`.

---

### P1-T05: Read-Only Source Scanning

| Field | Value |
|---|---|
| Description | Scan read-only Codex system skills and Claude plugin cache skills for optional display. These entries must never be toggleable in MVP. |
| Blocked By | P1-T04 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/scan/scan.go`
- `internal/model/model.go`

**Acceptance Criteria:**
1. Codex system skills under `~/.codex/skills/.system` are listed as `RO`.
2. Claude plugin skills under `~/.claude/plugins/cache/**/skills/*/SKILL.md` are listed as `RO`.
3. Read-only entries are hidden by default in TUI data output.
4. Read-only entries cannot produce enable/disable operations.
5. Plugin marketplace checkout skills are not mutated.

**Completion Notes (2026-05-08):**
- Added read-only scanning for Codex system skills and Claude plugin cache skills.
- Read-only entries are emitted with `RO` state, read-only source labels, and `ReadOnly: true`.
- Default row grouping hides read-only cells; `RowOptions{IncludeReadOnly: true}` includes them for future TUI toggle behavior.
- Added `ToolSkill.Toggleable()` so read-only and missing cells cannot be used for toggle operations.
- Added read-only scanner and row-filter tests and verified with `go test ./...`.

---

### P1-T06: State Store + Disabled Layout

| Field | Value |
|---|---|
| Description | Implement `~/.skill-manager/state.json`, backup directory, and disabled entry layout. The store must preserve enough data to restore symlinks and directories exactly. |
| Blocked By | P1-T02, P1-T03 |
| Wave | state |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New: `internal/state/store.go`
- `internal/model/model.go`

**Acceptance Criteria:**
1. State directory is `~/.skill-manager`.
2. Disabled layout is `~/.skill-manager/disabled/claude/<skill>` and `~/.skill-manager/disabled/codex/<skill>`.
3. Manifest records original path, disabled path, entry type, symlink target, source, and timestamp.
4. Loading a missing state file returns empty state.
5. Saving state writes valid JSON.
6. Backup helper can copy existing manifest to `backups/state-<timestamp>.json`.

**Completion Notes (2026-05-08):**
- Added `internal/state` store for `~/.skill-manager/state.json`.
- Added disabled path layout helpers for `disabled/claude/<skill>` and `disabled/codex/<skill>` with invalid input checks.
- Manifest entries persist original path, disabled path, entry type, symlink target, source label, and timestamp.
- Added atomic JSON save, missing-manifest load behavior, backup copy helper, and manifest get/upsert/remove helpers.
- Added state store tests and verified with `go test ./...`.

---

### P1-T07: Toggle Planner + Executor

| Field | Value |
|---|---|
| Description | Implement backend planning and execution for enable/disable operations. TUI and CLI must share this backend. |
| Blocked By | P1-T06 |
| Wave | state |
| Execution | Main |
| Effort | L |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- New: `internal/ops/ops.go`
- `internal/state/store.go`
- `internal/model/model.go`

**Acceptance Criteria:**
1. Disable validates source exists and destination is free.
2. Enable validates disabled entry exists and original path is free.
3. Symlink disable moves the symlink itself, not the target.
4. Directory disable moves the directory.
5. State is updated after successful operations.
6. Apply order is deterministic: disables first, enables second, then sort by tool and skill name.
7. Batch stops on first failure and returns the completed and failed operation details.

**Completion Notes (2026-05-08):**
- Added shared operation planner and executor in `internal/ops`.
- Disable and enable planning validate source/destination paths and use state manifest data for restores.
- Apply sorts a copy deterministically, moves symlinks/directories with `os.Rename`, updates state after each successful operation, and stops on first failure.
- Added temp-home tests for symlink and directory round trips, validation failures, deterministic ordering, read-only rejection, and partial batch failure behavior.
- Verified with `go test ./...`.

---

### P1-T08: Conflict Handling

| Field | Value |
|---|---|
| Description | Detect restore conflicts and expose them in scan/model output so CLI and TUI can show `CONFLICT` without overwriting anything. |
| Blocked By | P1-T07 |
| Wave | state |
| Execution | Main |
| Effort | S |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/ops/ops.go`
- `internal/scan/scan.go`
- `internal/model/model.go`

**Acceptance Criteria:**
1. Enable plan fails if original path is occupied.
2. Scanner can surface disabled entries whose restore path is blocked.
3. Conflict includes original path, disabled path, and blocker path.
4. No automatic overwrite, delete, merge, or cleanup is implemented.

**Completion Notes (2026-05-08):**
- Added disabled manifest scanning so off skills appear in scan output.
- Restore blockers are surfaced as `CONFLICT` cells with original path, disabled path, blocker path, and manual-resolution guidance.
- Row grouping now prefers conflict cells over active blocker cells for the same tool and skill name.
- Added conflict scanner tests and no-mutation assertions for blocked enable planning.
- Verified with `go test ./...`.

---

### P1-T09: CLI list/status/enable/disable

| Field | Value |
|---|---|
| Description | Implement minimal CLI commands using the shared scanner and operations backend. |
| Blocked By | P1-T04, P1-T07 |
| Wave | cli |
| Execution | Main |
| Effort | M |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `main.go`

**Acceptance Criteria:**
1. `skill-manager list` prints skills with Claude state, Codex state, and source.
2. `skill-manager status` summarizes counts: on, off, conflict, read-only.
3. `skill-manager disable --tool claude <skill>` works.
4. `skill-manager enable --tool codex <skill>` works.
5. Invalid tool names produce a non-zero exit.
6. Attempts to mutate read-only skills fail cleanly.

**Completion Notes (2026-05-08):**
- Replaced CLI stubs for `list`, `status`, `enable`, and `disable` with scanner and ops-backed behavior.
- `list` prints grouped skill rows with Claude, Codex, and source columns; `status` counts ON, OFF, CONFLICT, and RO cells.
- `disable --tool claude <skill>` and `enable --tool codex <skill>` are covered by temp-home CLI tests.
- Invalid tools and read-only-only mutation attempts return non-zero with clear errors.
- Verified with `go test ./...`.

---

### P1-T10: CLI Dry-Run

| Field | Value |
|---|---|
| Description | Add `--dry-run` to CLI mutating commands. It must print planned filesystem operations and perform no mutation. |
| Blocked By | P1-T09 |
| Wave | cli |
| Execution | Main |
| Effort | S |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `internal/ops/ops.go`

**Acceptance Criteria:**
1. `disable --dry-run` prints source path and disabled destination.
2. `enable --dry-run` prints disabled path and restore destination.
3. Dry-run does not move files.
4. Dry-run does not update `state.json`.
5. Dry-run still reports validation errors and conflicts.

**Completion Notes (2026-05-08):**
- Added optional trailing `--dry-run` for CLI `enable` and `disable`.
- Dry-run prints the planned source-to-destination move after normal planning validation.
- Dry-run returns before `Apply`, so it does not move files, save state, or create backups.
- Added tests for disable dry-run, enable dry-run, conflict reporting, and invalid flag placement.
- Verified with `go test ./...`.

---

### P1-T11: TUI Table Model

| Field | Value |
|---|---|
| Description | Implement the main Bubble Tea model for one row per skill with separate Claude and Codex columns. Rendering can be plain but must be usable. |
| Blocked By | P1-T04 |
| Wave | tui |
| Execution | Main |
| Effort | L |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- New: `internal/tui/model.go`
- New: `internal/tui/view.go`
- `internal/cli/cli.go`

**Acceptance Criteria:**
1. Rows are grouped by skill name across Claude and Codex.
2. Columns include skill name, Claude state, Codex state, and source.
3. Active cell can move between Claude and Codex with `Tab`.
4. Basic navigation works.
5. `go run .` launches the TUI by default.

**Completion Notes (2026-05-08):**
- Added Bubble Tea TUI model and plain table rendering in `internal/tui`.
- Default TUI scans managed and disabled entries, groups rows across Claude and Codex, and hides read-only-only rows.
- Added `Tab` active tool switching, up/down and `j`/`k` navigation, and `q`/`ctrl+c` quit handling.
- Wired CLI default and `skill-manager tui` to launch the TUI through an injectable runner for tests.
- Added TUI model and CLI dispatch tests and verified with `go test ./...`.

---

### P1-T12: TUI Pending Changes + Apply

| Field | Value |
|---|---|
| Description | Add pending toggle behavior and apply flow to the TUI. Changes must not mutate the filesystem until apply. |
| Blocked By | P1-T07, P1-T11 |
| Wave | tui |
| Execution | Main |
| Effort | L |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/ops/ops.go`

**Acceptance Criteria:**
1. `Space` toggles pending state for active cell.
2. `b` toggles both tools where possible.
3. `a` or `Enter` applies pending changes.
4. `u` undoes pending change for active cell.
5. `U` clears all pending changes.
6. `q` warns or blocks when pending changes exist.
7. Apply uses the deterministic backend order from P1-T07.

**Completion Notes (2026-05-08):**
- Added in-memory pending operations for active-cell and both-tool toggles.
- Added `a`/`Enter` apply flow through `ops.Service`, with planning before mutation and completed pending cleanup by tool/skill.
- Added active-cell undo, clear-all pending, and guarded quit behavior when pending changes exist.
- Render pending cells as projected state transitions such as `ON->OFF` and `OFF->ON`.
- Added TUI tests for pending toggles, both-tool toggles, undo/clear, guarded quit, apply, and no filesystem mutation before apply.
- Verified with `go test ./...`.

---

### P1-T13: Read-Only Toggle + Filters

| Field | Value |
|---|---|
| Description | Add filtering controls: text filter, source filter, and show/hide read-only entries. |
| Blocked By | P1-T05, P1-T11 |
| Wave | tui |
| Execution | Main |
| Effort | M |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`

**Acceptance Criteria:**
1. Default TUI view hides read-only entries.
2. `o` toggles read-only visibility.
3. `/` filters by text.
4. `s` cycles or opens source filter.
5. Read-only cells cannot be toggled.
6. Hidden invalid entries without `SKILL.md` do not appear.

**Completion Notes (2026-05-08):**
- Added read-only visibility state and `o` toggle; read-only scan errors remain ignored by default and surface only when read-only visibility is enabled.
- Added case-insensitive text filtering with `/`, filter-edit mode, and source filtering with `s`.
- Added visible filter/read-only/source status line while keeping pending changes tracked even when filtered out.
- Kept read-only cells non-toggleable for `Space` and `b`.
- Added TUI tests for read-only visibility, text/source filters, invalid managed/read-only entries, mixed read-only rows, and applying hidden pending changes.
- Verified with `go test ./...`.

---

### P1-T14: Details Panel

| Field | Value |
|---|---|
| Description | Add a read-only details view for the selected skill. It should help debug source, symlink, and conflict state. |
| Blocked By | P1-T04, P1-T08, P1-T11 |
| Wave | tui |
| Execution | Main |
| Effort | M |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`

**Acceptance Criteria:**
1. `d` opens or toggles details.
2. Details show description when available.
3. Details show active paths for Claude and Codex when present.
4. Details show disabled paths when off.
5. Details show symlink target when present.
6. Details show repo origin and commit when detected.
7. Conflict details are visible when present.
8. Details are read-only.

**Completion Notes (2026-05-08):**
- Added `d` details toggle and footer help.
- Added read-only details rendering for selected rows, including description, source, state, pending operation, entry type, active/disabled paths, skill file, symlink target, repo origin/commit, and conflict fields.
- Enriched disabled scan rows with metadata from disabled `SKILL.md` files and repo metadata for disabled symlink entries.
- Added tests for details toggling, all required detail fields, conflict details, read-only behavior, and scanner-backed disabled metadata.
- Verified with `go test ./...`.

---

### P1-T15: Backend Tests with Temp Homes

| Field | Value |
|---|---|
| Description | Add tests for scanning, state, and operations using temporary directories. Never touch real global directories in tests. |
| Blocked By | P1-T07 |
| Wave | quality |
| Execution | Main |
| Effort | L |
| Scope | Test |
| Source | Planning |

**Files to modify:**
- New: `internal/scan/scan_test.go`
- New: `internal/state/store_test.go`
- New: `internal/ops/ops_test.go`

**Acceptance Criteria:**
1. Tests cover symlink skill scanning.
2. Tests cover local directory skill scanning.
3. Tests cover hiding entries without `SKILL.md`.
4. Tests cover Skills CLI source classification from lockfile.
5. Tests cover symlink disable/enable round trip.
6. Tests cover directory disable/enable round trip.
7. Tests cover conflict detection.
8. Tests cover deterministic operation ordering.
9. `go test ./...` passes.

**Completion Notes (2026-05-08):**
- Audited backend temp-home tests in `internal/scan`, `internal/state`, and `internal/ops` against every P1-T15 acceptance criterion.
- Added a no-git Skills CLI lockfile source classification test for directory installs.
- Strengthened directory disable/enable round-trip assertions to verify state manifest write and removal.
- Strengthened restore conflict planning assertions to check error context.
- Verified with `go test ./...`.

---

### P1-T16: TUI Model Tests

| Field | Value |
|---|---|
| Description | Test the TUI update/model layer for pending changes, filters, read-only visibility, and apply behavior. Full terminal snapshot testing is optional. |
| Blocked By | P1-T12 |
| Wave | quality |
| Execution | Main |
| Effort | M |
| Scope | Test |
| Source | Planning |

**Files to modify:**
- New: `internal/tui/model_test.go`

**Acceptance Criteria:**
1. Pending toggle does not mutate backend immediately.
2. Undo pending works.
3. Clear all pending works.
4. Read-only entries cannot be toggled.
5. Show/hide read-only changes visible row set.
6. Text filtering changes visible row set.
7. Apply invokes backend with expected operations.
8. `go test ./...` passes.

**Completion Notes (2026-05-08):**
- Audited existing TUI model tests against every P1-T16 acceptance criterion.
- Added read-only hide assertion after showing read-only rows.
- Added mixed apply coverage for one pending disable and one pending enable, including filesystem effects, pending cleanup, and rescan state.
- Added explicit pending-key ordering test for deterministic TUI apply planning.
- Verified with `go test ./...`.

---

### P1-T17: README Verification Pass

| Field | Value |
|---|---|
| Description | Update README after implementation so commands, build instructions, keybindings, and safety notes match actual behavior. |
| Blocked By | P1-T09, P1-T12, P1-T15 |
| Wave | docs |
| Execution | Main |
| Effort | S |
| Scope | Docs |
| Source | Planning |

**Files to modify:**
- `README.md`
- `AGENTS.md` if implementation intentionally changes a recorded decision

**Acceptance Criteria:**
1. README command examples work.
2. README includes TUI keybindings.
3. README documents state and disabled paths.
4. README describes conflict behavior.
5. README documents `--dry-run`.
6. `go test ./...` passes before marking this task done.

**Completion Notes (2026-05-08):**
- Updated README from planned-language to implemented MVP usage docs.
- Documented commands, exact `--dry-run` placement, TUI keybindings, managed/read-only paths, state layout, backup path, conflict behavior, and safety notes.
- Added and tested the `r` TUI rescan key so README, AGENTS, and implementation match.
- Verified `go run . help`, `go run . list`, `go run . status`, a synthetic
  temporary-home disable dry-run, a temporary CLI build, and `go test ./...`.

---

## Summary Table

> **Status legend:** `todo` = ready to pick up | `in-progress` = being worked on | `done` = completed | `blocked` = waiting on dependencies

| ID | Title | Blocked By | Wave | Execution | Effort | Scope | Source | Status |
|---|---|---|---|---|---|---|---|---|
| P1-T01 | Go Module + Command Skeleton | -- | foundation | Main | S | Core | Planning | done |
| P1-T02 | Domain Model + Path Defaults | P1-T01 | foundation | Main | S | Core | Planning | done |
| P1-T03 | Skill Scanning - Managed Sources | P1-T02 | foundation | Main | M | Core | Planning | done |
| P1-T04 | Metadata + Source Classification | P1-T03 | foundation | Main | M | Core | Planning | done |
| P1-T05 | Read-Only Source Scanning | P1-T04 | foundation | Main | M | Core | Planning | done |
| P1-T06 | State Store + Disabled Layout | P1-T02, P1-T03 | state | Main | M | Core | Planning | done |
| P1-T07 | Toggle Planner + Executor | P1-T06 | state | Main | L | Core | Planning | done |
| P1-T08 | Conflict Handling | P1-T07 | state | Main | S | Core | Planning | done |
| P1-T09 | CLI list/status/enable/disable | P1-T04, P1-T07 | cli | Main | M | CLI | Planning | done |
| P1-T10 | CLI Dry-Run | P1-T09 | cli | Main | S | CLI | Planning | done |
| P1-T11 | TUI Table Model | P1-T04 | tui | Main | L | TUI | Planning | done |
| P1-T12 | TUI Pending Changes + Apply | P1-T07, P1-T11 | tui | Main | L | TUI | Planning | done |
| P1-T13 | Read-Only Toggle + Filters | P1-T05, P1-T11 | tui | Main | M | TUI | Planning | done |
| P1-T14 | Details Panel | P1-T04, P1-T08, P1-T11 | tui | Main | M | TUI | Planning | done |
| P1-T15 | Backend Tests with Temp Homes | P1-T07 | quality | Main | L | Test | Planning | done |
| P1-T16 | TUI Model Tests | P1-T12 | quality | Main | M | Test | Planning | done |
| P1-T17 | README Verification Pass | P1-T09, P1-T12, P1-T15 | docs | Main | S | Docs | Planning | done |

## Effort Summary

| Category | Tasks | Total Effort |
|---|---|---|
| Foundation | P1-T01, P1-T02, P1-T03, P1-T04, P1-T05 | S + S + M + M + M |
| State + operations | P1-T06, P1-T07, P1-T08 | M + L + S |
| CLI | P1-T09, P1-T10 | M + S |
| TUI | P1-T11, P1-T12, P1-T13, P1-T14 | L + L + M + M |
| Quality | P1-T15, P1-T16 | L + M |
| Docs | P1-T17 | S |

## Implementation Notes

- Use Go and Bubble Tea.
- Keep UI and CLI text in English.
- Keep config support out of MVP.
- Keep plugin toggling out of MVP.
- Move symlinks as symlinks. Never dereference them for disable/enable.
- Tests must use temp homes and injected paths. Never mutate the real `~/.claude`, `~/.agents`, `~/.codex`, or `~/.skill-manager`.
- The first implementation can keep rendering plain. Correct state management and reversible operations matter more than visual polish.
