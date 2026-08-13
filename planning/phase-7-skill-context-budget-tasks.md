# Phase 7: Skill Context Budget Dashboard - Task Breakdown

> **Source**: user-approved planning session (2026-08-11). Iterations 1-6 are
> complete. Iteration 7 adds read-only global skill-catalog context metrics to
> the existing macOS Dashboard.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Measure startup skill metadata rather than full `SKILL.md` bodies.
- Show the entire discoverable global catalog, excluding repository and
  active-session-only skills.
- Codex reports the 2% catalog budget; Claude reports its configured budget,
  defaulting to 1%.
- Token values are character-derived estimates and carry an approximation mark.
- Claude uses a labeled 200k fallback when its model window is unresolved and
  remains labeled partial when opaque sources cannot be measured.
- Pending GUI toggles show a projected `After Apply` value without applying.
- Diagnostics are read-only and best-effort; failure cannot block core GUI use.
- No CLI or TUI context-budget surface is added in this iteration.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P7-T01 | Product contract and context-budget model | done | - |
| P7-T02 | Provider analyzers and GUI backend projection | done | P7-T01 |
| P7-T03 | Dashboard context-budget panel and pending preview | done | P7-T02 |
| P7-T04 | Documentation, wiki, verification, and build | done | P7-T03 |

## Task Definitions

### P7-T01: Product Contract And Context-Budget Model

- Record scope, provider defaults, approximation rules, fallbacks, and partial
  coverage semantics in authoritative product and design documents.
- Define serializable current/projected report types with health and accuracy.

### P7-T02: Provider Analyzers And GUI Backend Projection

- Add deterministic character/token/budget calculations and provider adapters.
- Use fixed-argument, timeout-bounded local diagnostics without a shell.
- Cache the applied catalog report per scan and project pending deltas in memory.
- Preserve successful GUI scans when diagnostics are unavailable.

### P7-T03: Dashboard Context-Budget Panel And Pending Preview

- Add the two-tool context panel, accessible progress state, coverage labels,
  provider explanations, and responsive layout.
- Update projected values immediately for stage, undo, clear, and Apply flows.

### P7-T04: Documentation, Wiki, Verification, And Build

- Add backend and frontend tests using temporary homes and fake diagnostics.
- Update README, DESIGN, generated bindings, and wiki synthesis/log.
- Run Go tests/vet, frontend tests/typecheck, design validation, Wails build,
  binary checks, and `make dev`.
