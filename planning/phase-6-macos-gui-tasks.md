# Phase 6: macOS GUI - Task Breakdown

> **Source**: user-approved planning session (2026-08-11). Iterations 1-5 are
> complete. Iteration 6 adds a local Apple Silicon desktop interface without
> changing CLI/TUI source-management behavior.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Use Wails 2.13 in a nested desktop Go module so the root CLI module can retain
  its existing Go directive and command behavior.
- Use React, TypeScript, Vite, npm, generated Wails bindings, plain CSS tokens,
  and no router, external state store, Tailwind, or chart library.
- GUI v1 contains Dashboard and Skills only. Repository/local install, update,
  and uninstall remain CLI-only.
- Preserve pending-change, smart-toggle, conflict, deterministic apply, backup,
  and partial-failure semantics from the TUI.
- Keep pending state in the Go process. The frontend sends identifiers, never
  filesystem paths or prebuilt operations.
- Scan on startup, after Apply, and on explicit Refresh. Do not add filesystem
  watchers or polling.
- Ship a dark-only, local ad-hoc signed `darwin/arm64` `.app`. Developer ID
  signing, notarization, universal builds, light theme, and publishing are out
  of scope. Retain Wails' ad-hoc signature because removing it makes the bundle
  unreliable on supported macOS releases.
- Keep the validated design contract in root `DESIGN.md` and maintain
  repository-owned screenshots under `docs/images/` using synthetic demo data.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P6-T01 | Product and design contract | done | - |
| P6-T02 | Shared staging engine and GUI backend | done | P6-T01 |
| P6-T03 | Wails desktop foundation | done | P6-T02 |
| P6-T04 | React Dashboard and Skills interface | done | P6-T03 |
| P6-T05 | Packaging, documentation, wiki, and verification | done | P6-T04 |

## Task Definitions

### P6-T01: Product and Design Contract

- Record accepted scope in `AGENTS.md`, this task plan, and a validated
  implementation-derived `DESIGN.md`.
- Preserve repository-owned screenshots generated from synthetic demo data.
- Verification: the design contract and screenshot privacy checks pass.

### P6-T02: Shared Staging Engine and GUI Backend

- Extract the pure pending/smart-toggle behavior for reuse by TUI and GUI.
- Add a stateful, concurrency-safe desktop service for scanning, dashboard
  summaries, staging, applying, partial failures, and fresh snapshots.
- Verification: temporary-home backend tests and unchanged TUI regression tests.

### P6-T03: Wails Desktop Foundation

- Add the nested Wails 2 module, native macOS window configuration, generated
  type-safe bindings, application menu, close warning, build assets, and npm
  frontend foundation.
- Verification: desktop Go tests, frontend typecheck, and binding generation.

### P6-T04: React Dashboard and Skills Interface

- Implement the DESIGN.md shell, dashboard, filters, skill table, details,
  smart toggles, pending bar/review drawer, feedback, shortcuts, and accessible
  states.
- Verification: Vitest/Testing Library behavior tests and visual inspection at
  initial and minimum window sizes.

### P6-T05: Packaging, Documentation, Wiki, and Verification

- Add Make targets and README guidance, synchronize authoritative docs and wiki,
  build the ARM64 `.app`, and verify the full CLI/TUI/desktop/frontend suite.
- Verification: tests, vet, design validator, Wails build, binary architecture,
  app launch, and wiki link/index lint.
