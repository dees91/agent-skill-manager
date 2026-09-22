# Phase 26: Cross-Source Skill Alias - Task Breakdown

> **Source**: user-approved plan (2026-09-21) for the class Iteration 25 left
> blocked: two recorded sources (two Git repositories, or a repository and a
> local path) that each ship a skill with the same basename. The target
> `<tool skills dir>/<name>` is one slot, so the ownership model blocks the
> second install. Iteration 25 behavior inside one source does not change.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Mechanism: an install name (alias). The second source links its skill under
  a different link basename. Links stay symlinks; `SKILL.md` is never edited
  or copied.
- Policy: explicit, with a suggestion. A cross-source collision still blocks.
  The CLI prints a pasteable `--as '<name>=<install-name>'`; the desktop
  matrix shows a `needs-name` row with the suggestion prefilled. Nothing
  installs under another name without a user decision.
- Suggestion scheme `<name>-<owner>`: Git uses the repository owner segment
  (`alpha-acme`), a local path uses the source root basename
  (`alpha-sample-pack`); if taken, `<name>-<owner>-<repo>`, then a numeric
  suffix. Segments follow the Agent Skills name rule (1-64 characters,
  `[a-z0-9-]`, no leading, trailing, or double hyphen).
- One install name per (source, skill) across all tools; no per-tool aliases.
- The recorded install name wins. Reinstall and extend reuse it; a different
  `--as` for a recorded skill is drift (uninstall plus reinstall).
- Manifest version 3 adds `installedAs,omitempty` to installed skill entries.
  Older binaries refuse version 3; state backups cover rollback.

Rejected alternatives: sharing one cell between sources (breaks ownership and
toggle semantics), automatic aliasing (silent command renames in Claude Code),
and copying or editing `SKILL.md` (violates hard invariants).

## Host Behavior (verified 2026-09-21)

| Host | Directory vs frontmatter `name` | Same `name` twice |
| --- | --- | --- |
| Claude Code | Command comes from the directory; `name` is a display label. | Scope precedence; within a scope not addressed. |
| Codex | `name` is required and used for invocation. | Not merged; both can appear in selectors. |
| Grok | Identifier is frontmatter `name`, directory if omitted. | Not addressed. |
| Agent Skills spec | `name` must match the parent directory name. | Not addressed. |
| Muse | No vendor documentation found. | Unverified; treated like Codex. |

An alias gives Claude Code a distinct `/<install-name>` command. In Codex and
Grok both installs load but can share one label, because the frontmatter
`name` is not edited. This known limitation is documented and noted in the
desktop matrix.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P26-T01 | Manifest v3, `installedAs`, `ValidInstalledName` | done | - |
| P26-T02 | Install core: alias resolution, suggestion, planner, apply, ownership | done | P26-T01 |
| P26-T03 | Audits, uninstall, and extend by install name | done | P26-T02 |
| P26-T04 | Scan and ops classification of aliased local links | done | P26-T01 |
| P26-T05 | CLI `--as`, suggestion output, alias labels, new-skill report | done | P26-T02 |
| P26-T06 | GUI backend: needs-name drafts, review/apply validation, impact maps | done | P26-T02 |
| P26-T07 | Desktop install-name input, bindings, Skills label, styles | done | P26-T06 |
| P26-T08 | AGENTS, CLAUDE, usage, DESIGN, and wiki documentation | done | P26-T05 |

## Task Definitions

### P26-T01: Manifest v3, `installedAs`, `ValidInstalledName`

- `internal/state/store.go`: `manifestVersion = 3`, versions 0-2 migrate in
  memory, `InstalledSkillEntry.InstalledAs` with `InstalledName()`,
  `ValidInstalledName`, and `installedAs == name` collapsed on normalize.
- Verification: `go test ./internal/state/...`.

### P26-T02: Install core

- `internal/install/alias.go`: `SanitizeNameSegment`, `SourceRef`,
  `SuggestInstalledName`, recorded install-name lookups, and
  `resolveInstalledNames` (recorded wins; drift, invalid, discovered-name,
  duplicate, and unselected errors).
- `PlanOptions.InstalledAs`, `InstallCell.InstalledAs`, and
  `LinkPlan`/`AlreadyInstalled` `InstalledAs` with `InstalledName()`; every
  link, disabled path, and apply revalidation uses the install name.
- Ownership returns a `cellOwner` keyed by install name;
  `PreflightConflict.SuggestedAs` and `OwnerGroup`; recorded plain names get
  uninstall-plus-reinstall guidance instead of a suggestion.
- Verification: `go test ./internal/install/ -run
  'TestSanitize|TestSuggest|TestResolveInstalled|TestPlanInstall'`.

### P26-T03: Audits, uninstall, and extend by install name

- `RepositoryReference.InstalledName`; Git and local audits derive paths from
  the install name and reject two entries of one source sharing one.
- Uninstall staging and disabled-record removal use the install name.
- `extendCells` carries `InstalledAs`; the claim map, `extendEndsOn`, and
  `DisableAfter` use install names. Update, new-skill discovery, and repair
  stay on source names.
- Verification: `go test ./internal/install/...`.

### P26-T04: Scan and ops

- `newScanContextWithManifest` keys local cells by install name.
- Verification: `go test ./internal/scan/... ./internal/ops/...`, including
  an aliased link beside an unrelated plain link and a disable/enable round
  trip that keeps the local group.

### P26-T05: CLI

- Repeatable `--as <name>=<install-name>` parsing, suggestion output with a
  shell-quoted pasteable form (omitted for unprintable values),
  `tool/<install-name> (<name>)` labels, `installed "<name>" as
  "<install-name>"`, a suggested `--as` in the update new-skill command, and
  help text.
- Verification: `go test ./internal/cli/`.

### P26-T06: GUI backend

- `InstallCellRequest.InstalledAs`; `InstallCandidate.InstalledAs`,
  `NeedsName`, `SuggestedAs`; `InstallConflict.SuggestedAs`; path-free
  `needs-name` cells naming the owner group; `validateBridgeInstalledNames`
  on review and apply; favorites, Skill Set, and Discover lookups by install
  name.
- Verification: `go test ./internal/gui/`, `make gui-bindings`, and
  `go test ./... && go vet ./...` in `desktop`.

### P26-T07: Desktop

- Install-name input per needs-name row (prefilled, validated, never
  preselected, skipped by column toggles, blocks Review while invalid), the
  host note, recorded `installed as` label, Skills view `SKILL.md` name label
  and search, demo candidate, and styles.
- Verification: `make gui-test`.

### P26-T08: Documentation

- Iteration 26 contract in `AGENTS.md`, the Phase 26 route in `CLAUDE.md`,
  `--as` in `docs/usage.md`, the matrix input in `DESIGN.md`, and wiki topics
  plus a log entry.
- Verification: wiki relative links resolve, `git diff --check`, and every
  example uses synthetic sources.

## Non-Goals

- An install-name offer in Discover.
- Per-tool aliases.
- Advisor indexing of frontmatter names.
- Editing `SKILL.md` frontmatter to make Codex or Grok labels distinct.
