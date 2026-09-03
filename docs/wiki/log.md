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

## [2026-08-17] verification | Publish v0.5.0 public preview

- Tagged the exact green candidate commit after the full local Apple Silicon
  packaging gate and public Root Go plus Desktop/frontend CI succeeded.
- Uploaded the desktop ZIP, CLI tarball, and SHA-256 manifest to a draft,
  downloaded them again, verified byte identity, signatures, architecture,
  version metadata, archive contents, checksums, CLI behavior, and an isolated
  desktop launch, then published the draft as a GitHub prerelease.

## [2026-08-17] implementation | Add native desktop About information

- Added `About Skill Manager` through Wails' standard macOS application menu
  with the existing icon, product name, current version, and short description.
- Kept `desktop/wails.json` as the shared About and bundle metadata source with
  no frontend binding, state, terminal-interface, release, or install changes.

## [2026-08-17] verification | Verify native desktop About information

- Passed focused and full desktop tests, root/backend and frontend suites, both
  Go vet runs, dependency notices, wiki links, and the ad-hoc ARM64 build.
- Launch-checked the bundle with an isolated home and inspected the live native
  menu target to confirm the item label, exact version/description, and icon.

## [2026-08-22] maintenance | Rewrite the public README for clarity

- Reworked the user-facing prose and navigation while preserving the documented
  commands, paths, product behavior, safety rules, and link targets.

## [2026-08-25] implementation | Report already-active advisor selections

- Updated the first-party Skill Advisor to report relevant ON and OFF skills
  separately, retain a five-skill total bound, activate through the existing
  receipt API, and load instructions for every selected skill.

## [2026-08-25] decision | Make advisor cleanup workflow-owned

- Replaced user-delegated task-end cleanup with agent-owned cleanup of the
  exact receipt before every normal final response.
- Kept unknown receipts explicit-only and deferred expiry, bulk cleanup,
  provider hooks, and crash recovery.

## [2026-08-25] verification | Verify advisor-owned cleanup contract

- Passed skill validation, the focused and full Go suites, Go vet, strict wiki
  validation, and diff checks without changing the CLI or persisted receipt
  schema.

## [2026-08-26] implementation | Add ranked Skill Advisor search

- Added additive, capability-gated `advisor search` over the selected host's
  toggleable ON/OFF cells using deterministic local weighted BM25F, phrase
  bonuses, and bounded fuzzy token matching.
- Kept legacy `list --query` substring behavior unchanged, omitted paths,
  queries, scores, and reasons from search results, and migrated the first-party
  skill without an old-CLI fallback.

## [2026-08-26] verification | Verify ranked Skill Advisor search

- Passed focused, repeated deterministic, race, full root/desktop/frontend,
  vet, vulnerability, npm audit, notices, skill-contract, wiki-link, and diff
  checks, then rebuilt the persistent CLI with `make dev`.
- Live read-only smoke testing advertised `ranked_search_v1`, ranked the ADR and
  quality-review skills first for the target query, preserved legacy list
  ordering, completed in under half a second, and left `state.json` unchanged.

## [2026-09-03] implementation | Add Muse as a third managed tool

- Added `muse` as an independent managed tool alongside Claude Code and Codex:
  `~/.config/muse/skills` (`$XDG_CONFIG_HOME/muse/skills` when set) plus
  `~/.skill-manager/disabled/muse/`, with no shared ownership of
  `~/.agents/skills`.
- Extended domain scan/staging, CLI tables and `--tool` targets (`muse`,
  `both`, `all`), install/update/uninstall planning, TUI third column,
  advisor search/activate/status, and an always-estimated Muse context report
  at manifest version 2.
- Projected Muse cells through the GUI backend and desktop frontend (Skills
  column, install matrix, Dashboard, Skill Sets, bindings, fixtures) and
  taught the first-party `skill-advisor` skill the `muse` host and path.
- Updated README, DESIGN, AGENTS.md, planning/phase-19 tasks, and wiki
  synthesis; backend, desktop, and frontend suites pass on synthetic homes.

## [2026-09-03] fix | Keep ForHome hermetic for the Muse path

- `paths.ForHome` read ambient `XDG_CONFIG_HOME`, so the Muse directory
  escaped synthetic test homes and leaked state into the real
  `~/.config/muse/skills` on CI (green locally, red in the Root Go job).
