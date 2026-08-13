# Phase 2: Groups and Bulk Toggle - Task Breakdown

> **Source**: user-approved planning session (2026-05-08). Iteration 1 MVP is complete. Iteration 2 adds source-package grouping, group filtering, group toggle, and all-visible toggle.
>
> **Related**: Project-wide agent guidance is in [../AGENTS.md](../AGENTS.md). Phase 1 MVP tasks are in [phase-1-mvp-tasks.md](./phase-1-mvp-tasks.md).

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
Phase 2 Groups + Bulk Toggle

  Foundation:
    P2-T01 [Group Model]
      -> P2-T02 [Group Detection]
      -> P2-T03 [Group Rows + Summaries]

  TUI:
    P2-T03 -> P2-T04 [Replace Source Column with Group]
    P2-T03, P2-T04 -> P2-T05 [Group Filter]
    P2-T03, P2-T04 -> P2-T06 [Smart Batch Toggle Engine]
    P2-T06 -> P2-T07 [TUI Group Toggle]
    P2-T06 -> P2-T08 [TUI All Visible Toggle]
    P2-T07, P2-T08 -> P2-T09 [Bulk Toggle Messages + Details]

  CLI:
    P2-T03 -> P2-T10 [Read-Only groups Command]

  Quality + docs:
    P2-T02, P2-T06, P2-T10 -> P2-T11 [Tests]
    P2-T04, P2-T07, P2-T08, P2-T10 -> P2-T12 [README/AGENTS Verification]
```

**Max parallelism:** After P2-T03, TUI display/filtering and CLI group summary can proceed in parallel. Batch toggle work should stay centralized to avoid duplicating smart-toggle rules.

---

## Product Decisions

- Group = automatically detected source package/repo/collection, not manual config.
- GitHub repo groups use `owner/repo`.
- `Source` remains the installation mechanism; `Group` becomes the primary table column.
- `g` smart-toggles the selected row's group across both tools.
- `A` smart-toggles all visible rows across both tools.
- `G` cycles group filter.
- Bulk toggles create pending changes only; `a`/`Enter` still applies.
- Bulk toggles skip read-only, missing, and conflict cells.
- One-sided skills are valid and only toggle their existing side.
- Iteration 2 adds `skill-manager groups` read-only.
- Mutating group/all CLI commands are out of scope for Iteration 2.

---

## Task Definitions

### P2-T01: Group Model

| Field | Value |
|---|---|
| Description | Add group fields and types to the domain model without changing behavior yet. |
| Blocked By | -- |
| Wave | foundation |
| Execution | Main |
| Effort | S |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/model/model.go`

**Acceptance Criteria:**
1. `ToolSkill` has a group label field.
2. `SkillRow` has a group label field.
3. Group can represent `example-labs/engineering-skills`, `sample-org/product-skills`, `Skills CLI`, `local`, `Codex system`, `Claude plugin`, `unknown`.
4. Existing source labels remain unchanged.
5. Existing tests still pass.

**Completion Notes:**
- Added typed `GroupLabel` model support with source-derived constants and arbitrary repo label support.
- Added `Group` fields to `ToolSkill` and `SkillRow`, plus model tests covering labels and field exposure.
- Verified with `go test ./internal/model` and `go test ./...`.

---

### P2-T02: Group Detection

| Field | Value |
|---|---|
| Description | Detect group labels during scanning. Symlink git repos should become stable repo labels, preferably GitHub `owner/repo`. Non-repo and read-only sources use source-derived group labels. |
| Blocked By | P2-T01 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/scan/scan.go`
- `internal/metadata/metadata.go` if helper parsing belongs there
- `internal/scan/scan_test.go`

**Acceptance Criteria:**
1. GitHub HTTPS remote `https://github.com/example-labs/engineering-skills.git` maps to `example-labs/engineering-skills`.
2. GitHub SSH remote `git@github.com:sample-org/product-skills.git` maps to `sample-org/product-skills`.
3. Non-GitHub remotes produce a stable fallback label.
4. Git repos without remote fall back to repo root directory name.
5. Skills CLI installs use `Skills CLI`.
6. Local managed dirs use `local`.
7. Codex system and Claude plugin read-only skills use their read-only group labels.
8. Disabled entries preserve or recover group labels.

**Completion Notes:**
- Added scan-time group detection for managed, read-only, and disabled skill entries.
- Added GitHub HTTPS/SSH parsing, non-GitHub remote fallback labels, and no-remote repo-root fallback labels.
- Persisted group labels through planned operations and disabled state entries so disabled skills preserve group data.
- Added focused scan, ops, and state tests for source-derived groups, repo groups, disabled recovery, and persistence.
- Verified with `go test ./internal/scan ./internal/ops ./internal/state` and `go test ./...`.

---

### P2-T03: Group Rows + Summaries

