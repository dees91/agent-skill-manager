# Testing, Development, And Roadmap

## Development Loop

```bash
go test ./...
make vulncheck
go run . help
go run .
make build
make dev
make gui-test
make gui-build
make release-package RELEASE_VERSION=0.5.0
```

- `go run .` is suitable for repository-local checks.
- `make dev` rebuilds the persistent command at
  `~/.local/bin/skill-manager`; run it after user-visible code changes.
- Documentation-only changes do not require rebuilding the binary.
- Release verification and publication remain local/manual. Public CI is added
  as a non-publishing source-validation layer.
- Release packaging requires a clean Apple Silicon macOS checkout and leaves
  only ignored archives and checksums under `dist/release/`.

### Repository Marketing Media

The tracked `.github/assets/demo.gif` is generated from the isolated Remotion
project under `video/`. `video/STORYBOARD.md` is the human-readable story and
review contract; `video/src/storyboard.ts` is the executable projection of its
timings, copy, and assets. The same project generates the tracked
`.github/assets/social-preview.png` as a 1280×640 static Remotion composition.
Update the human and executable story sources before changing either asset.

The master is a silent 1920×1080, 30 fps, 600-frame MP4 written to the ignored
`video/out/` directory. `video/scripts/render-gif.sh` derives the README asset,
first attempting 960×540 at 15 fps and reducing frame rate, palette, then
resolution only as needed to remain below 10 MiB. The committed UI captures
come from the frontend demo backend and use only synthetic identities, paths,
skills, repositories, and counts.

```bash
cd video
npm ci
npm run check
npm run render
npm run gif
npm run social-preview
npm run verify
```

The capture refresh procedure is documented in `video/README.md`; rendering
from committed assets needs only Node/npm and FFmpeg. The verifier enforces a
10 MiB infinite-loop contract for the GIF and a 1 MiB PNG contract for the
social preview. GitHub's repository-card image must still be uploaded manually
from the tracked PNG under repository settings.

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
- Skill Set suites cover version/shape validation, atomic owner-only storage,
  bounded independent backups, CRUD, corruption isolation, missing-member
  retention/reconnection, overlapping projections, scoped preview/staging,
  Pending preservation, and source-uninstall impact.
- Advisor suites use temporary homes to cover path-free JSON, argument/version
  and capability boundaries, deterministic field-weighted BM25F/fuzzy ranking,
  phrase/plural/typo behavior, stable ties, eligibility and result limits,
  legacy substring-list compatibility, baseline-ON preservation, receipt/lease
  sharing, concurrent activation locking, strict dry-run, exact cleanup, manual
  early disable, and symlink drift rejection. A repository contract test covers
  the public skill.
- Favorite suites use temporary homes to cover version/shape validation,
  owner-only atomic storage, bounded backups, managed/conflict eligibility,
  read-only rejection, corruption isolation, basename reconnection, Pending
  independence, source-busy exclusion, and uninstall retention.
- Desktop About tests cover build-metadata parsing, required fields, icon
  presence, exact version/description formatting, and the repository's embedded
  production metadata.
- Discover backend coverage projects ON/OFF/conflict state and verifies that a
  selected catalog entry installs exactly one skill into only the selected
  agent cells while rejecting unknown, well-known, offline, pending, and busy
  requests.
- Context-budget suites use temporary homes and fake provider diagnostics to
  cover fallback discovery, Claude visibility/budget rules, Codex
  shortening/omission measurement, the Muse labeled filesystem estimate, token
  math, pending deltas, and subprocess home isolation, argument allowlisting,
  secret/proxy omission, and the no-subprocess default estimate.
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
  Skill Set coverage checks dedicated navigation, explicit tool selection,
  preview/staging, creation from Pending, addition from skill details, and
  non-blocking uninstall warnings.
- Wails production packaging is checked as a local ad-hoc-signed
  `darwin/arm64` `.app`; native render checks cover both 1440×960 initial and
  1024×720 minimum sizes.

## Verification Expectations

- Run focused package tests while iterating.
- Run `go test ./...` before considering implementation work complete.
- Run pinned `make vulncheck` across both Go modules before a release.
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
- Phase 14: independent saved-recipe persistence, basename-based Skill Sets,
  path-free CRUD and scoped smart-toggle preview/staging, contextual desktop
  creation, missing-member reconnection, uninstall impact warnings,
  responsive/accessibility validation, and local builds.
- Phase 15: public first-party Skill Advisor, versioned path-free inventory,
  opaque receipt/reference-counted lease persistence, cross-process locking,
  drift-safe agent-owned normal-exit cleanup, unknown-receipt isolation, and
  same-turn instruction loading.
- Phase 16: private basename-based favorites, path-free desktop mutation,
  favorite-first active workspace filtering, corruption isolation, missing-name
  reconnection, uninstall impact, and responsive/accessibility coverage.
- Phase 17: native macOS About version visibility backed by validated embedded
  Wails build metadata and the existing application icon.
- Phase 18: additive deterministic local `advisor search` ranking with weighted
  BM25F, phrase bonuses, bounded fuzzy matching, path-free capability-gated
  results, legacy list compatibility, and first-party skill migration.
- Phase 19: Muse as a third managed tool across CLI, TUI, install/update/
  uninstall, Skill Advisor, labeled filesystem context estimate, macOS GUI,
  first-party skill, and documentation with manifest version 2 unchanged.

All Phase 13 tasks are complete after clean-commit packaging, successful public
CI, repository security configuration, and `v0.4.1` prerelease publication.
The `v0.4.2` maintenance prerelease superseded that first preview with patched
Go, Wails, terminal UI, and frontend dependencies plus the empty-profile GUI
projection fix. The published `v0.5.0` feature prerelease supersedes it with
the completed Phase 14 through 16 work: Skill Sets, the first-party Skill
Advisor, and favorites.

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
