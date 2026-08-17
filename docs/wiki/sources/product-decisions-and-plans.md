# Product Decisions And Plans Source Digest

## Scope

This page routes agents to the authoritative product and planning sources used
to seed the wiki. It summarizes their roles; it does not replace them.

## Source Map

- [`AGENTS.md`](../../../AGENTS.md)
  - `documented`: canonical product intent, supported paths, toggle and install
    semantics, source/group labels, TUI/CLI behavior, safety boundaries, and
    documentation rules.
- [`README.md`](../../../README.md)
  - `documented`: current user-facing commands, TUI keys, behavior, conflicts,
    safety notes, and development commands.
- [`planning/phase-1-mvp-tasks.md`](../../../planning/phase-1-mvp-tasks.md)
  - `documented`: MVP dependency graph, acceptance criteria, completion notes,
    and task status. All Phase 1 tasks are marked `done`.
- [`planning/phase-2-group-bulk-toggle-tasks.md`](../../../planning/phase-2-group-bulk-toggle-tasks.md)
  - `documented`: group detection, group summaries, group filtering, and smart
    bulk-toggle semantics. All Phase 2 tasks are marked `done`.
- [`planning/phase-3-repo-install-tasks.md`](../../../planning/phase-3-repo-install-tasks.md)
  - `documented`: CLI-first Git repository installation, managed checkout,
    strict dry-run, preflight/idempotency, rollback, and repository manifest
    behavior. All Phase 3 tasks are marked `done`.
- [`planning/phase-4-repo-update-uninstall-tasks.md`](../../../planning/phase-4-repo-update-uninstall-tasks.md)
  - `documented`: CLI-first fast-forward repository update, shared reference
    auditing, transactional whole-repository uninstall, strict dry-runs, and
    failure recovery. All Phase 4 tasks are marked `done`.
- [`planning/phase-5-local-path-install-tasks.md`](../../../planning/phase-5-local-path-install-tasks.md)
  - `documented`: canonical local path identity, direct/recursive discovery,
    link-in-place install, manifest cell ownership, source-preserving whole
    uninstall, missing-source cleanup, and strict dry-runs.
- [`planning/phase-6-macos-gui-tasks.md`](../../../planning/phase-6-macos-gui-tasks.md)
  - `documented`: Wails/React desktop scope, shared staging/backend boundary,
    dark Dashboard/Skills UX, local Apple Silicon packaging, documentation,
    and verification tasks.
- [`planning/phase-7-skill-context-budget-tasks.md`](../../../planning/phase-7-skill-context-budget-tasks.md)
  - `documented`: global startup skill-catalog cost, Codex/Claude provider
    budgets, approximation and partial-coverage labels, pending projections,
    Dashboard UX, and verification tasks.
- [`planning/phase-8-gui-source-management-tasks.md`](../../../planning/phase-8-gui-source-management-tasks.md)
  - `documented`: managed-only Sources navigation, exact-cell install review,
    filter-independent per-tool column selection, native local selection, safe
    Git update, typed-confirmed source uninstall, exclusive progress, and
    packaging tasks.
- [`planning/phase-9-skillssh-discover-tasks.md`](../../../planning/phase-9-skillssh-discover-tasks.md)
  - `documented`: experimental anonymous skills.sh rankings/search/details,
  normalized offline cache, live revalidation, exact selected-skill/tool
  install, unsupported well-known sources, safety UX, and verification tasks.
- [`planning/phase-10-skills-workspace-tasks.md`](../../../planning/phase-10-skills-workspace-tasks.md)
  - `documented`: active-first Skills hierarchy, collapsed source packs,
    state/tool scopes, advanced filters, session-only expansion, scoped bulk
    bindings, frontend behavior, and packaging tasks.
- [`planning/phase-11-publication-readiness-tasks.md`](../../../planning/phase-11-publication-readiness-tasks.md)
  - `documented`: public namespace and policy files, anonymized documentation
    and screenshots, clean-clone verification, local publication checks, and
    single-root public history.