| Field | Value |
|---|---|
| Description | Propagate group labels into grouped rows and add summary helpers for counts per group. These helpers will power TUI filters and the CLI `groups` command. |
| Blocked By | P2-T02 |
| Wave | foundation |
| Execution | Main |
| Effort | M |
| Scope | Core |
| Source | Planning |

**Files to modify:**
- `internal/scan/scan.go`
- `internal/model/model.go`
- New or existing summary helper file, e.g. `internal/scan/groups.go`

**Acceptance Criteria:**
1. `RowsFromSkillsWithOptions` sets `SkillRow.Group`.
2. Rows with both tools from the same repo show that repo group.
3. Rows with mixed groups fall back to `unknown` unless a deterministic single group is available.
4. Summary helper returns group name, total row count, Claude ON/OFF/CONFLICT/RO counts, Codex ON/OFF/CONFLICT/RO counts, and source labels.
5. Summaries are sorted by group name.

**Completion Notes:**
- Propagated cell group labels into `SkillRow.Group` with deterministic merge rules.
- Added group summary model types and `scan.GroupSummaries` with per-tool state counts and sorted source labels.
- Added row merge and summary tests covering same-group, mixed-group, unknown/empty, one-sided, read-only, and source aggregation behavior.
- Verified with `go test ./internal/model ./internal/scan` and `go test ./...`.

---

### P2-T04: Replace Source Column with Group

| Field | Value |
|---|---|
| Description | Update TUI table rendering to show `Group` instead of `Source`. Keep source information in the details panel. |
| Blocked By | P2-T03 |
| Wave | tui |
| Execution | Main |
| Effort | S |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/view.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. Main table header uses `Group`.
2. Main table rows show group labels.
3. Details panel shows both `Group` and `Source`.
4. Existing source filter remains available until group filter replaces or complements it.
5. TUI viewport behavior from Iteration 1 remains intact.

**Completion Notes:**
- Replaced the main TUI table's `Source` column with `Group` while leaving source filtering/status/help unchanged.
- Added row-level and tool-cell `Group` lines to the details panel alongside existing `Source` lines.
- Adjusted details viewport height reservation for the added lines and added regression tests for table group rendering.
- Verified with `go test ./internal/tui`, `go test ./...`, and `make dev`.

---

### P2-T05: Group Filter

| Field | Value |
|---|---|
| Description | Add group filtering to the TUI. `G` cycles group filters similarly to the current source filter. |
| Blocked By | P2-T03, P2-T04 |
| Wave | tui |
| Execution | Main |
| Effort | M |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. `G` cycles through available groups and back to all.
2. Status line shows active group filter.
3. Text filter and group filter compose correctly.
4. Source filter behavior remains available with `s`.
5. Cursor is clamped after group filter changes.

**Completion Notes:**
- Added `G` group filter cycling, group filter status text, and help text for `G group`.
- Composed filters as text -> source -> group, with group choices scoped to the visible source-filtered set.
- Matched group filtering to the visible `SkillRow.Group` column, including mixed-row `unknown` behavior.
- Added tests for cycling, status, text/source composition, cursor clamping, edit-mode behavior, and source-driven group reset.
- Verified with `go test ./internal/tui`, `go test ./...`, and `make dev`.

---

### P2-T06: Smart Batch Toggle Engine

| Field | Value |
|---|---|
| Description | Add shared TUI batch-toggle logic for a set of rows/cells. It must implement smart target selection and pending merge behavior without applying changes. |
| Blocked By | P2-T03, P2-T04 |
| Wave | tui |
| Execution | Main |
| Effort | L |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. If every applicable cell in scope is ON, batch creates pending disables.
2. Otherwise, batch creates pending enables for OFF cells.
3. Read-only cells are skipped.
4. Missing cells with no restore state are skipped.
5. Conflict cells are skipped.
6. Existing same pending operations are removed on repeated batch toggle.
7. Existing opposite pending operations are replaced.
8. If all applicable cells already have the same pending operation, batch toggle removes them all.
9. Batch logic returns counts for changed, removed, read-only skipped, missing skipped, and conflict skipped.

**Completion Notes:**
- Added shared TUI batch toggle engine with effective-state target selection and per-cell changed/removed/skip counts.
- Ensured pending changes remain valid relative to real ON/OFF state by canceling existing pending operations when that is the correct effective toggle.
- Added tests for all-ON disables, mixed enables, read-only/missing/conflict skips, one-sided rows, repeated batch undo, opposite replacement, effective pending cancellation, and changed/removed counts.
- Verified with `go test ./internal/tui` and `go test ./...`.

---

### P2-T07: TUI Group Toggle

