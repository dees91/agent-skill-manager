# Phase 19: Muse Tool Support - Task Breakdown

> **Source**: user-approved planning session (2026-09-03).
> Iterations 1-18 are complete. Iteration 19 adds Muse as a third managed tool
> alongside Claude Code and Codex across the CLI, TUI, install/update/uninstall
> services, Skill Advisor, context budget, macOS GUI, first-party skill, and
> documentation.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Muse user skills live in `~/.config/muse/skills`, or
  `$XDG_CONFIG_HOME/muse/skills` when `XDG_CONFIG_HOME` is set to an absolute
  path. Disabled Muse entries live in `~/.skill-manager/disabled/muse/`.
- Muse is an independent third tool column, not shared ownership of
  `~/.agents/skills`. A Muse read of that directory from outside Skill Manager
  is out of scope and changes no classification.
- `--tool` accepts `claude`, `codex`, `muse`, `both`, and `all`. Empty, `both`,
  and `all` all target every supported tool.
- `list --json` and advisor JSON stay at `apiVersion: 1` with an additive
  `muse` cell. No capability change is required.
- Muse context reporting is always a labeled filesystem estimate (1% of an
  assumed 200,000-token context). No provider diagnostic is launched for Muse.
- `state.json` stays at manifest version 2; no migration is required.
- No config file support is added; the Muse path honors only the standard
  `XDG_CONFIG_HOME` environment variable.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P19-T01 | Domain core: ToolMuse, paths, scan rows/groups | done | - |
| P19-T02 | CLI tables, parsing, list JSON, advisor search | done | P19-T01 |
| P19-T03 | Install/update/uninstall planner and protected paths | done | P19-T01 |
| P19-T04 | TUI third column, details, and bulk toggles | done | P19-T01 |
| P19-T05 | Context budget Muse estimate and advisor parity | done | P19-T01 |
| P19-T06 | GUI backend projections and desktop frontend | done | P19-T01 |
| P19-T07 | First-party skill, README, DESIGN, and validation | done | P19-T02 |
| P19-T08 | AGENTS.md, wiki synthesis, branch, and PR | done | P19-T07 |
| P19-T09 | Extend managed sources to one tool (CLI/GUI) | done | P19-T06 |

## Task Definitions

### P19-T01: Domain Core

- Add `model.ToolMuse`, extend `ParseTool` and deterministic `Tools()` order.
- Add `SkillRow.Muse` and `GroupSummary.Muse`.
- Add `Paths.MuseUserSkills` with `XDG_CONFIG_HOME` handling and
  `Paths.MuseDisabledDir`; extend `UserSkillsDirFor` and `DisabledDirFor`.
- Extend scan row assembly, toggleable/source/group merges, and group counts.
- Extend `staging.SkillForTool`; state and ops stay generic with no migration.
- Cover with model, paths (including XDG), scan, and ops round-trip tests.

### P19-T02: CLI And Advisor

- Add the Muse column to `list`, `status`, and `groups` output.
- Accept `muse` in enable/disable `--tool`, `--available-for`, advisor
  `search`/`activate`/`status`, and update session-rotation messages.
- Add the additive `muse` cell to `list --json` at `apiVersion: 1`.
- Update CLI group-row test helpers and JSON assertions for three tools.

### P19-T03: Install And Uninstall Services

- Accept `muse`, `both`, and `all` in `ParseToolTarget`; empty, `both`, and
  `all` expand to every supported tool.
- Protect the Muse user skills directory against local-path installs.
- Update install, local install/uninstall, and planner tests for three-tool
  defaults, idempotency, conflicts, and rollback.

### P19-T04: TUI

- Render the third Muse column, cycle `Tab` across three tools, and show Muse
  in details, text/source matching, and the `b` row-toggle hint.
- Keep group (`g`) and all-visible (`A`) smart toggles over every tool with
  the existing pending-change safety model.
- Update batch-toggle tests for three-tool skip accounting.

### P19-T05: Context Budget And Advisor

- Add the Muse `ToolReport` as a labeled filesystem estimate with a 1%
  budget of an assumed 200,000-token context; project it independently.
- Keep `apiVersion: 1` and the existing `ranked_search_v1` capability.
- Cover with analyzer tests for Muse counting, budget, and projection.

### P19-T06: GUI And Desktop

- Project Muse cells, counts, summaries, candidates, sources, skill sets,
  favorites eligibility, and discover states through the GUI backend.
- Default group/visible bulk scopes to all supported tools.
- Add the Muse column, tool chip, install-matrix column, dashboard card and
  context row, skill-set summaries, bindings, fixtures, and demo data in the
  desktop frontend; update App tests for three-tool expectations.

### P19-T07: Skill, README, DESIGN, Validation

- Teach the first-party `skill-advisor` skill the `muse` host, flags, and
  instruction path.
- Update README paths, commands, TUI keys, desktop, state, and privacy notes.
- Update the DESIGN Skill Sets preview wording for three tools.
- Run `go test ./...`, `go vet ./...`, frontend typecheck/tests/build, and
  desktop `go test ./...` on synthetic homes only.

### P19-T08: AGENTS.md, Wiki, Branch, PR

- Update AGENTS.md paths, toggle semantics, TUI/CLI/install/advisor/budget
  contracts, and the state tree for Muse.
- Update wiki topics, index, and log.
- Commit from a dedicated branch and open a GitHub PR.

### P19-T09: Extend Managed Sources To One Tool

- Add `extend --tool <tool>` (strict dry-run): plan missing cells per managed
  source in manifest order, audit/disabled-mirror shared with install, stop at
  the first blocked source with an `extend --tool <tool> failed for source
  <group>` error. Nothing hardcodes `muse`.
- Add `PreviewExtend`/`ExtendSources` GUI methods plus `PreviewExtend`/
  `ExtendSources` desktop bindings and a Sources "Extend to tool" dialog with
  tool radio, live preview, and stop-at-first-failure finish.
- Cover with domain, CLI, GUI, and frontend tests; document in README, wiki,
  and the PR description.
