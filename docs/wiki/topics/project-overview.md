# Project Overview

## Current Synthesis

- `skill-manager` is a local Go terminal and macOS desktop application for
  managing globally installed Claude Code and Codex skills.
- `user-confirmed`: the public repository and Go module use
  `github.com/dees91/agent-skill-manager`; the product and binary remain named
  `Skill Manager` and `skill-manager`.
- Its core promise is visibility plus reversible control: show what each tool
  can see, temporarily remove toggleable entries from discovery paths, and
  restore the exact original entry later.
- The current implementation covers the Phase 1 MVP, Phase 2 group/bulk
  toggles, Phase 3 Git repository installation, Phase 4 fast-forward update
  plus whole-repository uninstall, Phase 5 link-in-place local sources, and
  Phase 6 Wails/React desktop management, Phase 7 global skill-catalog context
  visibility, Phase 8 managed source lifecycle in the desktop app, Phase 9
  experimental skills.sh discovery with exact selected-skill installation,
  Phase 10 active-first source-grouped Skills workspace, and Phase 11 public
  namespace, documentation, synthetic assets, clean-checkout verification, and
  history hygiene. Phase 12 adds locally verified Apple Silicon desktop and CLI
  archives; Phase 13 hardens privacy/licensing and publishes `v0.4.1` as the
  first public binary preview. The `v0.4.2` maintenance preview supersedes it
  with security and dependency updates plus an empty-profile GUI fix. Phase 14
  adds saved task-oriented Skill Sets to the desktop source build without
  changing CLI/TUI behavior. Phase 15 adds a public first-party Skill Advisor
  plus a tool-neutral receipt/lease CLI for temporary task-specific activation.

## Product Shape

- Interactive Bubble Tea TUI for scanning, filtering, staging, and applying
  enable/disable changes.
- Minimal CLI for listing/status, direct enable/disable, group/repository
  summaries, managed Git repository lifecycle, local path install/uninstall,
  path-free JSON inventory, and receipt-scoped advisor activation/cleanup.
- Dark-only Wails desktop with Dashboard, Skills, saved Skill Sets, and
  managed-only Sources over the same scan, staging,
  install/update/uninstall, and state boundaries as the terminal interfaces.
  The experimental Discover domain is dormant and has no public
  binding/navigation in `v0.4.2`. The Dashboard shows approximate Claude and
  Codex startup catalog cost and runs provider diagnostics only after an
  explicit action. Skills keeps applied ON rows prominent and collapses the
  much larger OFF catalog by source while preserving pending Apply semantics.
  Skill Sets remember overlapping task recipes and stage them for an explicitly
  chosen tool scope.
- Fixed macOS-oriented paths derived directly from `$HOME`; no configuration
  file in the current scope.
- Local state under `~/.skill-manager/`, including disabled entries, manifest
  backups, separate saved-recipe and advisor receipt metadata, managed
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
- User-facing commands and behavior: [`../../../README.md`](../../../README.md).
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
