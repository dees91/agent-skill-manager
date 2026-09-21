# Phase 25: Duplicate Skill-Name Resolution - Task Breakdown

> **Source**: user-approved plan (2026-09-21) for the install-blocking
> `duplicate skill names discovered` class: multi-harness fan-out copies,
> generated/test copies, and genuine same-name collisions inside one source.
> Cross-source same-name collisions (one skill name owned by two recorded
> sources) are a separate class and stay blocked by the ownership model.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Discovery groups skills by basename instead of failing. Identical copies
  (bounded content hash) resolve to one canonical copy; the ranking prefers
  real distribution paths over test/example copies, visible over hidden
  directories, shallower over deeper paths, then lexicographic order.
- Differing copies are ambiguous and never guessed. CLI selection is
  `--skill <name>[=<path>]`; the desktop install matrix shows one row with a
  copy picker and `needs-choice` cells until a copy is chosen.
- Resolution reuses the same source's manifest-recorded path when it still
  holds the skill; explicit choices win; a recorded path that no longer
  holds the skill is drift (uninstall plus reinstall), never a silent switch.
- Update reports unrecorded identical groups as ordinary new skills and
  unrecorded conflicting groups as ambiguous new skills with a qualified
  template. Recorded names are never new. Extend carries recorded paths as
  exact-cell choices. Strict dry-run, ownership, backup, and rollback
  semantics are unchanged.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P25-T01 | Grouped discovery, fingerprints, canonical ranking | done | - |
| P25-T02 | Shared resolver, planner selection, recorded-path reuse | done | P25-T01 |
| P25-T03 | CLI qualified `--skill`, ambiguity output, new-skills report | done | P25-T02 |
| P25-T04 | GUI draft candidates, review/apply choices, health items | done | P25-T02 |
| P25-T05 | Desktop copy picker, bindings, styles | done | P25-T04 |
| P25-T06 | Update/extend/discover consistency | done | P25-T02 |
| P25-T07 | AGENTS, CLAUDE, usage, DESIGN, and wiki documentation | done | P25-T03 |

## Task Definitions

### P25-T01: Grouped discovery, fingerprints, canonical ranking

- `Discovery` with unique skills plus ranked `DuplicateGroup`s in
  `internal/install/discovery.go`; discovery errors only on I/O problems.
- Bounded content fingerprint in `internal/install/fingerprint.go` (file
  names, entry kinds, bytes; symlinks by target text, never followed;
  oversized skills treated as differing).
- Canonical ranking: fixture segments last, hidden after visible, deeper
  after shallower, lexicographic ties.
- Verification: `go test ./internal/install/ -run
  'TestDiscover|TestFingerprint|TestOversized'`.

### P25-T02: Shared resolver, planner selection, recorded-path reuse

- `ResolveDiscovery`, `AmbiguousSkillsError`, `SplitSkillRequests`, and
  `CellsToRequests` in `internal/install/resolver.go`; qualified `name=path`
  parsing with a literal-`=` fallback; recorded-path seeds with drift errors.
- `PlanInstall`/`PlanLocalInstall` select through the resolver, carry
  `InstallCell.Path`, and report `ResolvedDuplicates`.
- Verification: `go test ./internal/install/ -run
  'TestPlanInstall|TestParse|TestResolve|TestSplit|TestCells|TestRecorded'`.

### P25-T03: CLI qualified `--skill`, ambiguity output, new-skills report

- Usage `[--skill name[=path]...]`, `resolved ...` notes, full
  `printAmbiguousSkills` listings, and the ambiguous section of
  `reportNewSkills` in `internal/cli/cli.go`.
- Verification: `go test ./internal/cli/`, including the local-install
  duplicates end-to-end test.

### P25-T04: GUI draft candidates, review/apply choices, health items

- `InstallCandidate` options/`needsChoice`/`identicalCopies`,
  `InstallCellRequest.path`, draft projection, review/apply choice
  validation, and merged ambiguous new-skill names in update results and
  source health.
- Verification: `go test ./internal/gui/`, including the duplicate-draft
  review/apply test.

### P25-T05: Desktop copy picker, bindings, styles

- Regenerated Wails bindings, per-row copy `<select>`, `needs-choice`
  gating of checkboxes, Review guard, bulk-toggle exclusion, matrix styles.
- Verification: `npm run typecheck`, `npm test`, and `npm run build` in
  `desktop/frontend`.

### P25-T06: Update/extend/discover consistency

- `NewSkillsReport` with ambiguous groups, `UpdateResult.AmbiguousNewSkills`,
  recorded-path exact cells in `extendCells`, and unchanged dormant Discover
  behavior through the new planner.
- Verification: `go test ./internal/install/ -run
  'TestNewSkills|TestUpdate|TestExtend'`.

### P25-T07: AGENTS, CLAUDE, usage, DESIGN, and wiki documentation

- Iteration 25 contract in `AGENTS.md`, the Phase 25 route in `CLAUDE.md`,
  qualified `--skill` and ambiguous new-skills in `docs/usage.md`, the matrix
  picker in `DESIGN.md`, and updated wiki topics plus a log entry.
- Verification: no document describes duplicate basenames as a discovery
  failure; every example uses synthetic sources.