- `ForHome` now always derives `MuseUserSkills` from the given home; only the
  production `paths.Default()` constructor applies the `XDG_CONFIG_HOME`
  override. Rule: path constructors used by tests stay pure functions of
  their arguments.

## [2026-09-03] fix | Review follow-up: desktop layout and Muse estimate contract

- Sized desktop grids for the third tool column (metric cards, skills table,
  install matrix, skill-set table/member/editor rows, set tool choice) and
  added the missing `.accuracy-estimated` badge style.
- Fixture and demo backends now report Muse context as `estimated` with the
  no-diagnostic message, matching the Go analyzer; the App test asserts one
  `Partial estimate` and one `Estimated` badge.
- Deduped `codexLine`/`museLine` into `catalogLine`, renamed the SkillSets
  `ToolChoice` value `both` to `all`, and cleared Claude-and-Codex leftovers
  in AGENTS.md, DESIGN.md, README.md, App copy, and wiki sources.
- Existing `docs/images/*.png` still show two tool columns; regenerating them
  requires running the desktop app and stays open.

## [2026-09-03] implementation | Extend managed sources to a tool

- Added `skill-manager extend --tool <tool> [--dry-run]` (P19-T09): generic
  `PlanExtend`/apply over `model.Tool` walks Git then local sources in
  manifest order, reuses install discovery/preflight plus a cross-source
  claim map, mirrors OFF state through the shared disable path, and stops at
  the first failure with an `extend --tool <tool> failed for source <group>`
  error. Nothing hardcodes `muse`.
- Added `PreviewExtend`/`ExtendSources` GUI methods and desktop bindings plus
  a Sources "Extend to tool" dialog (tool radio, per-source preview,
  stop-at-first-failure finish), with `docs/images/sources-extend.png`
  captured from synthetic demo data at 1440x960 and registered in DESIGN.md.
- Covered by domain, CLI, GUI, and frontend tests; documented in README,
  planning/phase-19-muse-support-tasks.md, and the repository/local-path
  install, interfaces, and desktop-gui topic pages.

## [2026-09-03] fix | Extend review follow-up: result-only GUI mutation

- Changed `ExtendSources` to the sibling result-only contract
  `(toolName, includeReadOnly) SourceMutationResult`: partial-failure results
  now reach the frontend with a fresh snapshot instead of being dropped by a
  rejected Wails promise; the previous resolved-with-failure frontend mock is
  now the real contract.
- Replaced the hardcoding `ExtendPreview.MuseCount` with generic
  `CreateCount`/`BlockedCount`; preview sources now surface status, reason,
  skipped skills, and plan conflicts, and the dialog confirm stays disabled
  while any source is blocked or no new links are planned.
- Fixed the GUI success message double punctuation, threaded the source kind
  through extend disable failures, single-passed the CLI summary totals, and
  aligned the dry-run blocked error with the
  `extend --tool <tool> failed for source <group>` format.
- Verified the `gitInitCheckout` fixture is required: the extend test reaches
  `AuditRepositoryReferences`/`UninstallService.Plan`, which shell out to
  `git rev-parse`; documented why in the test.
- Updated AGENTS.md (CLI extend contract, Sources extend action), the
  local-path install wiki topic, and this log.

## [2026-09-03] fix | Extend blocked preview reason fallback

- `projectExtendPreview` now falls back to `source.Err.Error()` when a blocked
  source carries no mappable plan conflicts, so claim-map collisions, local
  drift, moved sources, and repo identity errors show a reason instead of a
  bare `blocked` status. Covered by a claim-collision GUI test.
- The install matrix and its column header now use the shared
  `MANAGED_TOOLS`/`toolDisplayName` from `api.ts`, closing the remaining tool
  list duplication in `SourcesView`.

## [2026-09-03] implementation | PR0 frontend tool lists on MANAGED_TOOLS

- `SkillsView`, `SkillSetsView`, `Dashboard`, `demoBackend`, and the shared
  `api.ts` helpers (`projectPending`, `favoriteEligible`, additive
  `toolFullName`) now iterate `MANAGED_TOOLS` instead of hardcoded
  claude/codex/muse triples. `TOOL_TONES`/`TOOL_ICON`/`DEMO_BUDGET_SPECS`
  records are keyed by `ManagedTool`, so the next tool is compiler-guided.
