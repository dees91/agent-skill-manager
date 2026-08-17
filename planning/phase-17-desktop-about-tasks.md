# Phase 17: Native Desktop About - Task Breakdown

> **Source**: user-approved planning session (2026-08-17).
> Iterations 1-16 are complete. Iteration 17 adds version visibility through
> the standard macOS application menu without changing app state or CLI/TUI
> behavior.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Add `Skill Manager → About Skill Manager` through Wails' native macOS About
  support. Do not add a custom React dialog or an in-app version label.
- Show the existing app icon, product name, current version, and the short
  product description.
- Read the product name, version, and description from the embedded
  `desktop/wails.json` metadata so the About dialog and app bundle share one
  version source.
- Keep Wails bindings, frontend state, CLI/TUI behavior, filesystem state, and
  source-management semantics unchanged.
- Build and verify the local app without replacing `/Applications/Skill
  Manager.app`, bumping the version, or publishing a release.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P17-T01 | Native About metadata, documentation, and verification | done | - |

## Task Definition

### P17-T01: Native About Metadata, Documentation, And Verification

- Add validated metadata projection and native Wails About configuration with
  focused tests for valid and invalid embedded metadata.
- Synchronize `AGENTS.md`, `README.md`, `DESIGN.md`, and the desktop wiki while
  correcting the stale design claim that version metadata appears in the
  navigation rail.
- Run the full backend/frontend verification, build the ad-hoc-signed Apple
  Silicon app, and confirm the native dialog content without installing it.