| Field | Value |
|---|---|
| Description | Wire `g` to smart-toggle the selected row's group across both Claude and Codex. |
| Blocked By | P2-T06 |
| Wave | tui |
| Execution | Main |
| Effort | M |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. Pressing `g` uses the selected row's group.
2. It adds/removes/replaces pending changes across both tools for that group.
3. One-sided skills are handled without errors.
4. It does not apply changes immediately.
5. Help text includes `g group`.

**Completion Notes:**
- Wired `g` to smart-toggle the selected row's normalized group across both Claude and Codex.
- Scoped group toggles over all loaded rows, not only currently visible rows, while leaving apply behavior pending-only.
- Added help text and tests for all-loaded group scope, one-sided rows, repeated undo, empty/unknown groups, no selection, and no filesystem mutation.
- Verified with `go test ./internal/tui`, `go test ./...`, and `make dev`.

---

### P2-T08: TUI All Visible Toggle

| Field | Value |
|---|---|
| Description | Wire `A` to smart-toggle all currently visible toggleable rows across both Claude and Codex. |
| Blocked By | P2-T06 |
| Wave | tui |
| Execution | Main |
| Effort | M |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. Pressing `A` uses rows after text/source/group/read-only filters.
2. It adds/removes/replaces pending changes across both tools.
3. Read-only-only rows do not create pending changes.
4. It does not apply changes immediately.
5. Help text includes `A all`.

**Completion Notes:**
- Wired `A` to smart-toggle all currently filtered table rows across both Claude and Codex.
- Added help text and result-based messages for empty visible scopes, no toggleable cells, and pending updates.
- Added tests for filtered scope, both-tool staging, read-only no-op, viewport-independent visibility, filter input mode, and no filesystem mutation.
- Verified with `go test ./internal/tui`, `go test ./...`, and `make dev`.

---

### P2-T09: Bulk Toggle Messages + Details

| Field | Value |
|---|---|
| Description | Make batch actions transparent in the TUI by reporting updated and skipped counts, and exposing group data in details. |
| Blocked By | P2-T07, P2-T08 |
| Wave | tui |
| Execution | Main |
| Effort | S |
| Scope | TUI |
| Source | Planning |

**Files to modify:**
- `internal/tui/model.go`
- `internal/tui/view.go`
- `internal/tui/model_test.go`

**Acceptance Criteria:**
1. Group toggle message includes group name and changed count.
2. All-visible toggle message includes visible scope and changed count.
3. Messages include skipped conflict count when non-zero.
4. Messages include no-op explanation when nothing applicable exists.
5. Details panel shows `Group`.

**Completion Notes:**
- Added shared bulk-toggle message formatting with updated, changed, removed, and skipped counts.
- Included skip counts for conflict, read-only, and missing cells in both group and all-visible bulk messages.
- Added no-op messages with skip explanations for groups and visible rows with no applicable cells.
- Verified existing details panel `Group` rendering through TUI tests.
- Verified with `go test ./internal/tui`, `go test ./...`, and `make dev`.

---

### P2-T10: Read-Only `groups` Command

| Field | Value |
|---|---|
| Description | Add `skill-manager groups` to summarize detected groups without mutating anything. |
| Blocked By | P2-T03 |
| Wave | cli |
| Execution | Main |
| Effort | M |
| Scope | CLI |
| Source | Planning |

**Files to modify:**
- `internal/cli/cli.go`
- `internal/cli/cli_test.go`

**Acceptance Criteria:**
1. `skill-manager groups` prints one row per group.
2. Output includes group name, row count, Claude counts, Codex counts, and sources.
3. Command includes read-only groups.
4. Command does not mutate files or state.
5. Help text documents `groups`.

**Completion Notes:**
- Added read-only `skill-manager groups` command with group, row count, per-tool state counts, and source columns.
- Reused scan group summaries over rows that include managed, disabled, conflict, and read-only skills.
- Documented `groups` in CLI help and rejected extra command arguments.
- Added CLI tests for output rows, read-only groups, no state/file mutation, help, and usage errors.
- Verified with `go test ./internal/cli` and `go test ./...`.

---

### P2-T11: Tests

| Field | Value |
|---|---|
| Description | Expand tests for group detection, group summaries, group filter, group toggle, all-visible toggle, and `groups` CLI output. |
| Blocked By | P2-T02, P2-T06, P2-T10 |
| Wave | quality |
| Execution | Main |
| Effort | L |
| Scope | Test |
| Source | Planning |

**Files to modify:**
- `internal/scan/scan_test.go`
- `internal/tui/model_test.go`
- `internal/cli/cli_test.go`

**Acceptance Criteria:**
1. Tests cover GitHub HTTPS group labels.
2. Tests cover GitHub SSH group labels.
3. Tests cover local, Skills CLI, Codex system, and Claude plugin groups.
4. Tests cover disabled entry group preservation.
5. Tests cover `g` group smart toggle.
6. Tests cover `A` all-visible smart toggle with filters.
7. Tests cover repeated batch toggle undo behavior.
8. Tests cover skipped conflict/read-only/missing counts.
9. Tests cover `skill-manager groups`.
10. `go test ./...` passes.

