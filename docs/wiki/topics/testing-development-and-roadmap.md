# Testing, Development, And Roadmap

## Development Loop

```bash
go test ./...
go run . help
go run .
make build
make dev
make gui-test
make gui-build
make release-package RELEASE_VERSION=0.4.1
```

- `go run .` is suitable for repository-local checks.
- `make dev` rebuilds the persistent command at
  `~/.local/bin/skill-manager`; run it after user-visible code changes.
- Documentation-only changes do not require rebuilding the binary.
- Release verification and publication remain local/manual. Public CI is added
  as a non-publishing source-validation layer.
- Release packaging requires a clean Apple Silicon macOS checkout and leaves
  only ignored archives and checksums under `dist/release/`.

## Test Strategy

- Inject `paths.Paths` derived from a temporary home.
- Use temporary filesystem trees and local/fake Git runners.
- Never point automated tests at the real `~/.claude`, `~/.agents`,
  `~/.codex`, or `~/.skill-manager` directories.
- Backend suites cover scanning, metadata, paths, state, toggle planning/apply,
  conflicts, install URL/checkout/discovery/preflight/apply, repository
  reference auditing, fast-forward update, transactional uninstall, rollback,
  local path resolution/discovery/ownership, source-preserving local uninstall,
  CLI output, and strict dry-run behavior.
- Catalog suites use test HTTP servers and temporary cache paths to cover
  normalization, unsafe/oversized response rejection, freshness, offline
  fallback, local search, parsed details, and the no-raw-files cache boundary.
- TUI suites exercise the Bubble Tea model/update behavior; full terminal
  rendering snapshots are optional.
- Desktop backend suites use temporary homes and exercise snapshots, read-only
  projection, pending group actions, context-budget projection, apply, and
  preflight failure retention. Source-lifecycle coverage includes manifest-only
  projections, opaque IDs, exact-cell local install/review/apply, typed local
  uninstall with source preservation, and pending-operation exclusion; shared
  install/update/uninstall packages retain fake/local Git safety coverage.
- Discover backend coverage projects ON/OFF/conflict state and verifies that a
  selected catalog entry installs exactly one skill into only the selected
  agent cells while rejecting unknown, well-known, offline, pending, and busy
  requests.
- Context-budget suites use temporary homes and fake provider diagnostics to
  cover fallback discovery, Claude visibility/budget rules, Codex
  shortening/omission measurement, token math, pending deltas, and subprocess
  home isolation, argument allowlisting, secret/proxy omission, and the
  no-subprocess default estimate.
- Vitest and Testing Library cover dashboard loading, local filtering, staging
  versus explicit Apply, read-only scan opt-in, effective-state projection,
  context accuracy labels, `After Apply` rendering, managed source actions,
  exact install matrix review, filter-independent column selection, mixed and
  unavailable column states, busy-state blocking, typed uninstall confirmation,
  explicit diagnostics, and absence of public Discover navigation. Dormant
  catalog adapter/domain behavior remains covered by Go tests. Skills coverage
  additionally checks active-first placement,
  collapsed source groups, session navigation memory, search expansion,
  scoped tool/result/group actions, pending placement, and conflict exposure.
- Wails production packaging is checked as a local ad-hoc-signed
  `darwin/arm64` `.app`; native render checks cover both 1440×960 initial and
  1024×720 minimum sizes.

## Verification Expectations

- Run focused package tests while iterating.
- Run `go test ./...` before considering implementation work complete.
- For user-visible behavior changes, update `README.md` and run `make dev` after
  tests.
- If scope, semantics, paths, or UX change, update `AGENTS.md` and the relevant
  planning status table in the same change.
- If verification is unavailable, record the exact limitation instead of
  implying success.

## Completed Iterations

- Phase 1: scanning, state, reversible toggles, CLI, initial TUI, conflicts,
  dry-run, tests, and README.
- Phase 2: group model/detection/summaries, Group table/filter, smart group and
  all-visible pending toggles, messages, `groups`, and tests.
- Phase 3: repository identity/checkouts, manifest records, discovery,
  preflight, symlink apply/rollback, `install`, `repos`, strict dry-run, and
  tests.
- Phase 4: shared repository reference audit, clean fast-forward-only update,
  installed-path target preflight, transactional whole-repository uninstall,
  strict dry-runs, CLI integration, documentation, and tests.
- Phase 5: canonical local sources, direct/recursive discovery, link-in-place
  install, manifest ownership/classification, missing-source cleanup,
  source-preserving transactional uninstall, CLI integration, and tests.
- Phase 6: implementation-derived design contract, shared staging engine,
  concurrency-safe GUI service, nested Wails module, generated bindings,
  Dashboard/Skills React UI, pending review/apply, local ARM64 packaging, and
  desktop/frontend tests.
- Phase 7: global skill-catalog context measurement, Codex 2% and Claude
  configured/default budget reporting, labeled fallbacks/partial coverage,
  pending `After Apply` projection, Dashboard visualization, and tests.
- Phase 8: exact-cell install planning, manifest-only Sources projection,
  opaque lifecycle IDs, native local picker, inspect/review/apply install,
  per-repository and deterministic all-repository update, typed whole-source
  uninstall, exclusive phase progress, responsive UI, and tests.
- Phase 9: isolated anonymous skills.sh adapter, normalized/offline cache,
  rankings/search/details, live local-state projection, safety-confirmed exact
  GitHub skill/tool install, responsive Discover UI, and tests.
- Phase 10: active-first Skills hierarchy, collapsed source packs, compact
  state/tool filters, scoped backend batch actions, session accordion state,
  responsive accessibility, and tests.
- Phase 11: public namespace and bundle identifier, MIT/public policy files,
  clean-clone build order, synthetic screenshots, documentation anonymization,
  release verification, and a single-root public history.
- Phase 12: CLI version reporting, locally verified Apple Silicon desktop/CLI
  archives, SHA-256 manifest, download/Gatekeeper documentation, and manual
  private GitHub prerelease publication.
- Phase 13: public-preview state/cache permission repair, bounded backup
  retention, query-free catalog cache migration, manual provider diagnostics,
  hidden Discover binding/UI, dependency notices, and public `v0.4.1`
  repository/release hardening.

All Phase 13 tasks are complete after clean-commit packaging, successful public
CI, repository security configuration, and `v0.4.1` prerelease publication.

## Explicitly Deferred Work

- `skill-manager install --branch`.
- `skill-manager install --force`.
- GitHub shorthand, submodules, sparse checkout, a local-source CLI summary
  command, and a TUI source-management screen.
- Installation of skills.sh well-known/non-GitHub sources, catalog publishing,
  ratings/comments, telemetry, and individual-skill uninstall from Discover.
- Plugin enable/disable and disabled-state disaster recovery without a
  manifest.
- Developer ID signing, notarization, DMG and universal macOS builds, SBOM and
  provenance, automated publishing, auto-update, and a light theme.

These are roadmap candidates, not implied commitments. A future iteration must
define their semantics and safety model before implementation.
