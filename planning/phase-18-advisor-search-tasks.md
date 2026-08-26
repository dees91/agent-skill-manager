# Phase 18: Ranked Skill Advisor Search - Task Breakdown

> **Source**: user-approved planning session (2026-08-26).
> Iterations 1-17 are complete. Iteration 18 adds deterministic local ranked
> retrieval for the first-party Skill Advisor without changing legacy list
> filtering or any mutation semantics.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Add a separate `advisor search` surface and preserve `list --json --query` as
  the existing case-insensitive OR-substring filter.
- Accept one concise query, rank only the requested tool's toggleable ON/OFF
  cells, and return at most 20 results by default with an explicit 1-50 limit.
- Use deterministic weighted BM25F, exact phrase bonuses, and bounded
  Damerau-Levenshtein token matching implemented with the Go standard library.
- Keep the operation local, read-only, path-free, dependency-free, uncached,
  and free of persisted or echoed query text and public numeric scores.
- Advertise `ranked_search_v1` additively under advisor API version 1. The new
  first-party skill requires that capability rather than silently falling back
  to substring lookup.
- Keep the existing five-skill selection, activation, same-turn instruction
  loading, receipt sharing, and workflow-owned cleanup contracts unchanged.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P18-T01 | Pure ranked retrieval domain and focused tests | done | - |
| P18-T02 | Additive CLI and capability contract | done | P18-T01 |
| P18-T03 | First-party skill migration | done | P18-T02 |
| P18-T04 | Documentation, review, and verification | done | P18-T03 |

## Task Definitions

### P18-T01: Pure Ranked Retrieval Domain And Focused Tests

- Implement bounded Unicode tokenization, lightweight plural normalization,
  weighted BM25F, phrase bonuses, token-level fuzzy matching, and deterministic
  ranking over synthetic `SkillRow` inputs.
- Cover field weights, exact/fuzzy precedence, punctuation, plural forms,
  short tokens, eligibility, limits, and stable ties without external packages.

### P18-T02: Additive CLI And Capability Contract

- Add `advisor search` argument validation, human output, structured path-free
  JSON, errors, help text, and `ranked_search_v1` status capability.
- Preserve existing advisor/list JSON shapes and prove `list --json --query`
  retains its exact legacy filter and ordering behavior.

### P18-T03: First-Party Skill Migration

- Replace repeated substring queries with one concise ranked search sentence,
  capability preflight, one broader retry, and no old-CLI fallback.
- Preserve candidate distrust, reporting, activation, direct instruction
  loading, receipt ownership, and normal-exit cleanup semantics.

### P18-T04: Documentation, Review, And Verification

- Synchronize AGENTS, README, the advisor/interface/architecture/testing wiki,
  product-plan digest, and wiki log without adding another ADR convention.
- Run focused and full tests, vet, vulnerability/dependency gates, persistent
  CLI rebuild, read-only live search smoke tests, and a five-axis change review.
