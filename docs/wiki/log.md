# Skill Manager Wiki Log

This public log summarizes reusable project decisions and implementation
milestones. Workstation-specific observations and private filesystem state are
intentionally excluded.

## [2026-08-13] decision | Simplified local release verification

- Removed the repository-specific release-check target and its documentation.
- Retained the standard root, desktop, frontend, vet, dependency-audit, and
  clean-export checks as individually runnable development commands.

## [2026-08-13] decision | Adopted the public agent-skill-manager slug

- Updated the public repository URL, Go module namespace, imports, security
  reporting link, install instructions, and publication checks to
  `github.com/dees91/agent-skill-manager`.
- Kept the product name, `skill-manager` binary, state paths, and macOS bundle
  identifier unchanged.

## [2026-08-13] verification | Verified the public source release

- Verified the one-commit tracked-file archive with pinned Gitleaks, root and
  desktop Go tests/vet, 23 frontend tests, TypeScript type checking, a clean
  production frontend build, zero npm audit findings, and CLI build/install.
- Verified the `darwin/arm64` app bundle, ad-hoc signature, public bundle
  identifier, version, isolated-home launch, synthetic screenshot dimensions,
  and screenshot privacy checks.

## [2026-08-13] implementation | Prepared the public source release

- Moved the Go modules to `github.com/dees91/agent-skill-manager` and the macOS bundle
  identifiers to `io.github.dees91.skillmanager`.
- Added the MIT license, security, privacy, and contribution policies.
- Made desktop verification build the frontend before compiling the embedded Go
  application and verified a clean tracked-file archive.
- Replaced external/local reference captures with repository-owned screenshots
  generated from synthetic demo data.

## [2026-08-13] decision | Defined public distribution boundaries

- Version `0.4.0` is published as source for Apple Silicon macOS.
- Developer ID signing, notarization, universal binaries, release automation,
  and CI remain deferred.
- Fixed provider paths remain the current product contract; tests and demos use
  isolated synthetic homes.

## [2026-08-13] implementation | Delivered the active-first Skills workspace

- Skills are partitioned into attention, active, available-by-source, and
  explicitly opted-in read-only sections without changing Apply semantics.
- Search, state/tool scopes, accordions, and bulk staging preserve stable
  applied-state placement until a successful Apply and rescan.

## [2026-08-12] implementation | Added catalog discovery

- Added an isolated anonymous skills.sh adapter, normalized offline cache,
  rankings/search/details, local-state projection, and a safety-confirmed
  selected-skill install flow.
- Network failures remain isolated from local skill management.

## [2026-08-12] implementation | Added desktop source management

- Added manifest-owned Git and local sources, exact-cell install review,
  per-source and update-all actions, typed uninstall confirmation, and exclusive
  progress handling.
- Frontend mutations use opaque IDs and never provide filesystem operations.

## [2026-08-11] implementation | Added context-budget diagnostics

- Dashboard estimates startup catalog pressure for Claude and Codex and shows
  explicit approximation, partial-coverage, fallback, and pending projections.
- Diagnostics are read-only and degrade without blocking the main scan.

## [2026-08-11] implementation | Added the macOS desktop app

- Added a nested Wails/React module that reuses the root Go domain services.
- Dashboard and Skills preserve the TUI pending, conflict, deterministic Apply,
  backup, and partial-failure semantics.

## [2026-08-11] implementation | Added local path lifecycle

- Local folders are linked in place using canonical source identity.
- Install reuses the repository planner; uninstall removes owned links and state
  while preserving the user-owned source directory.

## [2026-08-11] implementation | Added repository update and uninstall

- Git updates require clean, tracking, fast-forward-only checkouts and audited
  owned references.
- Whole-repository uninstall stages exact owned links and checkouts for rollback
  before committing state changes.

## [2026-08-11] implementation | Added Git repository installation

- Git sources use managed checkouts, recursive skill discovery, all-or-nothing
  preflight, symlink apply rollback, strict dry-run, and manifest ownership.

## [2026-08-10] implementation | Established reversible skill management

- Added filesystem scanning, source/group classification, reversible toggles,
  CLI/TUI surfaces, conflicts, pending batch Apply, backups, and temporary-home
  test coverage.
