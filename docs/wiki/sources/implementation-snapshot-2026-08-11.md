# Implementation Snapshot - 2026-08-11

## Provenance

- `observed`: direct inspection of the current Go source, tests, README, and
  planning summary tables on 2026-08-11.
- `documented`: this snapshot records package responsibilities and test
  boundaries, not details of the workstation used during development.

## Package Snapshot

- [`main.go`](../../../main.go) resolves default paths and delegates to the CLI.
- [`internal/cli`](../../../internal/cli/cli.go) owns command parsing, output,
  install orchestration, and the TUI entrypoint.
- [`internal/model`](../../../internal/model/model.go) defines tools, states,
  sources, groups, entry types, rows, conflicts, and planned operations.
- [`internal/paths`](../../../internal/paths/paths.go) derives the fixed MVP
  filesystem layout from a provided or current home directory.
- [`internal/metadata`](../../../internal/metadata/metadata.go) tolerantly reads
  `SKILL.md` frontmatter and Skills CLI lockfile names.
- [`internal/scan`](../../../internal/scan/scan.go) discovers managed, disabled,
  and read-only skills, attaches metadata, and merges tool cells into rows.
- [`internal/ops`](../../../internal/ops/ops.go) plans and applies reversible
  enable/disable moves.
- [`internal/state`](../../../internal/state/store.go) owns `state.json`,
  disabled-entry, repository, and local-source records, normalization, backups,
  migration to manifest version 2, and atomic replacement.
- [`internal/install`](../../../internal/install/) owns repository URL and local
  path identity, checkout reuse/clone, recursive and direct-root discovery,
  preflight, symlink apply, reference audits, update/uninstall, rollback, and
  manifest updates.
- [`internal/tui`](../../../internal/tui/model.go) owns the Bubble Tea model,
  filters, pending changes, smart batch toggles, rescan, and rendering.

## Current Capability Snapshot

- `implemented`: Phases 1-5 described by the planning files have corresponding
  implementation and test files in the repository.
- `implemented`: CLI commands include `tui`, `list`, `status`, `groups`,
  `repos`, `install`, `update`, `uninstall`, `enable`, and `disable`.
- `implemented`: the TUI has per-tool pending toggles, group/all-visible smart
  toggles, text/source/group filters, optional read-only rows, details, and
  deterministic apply.
- `implemented`: install supports HTTPS and SCP-style SSH Git URLs, recursive
  discovery, repeated skill selection, both/Claude/Codex targets, strict
  dry-run, preflight, idempotency, and rollback of links created by a failed
  symlink apply.
- `implemented`: local install supports canonical path identity, root or
  recursive discovery, link-in-place ownership and classification, and
  source-preserving uninstall even after the source disappears.

## Test Surface

Focused test files exist beside every main package. The larger suites are in
`internal/cli`, `internal/tui`, `internal/scan`, `internal/install`,
`internal/ops`, and `internal/state`. Tests use path injection and temporary
homes or local fixtures, matching the repository's safety contract.