- No visual or behavioral change: frontend typecheck, 28/28 vitest tests,
  production build, root `go test`/`go vet`, and desktop `go test` all green.
- Tracked in `planning/phase-20-grok-support-tasks.md` (PR0 done); Grok tool
  support follows as PR1 after the PR0 merge.

## [2026-09-03] implementation | Grok as fourth managed tool (phase 20)

- Grok user skills live in `~/.grok/skills` (per official xAI docs); disabled
  entries in `~/.skill-manager/disabled/grok/`. Fourth `ToolGrok` column across
  scan rows/groups, CLI tables/JSON, TUI, install/update/uninstall/extend
  planner, Skill Advisor inventory, and GUI projections. Empty/`both`/`all`
  now target all four tools; `list --json` stays at `apiVersion: 1` with an
  additive `grok` cell; `state.json` stays at manifest version 2.
- No provider diagnostic for Grok: the context budget report is always a
  labeled filesystem estimate (1% of an assumed 200,000-token context), like
  Muse. `GROK_HOME`, `[skills] paths`, project `.grok/skills`, and Grok plugin
  skills are out of scope.
- Frontend tool lists iterate `MANAGED_TOOLS` (PR0 groundwork); Grok adds one
  list entry, `Record<ManagedTool, ...>` display records, a purple metric tone,
  and four-column grid sizing. Fixed one missed PR0 site: install-draft
  initial selections in `SourcesView`.
- Tracked in `planning/phase-20-grok-support-tasks.md`; authoritative docs
  (`AGENTS.md`, `README.md`, `DESIGN.md`, first-party skill) and the wiki
  synthesis updated alongside.

## [2026-09-03] implementation | PR12 follow-up: TUI width, estimate spec, demo specs

- `fixedTableWidthWithoutSkill` now derives cursor/separator/state widths from
  `len(model.Tools())` (was off by one separator after Grok) with a new
  `view_test.go` locking the derivation.
- `analyzeMuse`/`analyzeGrok` bodies merged into `analyzeEstimatedTool` driven
  by per-tool spec records; a diagnostic-less tool is now one spec entry.
- Demo budget window/accuracy/label/message live in the shared exported
  `DEMO_BUDGET_SPECS`; fixtures consume the same record. Sources Targets cell
  renders all tools from `MANAGED_TOOLS` (Grok counter was missing) with a
  regression assertion.
- Deferred on purpose: `docs/images` screenshots still show three tools;
  refreshing them needs an interactive GUI capture session.

## [2026-09-03] documentation | README media refreshed for Muse and Grok

- Recaptured all README/demo screenshots from the demo frontend at 1440x960:
  `docs/images/{dashboard,skills,skill-sets,sources-extend}.png` and
  `video/public/ui/{dashboard,skills-before,skills-pending,skills-after,sources}.png`.
  Every capture now shows the four managed tool columns; `skills.png` is new and
  is the clearest README evidence of Claude/Codex/Muse/Grok parity.
- Storyboard revision 5: scene 1 shows four filesystem cards (added
  `~/.config/muse/skills`, `~/.grok/skills`), the social preview grid has four
  tool columns, and the eyebrow/supporting/footer copy reads
  "Claude Code · Codex · Muse · Grok". `storyboard.test.ts` locks the tool list
  and the per-tool copy so a fifth tool cannot be added silently.
- Re-rendered `.github/assets/demo.gif` (9.8 MiB, under the 10 MiB contract) and
  `social-preview.png` (1280x640, 330 KiB); `npm run verify` passes. The social
  preview still has to be uploaded by hand in repository settings.
- Closes the item deferred in the PR12 follow-up entry above.

## [2026-09-03] release | Prepare v0.6.0 public preview

- Selected a minor prerelease for the completed Muse and Grok managed-tool work
  while keeping the Apple Silicon, ad-hoc-signed preview limitations.
- Bumped `desktop/wails.json`, `desktop/frontend/package.json`, its lockfile,
  and the README download/version guidance to `0.6.0`; added
  `docs/releases/v0.6.0.md`.
- Third-party notices regenerated with the documented generator and unchanged.