- [`planning/phase-12-github-release-tasks.md`](../../../planning/phase-12-github-release-tasks.md)
  - `documented`: locally verified macOS ARM64 desktop/CLI archives, CLI
    version reporting, versioned release notes, checksums, and manual GitHub
    prerelease publication.
- [`planning/phase-13-public-preview-hardening-tasks.md`](../../../planning/phase-13-public-preview-hardening-tasks.md)
  - `documented`: owner-only state/cache persistence, bounded backups,
    diagnostics privacy, dependency notices, hidden experimental Discover
    surface, and public preview publication.
- [`planning/phase-14-skill-sets-tasks.md`](../../../planning/phase-14-skill-sets-tasks.md)
  - `documented`: separate saved-recipe persistence, basename-based
    membership, scoped smart-toggle preview/staging, contextual GUI creation,
    unavailable-member retention, and source-uninstall impact warnings.
- [`planning/phase-15-skill-advisor-tasks.md`](../../../planning/phase-15-skill-advisor-tasks.md)
  - `documented`: first-party public skill distribution, versioned path-free
    inventory, opaque receipt/reference-counted lease activation, explicit
    cleanup, and same-turn instruction loading.
- [`planning/phase-16-skill-favorites-tasks.md`](../../../planning/phase-16-skill-favorites-tasks.md)
  - `documented`: private basename favorites, isolated persistence, path-free
    desktop mutation, favorite-first Skills filtering, missing-name
    reconnection, and source-uninstall impact.
- [`DESIGN.md`](../../../DESIGN.md)
  - `documented`: implementation-derived design system and repository-owned
    screenshots generated from synthetic demo data.

## Stable Product Invariants

- User skills for Claude and Codex are toggleable; Codex system and Claude
  plugin skills are read-only in the current product.
- Disable/enable moves the original entry and never edits the skill source.
- The exact entry type and symlink target must survive a round trip.
- Restore and install conflicts block rather than overwrite existing paths.
- Each recorded skill/tool cell has one Git or local source owner; later install
  plans cannot take it over merely because its active path is free or matching.
- Install uses managed Git checkouts and symlinks and never updates an existing
  checkout implicitly; update is a separate explicit fast-forward operation.
- Whole-repository uninstall removes only audited owned references, state, and
  checkout through a rollback-capable staging step.
- Local path install links into a user-owned source without copying it; local
  uninstall removes only audited links and state and never stages the source.
- Mutating commands support dry-run, and tests must use temporary home
  directories rather than the real global skill layout.
- Context-budget diagnostics are read-only and best-effort; full skill bodies
  and project/session-only catalogs are outside the Dashboard measurement.
- Desktop source mutations resolve opaque IDs in Go and reuse the CLI domain
  services; pending toggle batches and source lifecycle operations never
  overlap.
- Discover catalog failures are isolated from local management. Cached catalog
  metadata remains browseable offline, while install requires live detail
  revalidation and resolves only backend-owned catalog IDs into a GitHub source
  and exact selected cells.
- Desktop Skills classification uses applied state until Apply/rescan. Scoped
  batch calls contain only skill/group and tool identifiers and reuse the
  shared smart-toggle engine.
- Saved Skill Sets are overlapping task recipes, not source Groups or active
  profiles. Membership stores only basenames, each use explicitly selects a
  tool scope, and all state changes remain in the shared Pending/Apply model.
  Missing members stay recorded, and source uninstall warns without rewriting
  recipes.
- Skill Advisor owns only cells it moves from OFF to ON. Opaque receipts may
  share leases, final cleanup restores OFF after exact drift validation, and
  pre-existing ON cells remain outside advisor ownership.
- Favorites store only tool-agnostic basenames, never own skill visibility, and
  survive missing skills or source uninstall. Only managed user rows are
  eligible for the macOS star/filter surface.

## Maintenance Rule

If accepted behavior changes, update the authoritative files above first as
required by [`AGENTS.md`](../../../AGENTS.md), then update the relevant wiki
topics and append to [`../log.md`](../log.md).
