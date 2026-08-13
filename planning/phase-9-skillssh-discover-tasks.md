# Phase 9: skills.sh Discover - Task Breakdown

> **Source**: user-approved grilling and planning session (2026-08-12).
> Iterations 1-8 are complete. Iteration 9 adds an experimental, read-only
> skills.sh catalog with exact single-skill installation into Claude Code and
> Codex.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Add a top-level `Discover` screen with all-time, trending, and hot rankings,
  plus debounced search and progressive loading.
- Use isolated anonymous JSON endpoints exposed by skills.sh. Treat the
  integration as experimental: no OIDC, no HTML scraping, and no silent
  fallback to scraped markup.
- Cache normalized ranking/search pages and parsed detail metadata. Cached
  results remain browsable offline, but installation requires a fresh live
  detail response.
- Install exactly the selected GitHub-hosted skill and exactly the selected
  Claude/Codex cells through the existing source ownership, preflight, apply,
  backup, and rescan machinery.
- Require a safety confirmation explaining that third-party skill instructions
  may be unsafe and should be reviewed before use.
- Show non-GitHub well-known catalog entries but keep them read-only until a
  later scope defines their trust and installation model.
- Do not add telemetry, ratings, comments, individual-skill uninstall, or a
  second filesystem mutation implementation.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P9-T01 | Catalog adapter, cache, and offline behavior | done | - |
| P9-T02 | Exact catalog install orchestration and Wails API | done | P9-T01 |
| P9-T03 | Discover interface and frontend behavior tests | done | P9-T02 |
| P9-T04 | Documentation, verification, and ARM64 bundle | done | P9-T03 |

## Task Definitions

### P9-T01: Catalog Adapter, Cache, And Offline Behavior

- Isolate skills.sh endpoint shapes and validation in one package.
- Normalize rankings, search, activity, and detail metadata into internal
  types; reject unsafe identifiers and oversized responses.
- Persist a versioned cache below `~/.skill-manager/cache/skills-sh/` without
  storing raw downloaded skill files.
- Fall back to cached catalog data on network failure and label it offline.

### P9-T02: Exact Catalog Install Orchestration And Wails API

- Project catalog entries against current active, disabled, conflict, and
  Skill Manager ownership state for both tools.
- Revalidate detail data live immediately before installation.
- Reject stale/offline, unknown-session, well-known, pending-toggle, and busy
  installation attempts.
- Resolve the fixed GitHub source and exact skill path through the existing
  checkout, discovery, exact-cell planner, apply service, and rescan flow.
- Expose path-free Wails methods using catalog session identifiers, skill
  identifiers, and tool names only.

### P9-T03: Discover Interface And Frontend Behavior Tests

- Add top-level navigation, ranking tabs, search, load-more behavior, activity
  visualization, local-state badges, and an accessible detail drawer.
- Add an external-only audit link and clear well-known/offline limitations.
- Add a per-agent install confirmation with a prominent third-party-code
  warning and operation progress.
- Cover browse/search race behavior, well-known entries, exact tool selection,
  successful install, and pending-toggle blocking in frontend tests.

### P9-T04: Documentation, Verification, And ARM64 Bundle

- Synchronize AGENTS, README, DESIGN, wiki synthesis/log, generated bindings,
  and the desktop version.
- Run all Go tests, frontend tests/typecheck, formatting, vet, design/wiki
  checks, and production builds.
- Exercise the Discover flow against the demo backend and build the local
  ad-hoc-signed Apple Silicon `.app` bundle.