**Completion Notes:**
- Audited existing Phase 2 tests against all P2-T11 acceptance criteria and confirmed coverage for GitHub HTTPS/SSH labels, local/Skills CLI/system/plugin groups, group preservation, smart toggles, and skipped counts.
- Added CLI integration coverage for repo-derived, Skills CLI, and persisted disabled group rows in `skill-manager groups`, including row-specific counts/sources and no state mutation.
- Added direct TUI key-handler coverage for pressing `A` twice to undo pending all-visible changes.
- Hardened the git-backed CLI fixture against ambient signing and hook configuration.
- Verified with `go test ./internal/cli`, `go test ./internal/tui`, and `go test ./...`.

---

### P2-T12: README/AGENTS Verification

| Field | Value |
|---|---|
| Description | Update documentation after implementation so keybindings, group semantics, and CLI behavior match actual code. |
| Blocked By | P2-T04, P2-T07, P2-T08, P2-T10 |
| Wave | docs |
| Execution | Main |
| Effort | S |
| Scope | Docs |
| Source | Planning |

**Files to modify:**
- `README.md`
- `AGENTS.md`
- `planning/phase-2-group-bulk-toggle-tasks.md`

**Acceptance Criteria:**
1. README documents `g`, `A`, and `G`.
2. README documents `skill-manager groups`.
3. AGENTS records final group and bulk toggle semantics.
4. Task statuses are updated correctly.
5. `go test ./...` passes.
6. `make dev` has been run after implementation.

**Completion Notes:**
- Updated README command docs for `go run . groups` and `skill-manager groups`, including read-only group summary columns and included skill scopes.
- Documented TUI `g`, `A`, and `G` keys with implemented group/all-visible scope, pending-only behavior, skip behavior, and group/source distinction.
- Updated AGENTS to record final group display, state, TUI key, bulk-toggle, and read-only `groups` CLI semantics in present tense.
- Verified with `go test ./...`, `make dev`, and subagent docs review.

---

## Summary Table

> **Status legend:** `todo` = ready to pick up | `in-progress` = being worked on | `done` = completed | `blocked` = waiting on dependencies

| ID | Title | Blocked By | Wave | Execution | Effort | Scope | Source | Status |
|---|---|---|---|---|---|---|---|---|
| P2-T01 | Group Model | -- | foundation | Main | S | Core | Planning | done |
| P2-T02 | Group Detection | P2-T01 | foundation | Main | M | Core | Planning | done |
| P2-T03 | Group Rows + Summaries | P2-T02 | foundation | Main | M | Core | Planning | done |
| P2-T04 | Replace Source Column with Group | P2-T03 | tui | Main | S | TUI | Planning | done |
| P2-T05 | Group Filter | P2-T03, P2-T04 | tui | Main | M | TUI | Planning | done |
| P2-T06 | Smart Batch Toggle Engine | P2-T03, P2-T04 | tui | Main | L | TUI | Planning | done |
| P2-T07 | TUI Group Toggle | P2-T06 | tui | Main | M | TUI | Planning | done |
| P2-T08 | TUI All Visible Toggle | P2-T06 | tui | Main | M | TUI | Planning | done |
| P2-T09 | Bulk Toggle Messages + Details | P2-T07, P2-T08 | tui | Main | S | TUI | Planning | done |
| P2-T10 | Read-Only `groups` Command | P2-T03 | cli | Main | M | CLI | Planning | done |
| P2-T11 | Tests | P2-T02, P2-T06, P2-T10 | quality | Main | L | Test | Planning | done |
| P2-T12 | README/AGENTS Verification | P2-T04, P2-T07, P2-T08, P2-T10 | docs | Main | S | Docs | Planning | done |

## Effort Summary

| Category | Tasks | Total Effort |
|---|---|---|
| Foundation | P2-T01, P2-T02, P2-T03 | S + M + M |
| TUI | P2-T04, P2-T05, P2-T06, P2-T07, P2-T08, P2-T09 | S + M + L + M + M + S |
| CLI | P2-T10 | M |
| Quality | P2-T11 | L |
| Docs | P2-T12 | S |

## Implementation Notes

- Keep group detection deterministic and filesystem-derived.
- Do not add manual group config in Iteration 2.
- Do not add mutating group/all CLI commands in Iteration 2.
- Reuse the existing pending/apply backend; bulk actions should only populate pending changes.
- Preserve existing single-skill controls: `Space`, `b`, `a`/`Enter`, `u`, `U`.
- Keep `s` source filter for now; add `G` group filter rather than replacing `s`.
- Tests must use temp homes and injected paths.
- Run `make dev` after user-visible implementation changes so the global command is updated.
