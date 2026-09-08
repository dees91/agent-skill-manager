# Project Overview

## Current Synthesis

- `skill-manager` is a local Go terminal and macOS desktop application for
  managing globally installed Claude Code, Codex, Muse, and Grok skills.
- `user-confirmed`: the public repository and Go module use
  `github.com/dees91/agent-skill-manager`; the product and binary remain named
  `Skill Manager` and `skill-manager`.
- Its core promise is visibility plus reversible control: show what each tool
  can see, temporarily remove toggleable entries from discovery paths, and
  restore the exact original entry later.
- The current public preview is `v0.7.0`, covering Phases 1 through 21:
  live discovery and reversible toggles, groups and bulk staging, managed Git
  and local sources, macOS GUI and context estimates, saved Skill Sets,
  favorites, Skill Advisor with ranked retrieval, four-tool support, and
  managed checkout diagnosis/repair. See the
  [release history](../../releases/) and [iteration plans](../../../planning/).
- `user-confirmed`: README leads with the macOS app, existing demo, and a
  numbered quick start. Detailed operator instructions live in
  [the user guide](../../usage.md), while source builds live in
  [Contributing](../../../CONTRIBUTING.md). This editorial order does not change
  the CLI/TUI product contracts.
- `documented`: the current public preview and `main` receive best-effort
  security support without a guaranteed response or remediation time.

## Product Shape

- Interactive Bubble Tea TUI for scanning, filtering, staging, and applying
  enable/disable changes.
- Minimal CLI for listing/status, direct enable/disable, group/repository
  summaries, managed Git repository lifecycle, local path install/uninstall,
  path-free JSON inventory, ranked advisor search, and receipt-scoped advisor
  activation/cleanup.
- Dark-only Wails desktop with Dashboard, Skills, saved Skill Sets, and
  managed-only Sources over the same scan, staging,
  install/update/uninstall, and state boundaries as the terminal interfaces.
  The experimental Discover domain is dormant and has no public
  binding/navigation in `v0.7.0`. The Dashboard shows approximate Claude,
  Codex, Muse, and Grok startup catalog cost and runs provider diagnostics only after
  an explicit action. Skills keeps applied ON rows prominent and collapses the
  much larger OFF catalog by source while preserving pending Apply semantics.
  Favorites keep recurring managed basenames easy to find across ON/OFF state.
  Skill Sets remember overlapping task recipes and stage them for an explicitly
  chosen tool scope.
- The standard macOS application menu exposes the current source build version
  and product description through native About UI backed by embedded bundle
  metadata.
- Fixed macOS-oriented paths derived directly from `$HOME`; no configuration
  file in the current scope.
- Local state under `~/.skill-manager/`, including disabled entries, manifest
  backups, separate favorite, saved-recipe, and advisor receipt metadata, managed
  repository checkouts, and transient uninstall staging.

## Primary Invariants

- Scan the filesystem each run; do not rely only on cached manifest data.
- Hide entries without `SKILL.md`.
- Preserve symlink-versus-directory identity and symlink targets.
- Treat user skill directories as managed and system/plugin caches as
  read-only.
- Never overwrite a blocker during restore or install.
- Never edit `SKILL.md`, plugin caches, or external skill manager lockfiles;
  repository updates are restricted to audited fast-forward Git changes.
- Never copy, update, move, stage, or delete a link-in-place local source.
- Keep mutating operations observable through dry-run and safe to test with a
  temporary home.

## Authority Map

- Product intent and invariants: [`../../../AGENTS.md`](../../../AGENTS.md).
- First launch and product introduction: [`../../../README.md`](../../../README.md).
- Detailed user-facing commands and behavior: [`../../usage.md`](../../usage.md).
- Source builds and checks: [`../../../CONTRIBUTING.md`](../../../CONTRIBUTING.md).
- Iteration status: [`../../../planning/`](../../../planning/).
- Actual runtime behavior: source and tests under [`../../../internal/`](../../../internal/).
- Cross-source synthesis and history: this wiki.

## Package Map

```text
main
  -> cli
      -> paths
      -> scan -> metadata, state, model
      -> ops  -> scan, state, model
      -> install -> paths, state, model, git/filesystem
      -> tui  -> staging -> model
      -> advisor -> scan, ops, state, model
      -> gui  -> staging, scan, ops, install, state, model
              -> favorites -> private versioned basename file
              -> skillsets -> private versioned recipe file
              -> contextbudget -> metadata, paths, model
              -> skillssh -> anonymous catalog HTTP + normalized disk cache

desktop (nested Wails module)
  -> gui
  -> generated bindings
  -> React/Vite frontend
```

See [architecture-and-data-flow.md](architecture-and-data-flow.md) for the
runtime flows and [testing-development-and-roadmap.md](testing-development-and-roadmap.md)
for completed and deferred scope.
