# Skill Manager Wiki Log

This public log summarizes reusable project decisions and implementation
milestones. Workstation-specific observations and private filesystem state are
intentionally excluded.

## [2026-08-14] maintenance | Updated the desktop runtime

- Upgraded the Wails library and pinned build CLI from 2.13.0 to 2.14.0.
- Raised both Go module requirements from 1.25.12 to 1.26.6 after new standard
  library advisories were published, including one without a 1.25 backport.
- Regenerated third-party notices and revalidated the native Apple Silicon app.

## [2026-08-13] publication | Published the v0.4.1 public preview

- Replaced the private `v0.4.0` prerelease/tag with verified Apple Silicon
  `v0.4.1` desktop and CLI artifacts plus checksums and dependency notices.
- Made the repository public, added passing pinned-action CI and Dependabot,
  enabled private vulnerability reporting and GitHub security scanning, and
  protected `main` from deletion and non-fast-forward pushes.

## [2026-08-13] security | Cleared initial dependency alerts

- Upgraded desktop `golang.org/x/crypto`, `x/net`, and `x/sys` after Dependabot
  identified patched 2026 advisories before the release tag was created.
- Raised both module requirements to Go 1.25.12 so their standard libraries and
  the root CLI's transitive `x/sys` dependency contain the available security
  fixes.
- Added pinned `govulncheck` gates to local release verification and public CI.

## [2026-08-13] implementation | Hardened the public preview boundary

- Version `0.4.1` uses owner-only state/cache metadata, bounded state backups,
  query-free catalog caching, and a legacy-cache privacy migration.
- Default scans are filesystem-only; provider diagnostics require an explicit
  action, fixed arguments, and a minimal environment without secrets/proxies.
- The public desktop binding and navigation exclude experimental Discover.

## [2026-08-13] implementation | Added binary dependency notices

- Added a reproducible notice generator for the Go runtime, exact linked Go
  modules, and locked production frontend packages.
- Release packaging now embeds the project license and notices in the desktop
  bundle and includes notices in the CLI archive.

## [2026-08-13] decision | Prepared v0.4.1 as the first public preview

- The private `v0.4.0` prerelease is superseded by public preview `v0.4.1`.
- Signing/notarization, universal builds, SBOM/provenance, auto-update,
  automation, and commercialization review remain pre-stable follow-ups.

## [2026-08-13] decision | Approved private GitHub preview binaries

- Version `0.4.0` gains locally built Apple Silicon desktop and CLI archives,
  one SHA-256 manifest, and repository-owned release notes.
- Publication remains a manual `gh` workflow; signing is ad-hoc and CI,
  notarization, DMG, universal binaries, and auto-update remain deferred.

## [2026-08-13] implementation | Added local release packaging

- Added CLI version reporting and a clean-checkout packaging workflow that
  validates source, frontend, desktop, extracted archives, signatures,
  architecture, metadata, isolated-home launch, and checksums.
- Documented downloads, integrity checks, Gatekeeper approval, maintainer
  publication steps, and the Phase 12 distribution contract.

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
