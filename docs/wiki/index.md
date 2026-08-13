# Skill Manager Wiki Index

This is the content map for the `skill-manager` LLM wiki. Update it whenever a
page is added, removed, renamed, or materially repurposed.

## Start Here

- [README.md](README.md) - operating contract, source precedence, claim labels,
  and ingest/query/lint workflows.
- [topics/project-overview.md](topics/project-overview.md) - compact product and
  implementation snapshot.
- [topics/architecture-and-data-flow.md](topics/architecture-and-data-flow.md)
  - package responsibilities and the scan, toggle, install, update, and
    uninstall flows.
- [topics/desktop-gui.md](topics/desktop-gui.md) - Wails/React architecture,
  security boundary, scan/apply and source-lifecycle session models,
  explicit context diagnostics, dormant Discover domain, visual contract, and packaging.
- [log.md](log.md) - append-only maintenance timeline.

## Topics

- [topics/project-overview.md](topics/project-overview.md)
  - goals, current implementation phase, authoritative documents, and package
    map.
- [topics/architecture-and-data-flow.md](topics/architecture-and-data-flow.md)
  - runtime boundaries and end-to-end data flows through CLI, TUI, scanning,
    operations, installation, and state.
- [topics/skill-discovery-and-grouping.md](topics/skill-discovery-and-grouping.md)
  - managed/read-only discovery, `SKILL.md` validation, metadata, source labels,
    groups, and row merging.
- [topics/toggle-and-state-lifecycle.md](topics/toggle-and-state-lifecycle.md)
  - reversible enable/disable moves, conflicts, deterministic batches, and TUI
    pending changes.
- [topics/repository-install-workflow.md](topics/repository-install-workflow.md)
  - URL normalization, managed checkouts, discovery, preflight, symlink apply,
    fast-forward update, transactional uninstall, rollback, and strict dry-run.
- [topics/local-path-install-workflow.md](topics/local-path-install-workflow.md)
  - canonical local source identity, direct/recursive discovery, link-in-place
    install, manifest ownership, source-preserving uninstall, and dry-run.
- [topics/interfaces.md](topics/interfaces.md)
  - command surface, TUI interaction model, filters, details, and bulk-toggle
    semantics.
- [topics/desktop-gui.md](topics/desktop-gui.md)
  - native app scope/stack, identifier-only bindings, pending and source
    operation sessions, global context-budget metrics, dormant skills.sh domain,
    implementation-derived design, responsive behavior, and build commands.
- [topics/state-safety-and-recovery.md](topics/state-safety-and-recovery.md)
  - manifest shape, backups, atomic writes, mutation boundaries, and recovery
    limits.
- [topics/testing-development-and-roadmap.md](topics/testing-development-and-roadmap.md)
  - test strategy, local development workflow, completed phases, and explicitly
    deferred work.

## Sources

- [sources/product-decisions-and-plans.md](sources/product-decisions-and-plans.md)
  - digest and routing map for `AGENTS.md`, `README.md`, `DESIGN.md`, and the
    twelve iteration plans.
- [sources/implementation-snapshot-2026-08-11.md](sources/implementation-snapshot-2026-08-11.md)
  - dated package-level source inspection and verification status used to seed
    the initial topic pages.
