# Phase 20: Grok Tool Support - Task Breakdown

> **Source**: user-approved planning session (2026-09-03, revised: PR0 refactor first).
> PR0 rewires hardcoded frontend tool lists to `MANAGED_TOOLS` with no
> behavior change. PR1 adds Grok (`~/.grok/skills`) as a fourth managed tool
> across the CLI, TUI, install/update/uninstall services, Skill Advisor,
> context budget, macOS GUI, first-party skill, and documentation.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Tables are the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Grok user skills live in `~/.grok/skills` from `$HOME` (per official xAI
  docs). Disabled Grok entries live in `~/.skill-manager/disabled/grok/`.
- Grok is an independent fourth tool column, not shared ownership of
  `~/.agents/skills`.
- `--tool` accepts `claude`, `codex`, `muse`, `grok`, `both`, and `all`.
  Empty, `both`, and `all` all target every supported tool.
- `list --json` and advisor JSON stay at `apiVersion: 1` with an additive
  `grok` cell. No capability change is required.
- Grok context reporting is always a labeled filesystem estimate (1% of an
  assumed 200,000-token context). No provider diagnostic is launched for Grok.
- `state.json` stays at manifest version 2; no migration is required.
- No config file support; `GROK_HOME`, `[skills] paths`, project
  `.grok/skills/`, and Grok plugin skills are out of scope.
- Kimi support is a separate follow-up phase, not part of this file.

## PR0 Summary Table (branch `refactor/managed-tools-frontend`)

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P20-R1 | Views on MANAGED_TOOLS: SkillsView, SkillSetsView, Dashboard | done | - |
| P20-R2 | demoBackend/fixtures-data on MANAGED_TOOLS; shared api.ts helpers | done | P20-R1 |
| P20-R3 | Validation (typecheck, tests, build, go tests) and PR | done | P20-R2 |

## PR1 Summary Table (Grok, after PR0 merge)

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P20-T01 | Domain core: ToolGrok, paths, scan rows/groups | done | PR0 merge |
| P20-T02 | CLI tables, parsing, list JSON, advisor search | done | P20-T01 |
| P20-T03 | Install/update/uninstall planner and protected paths | done | P20-T01 |
| P20-T04 | TUI fourth column, details, and bulk toggles | done | P20-T01 |
| P20-T05 | Context budget Grok estimate and advisor parity | done | P20-T01 |
| P20-T06 | GUI backend projections and desktop frontend | done | P20-T01 |
| P20-T07 | First-party skill, README, DESIGN, AGENTS, wiki | done | P20-T02 |
| P20-T08 | Full validation, branch, and PR | in-progress | P20-T07 |

## PR0 Task Definitions

### P20-R1: Views

- `SkillsView`: `SkillToolScope` from `ManagedTool`, scope options, header,
  cells, details, `canSaveToSet`, `toolsForScope`/`cellForTool`/`rowSources`
  from `MANAGED_TOOLS`.
- `SkillSetsView`: `ToolChoice` from `ManagedTool`, dialog choices, table
  headers/cells, member states, editor cells, `toolsForChoice`,
  `isToggleableRow`, derived `colSpan`.
- `Dashboard`: per-tool counts/cards/bars/context rows from `MANAGED_TOOLS`;
  `TOOL_TONES`/`TOOL_ICON` records keyed by `ManagedTool` (compiler-checked).

### P20-R2: Demo Backend And Shared Helpers

- `api.ts`: additive `toolFullName`, `projectPending` and `favoriteEligible`
  iterate `MANAGED_TOOLS`.
- `demoBackend.ts`: seed `row()` takes a states record, toggle/group/visible
  scopes, snapshot/groups/stats, uninstall link counts, skill-set
  projections, candidates, and budget specs from `MANAGED_TOOLS`.
- No fixture shape changes; no visual or behavioral changes.

### P20-R3: Validation And PR

- `npm run typecheck`, `npm test`, `npm run build`, root `go test ./...`,
  `go vet ./...`, desktop `go test ./...` — all green, no behavior change.
- Separate branch and PR before PR1 starts.
