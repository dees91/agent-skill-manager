# Skill Manager Wiki Log

This public log summarizes reusable project decisions and implementation
milestones. Workstation-specific observations and private filesystem state are
intentionally excluded.

## [2026-08-17] verification | Validated saved Skill Sets

- Passed root and desktop Go tests/vet, 21 frontend tests, production frontend
  build, pinned vulnerability scans, generated bindings, local CLI install, and
  an ad-hoc-signed Apple Silicon app build.
- Verified synthetic 1440×960 and 1024×720 GUI states, scoped preview/staging,
  contextual creation, console health, and WCAG A/AA automation with no
  violations.
- Wiki lint found every topic/source indexed and no broken relative links,
  stale Phase 14 planning status, secrets, or workstation inventory.

## [2026-08-17] implementation | Added saved Skill Sets

- Added GUI-only, tool-agnostic task recipes with optional `When to use` notes,
  explicit Claude/Codex/Both smart-toggle previews, and shared Pending/Apply
  semantics.
- Added an independent owner-only `skill-sets.json` store with atomic writes,
  bounded backups, corruption isolation, missing-member reconnection, and
  non-blocking source-uninstall impact warnings.
- Added Pending and skill-details creation shortcuts, synthetic demo/test data,
  a repository-owned screenshot, backend/frontend coverage, responsive browser
  QA, and WCAG A/AA validation.

## [2026-08-14] documentation | Improved repository discovery assets

- Tightened the README value proposition, added direct download, source
  installation, and compatibility links, and summarized the local reversible
  safety model without adding star prompts.
- Added a reproducible 1280×640 GitHub social preview composition to the
  existing Remotion project, with synthetic data and automated format, size,
  and media verification.

## [2026-08-14] publication | Published the v0.4.2 maintenance preview

- Published verified Apple Silicon desktop and CLI artifacts plus checksums;
  `v0.4.2` supersedes `v0.4.1` with a patched Go 1.26.6 baseline and current
  Wails, Bubble Tea, Lip Gloss, React, TypeScript, and Vite dependencies.
- Included the empty-profile GUI projection fix and regenerated binary
  dependency notices without expanding the public Dashboard, Skills, and
  Sources product surface.

## [2026-08-14] maintenance | Updated the frontend toolchain

- Upgraded React to 19.2.8, TypeScript to 7.0.2, Vite to 8.2.1, and the related
  testing and React plugin packages; documented the Node.js 22.12/npm 10 floor.
- Made empty managed-source snapshots serialize as arrays and added a backend
  contract test after live browser QA exposed the prior null projection.
- Revalidated Dashboard, Skills staging, Sources dialogs, console health, and
  WCAG A/AA automation with no accessibility violations.

## [2026-08-14] maintenance | Updated terminal UI dependencies

- Upgraded Bubble Tea from 0.27.1 to 1.3.10 and Lip Gloss from 0.13.0 to 1.1.0.
- Regenerated dependency notices and verified the TUI test suite plus an
  isolated interactive PTY launch and clean quit.

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

## [2026-08-14] query | Clarified Skills CLI installation boundaries

- Verified the upstream global canonical-copy, agent-link, lock metadata, npm
  execution-cache, dependency, and default install-telemetry behavior relevant
  to Skill Manager interoperability.

## [2026-08-14] implementation | Added a reproducible README demo animation

- Added a versioned human and executable storyboard, a self-contained Remotion
  project, synthetic desktop captures, and an FFmpeg size-bounded GIF pipeline.
- Embedded the 20-second local/reversible workflow demo in the README while
  retaining the static Dashboard documentation image.

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
## [2026-08-17] implementation | Added the first-party Skill Advisor

- Added a public standard skill, versioned path-free inventory, and
  receipt-scoped Claude/Codex activation with reference-counted leases.
- Kept cleanup explicit and drift-safe, isolated advisor metadata from the
  core manifest, and documented public Git/local installation.

## [2026-08-17] verification | Verified Skill Advisor activation and cleanup

- Passed focused, race, full backend/frontend, vet, dependency notice,
  vulnerability, skill contract, and strict wiki checks.
- Verified the public skill through a fresh Codex process and exercised live
  Claude/Codex receipt activation and exact cleanup with no receipts left.

## [2026-08-17] implementation | Add managed-skill favorites

- Added isolated basename persistence, path-free desktop mutation, active-first
  favorite filtering and ordering, source-uninstall impact, tests, and public
  documentation for Phase 16.

## [2026-08-17] verification | Verify and install skill favorites

- Passed focused, race, full backend/frontend, vet, vulnerability, dependency
  notice, strict wiki-link, privacy, accessibility, and media-output checks.
- Rebuilt the persistent CLI and ad-hoc-signed Apple Silicon desktop app,
  verified its bundle, installed it in `/Applications`, and launch-checked it.

## [2026-08-17] maintenance | Refresh the Dashboard visual evidence

- Replaced the stale documentation and video Dashboard captures with the
  current synthetic demo inventory and navigation, including Skill Sets.
- Advanced the demo storyboard revision and regenerated its README animation.

## [2026-08-17] release | Prepare v0.5.0 public preview

- Selected a minor prerelease for the completed Skill Sets, Skill Advisor, and
  favorites features while retaining the Apple Silicon preview limitations.
- Updated source version metadata, download guidance, release notes, and the
  current wiki synthesis for local packaging and GitHub publication.
- Reviewed and pinned the only npm dependency install-script approvals used by
  the desktop and repository-owned media build.

## [2026-08-17] implementation | Serialize advisor state-tree security

- Fixed a Linux CI race where concurrent activations could walk the disabled
  tree before acquiring `advisor.lock` while another activation moved a skill.
- Kept no-follow state-root bootstrapping before lock acquisition and moved the
  recursive permission pass behind the lock, with deterministic ordering and
  repeated concurrency coverage.
