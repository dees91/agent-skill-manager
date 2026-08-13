# Phase 11: Publication Readiness - Task Breakdown

> **Source**: user-approved publication review and planning session
> (2026-08-13). This phase prepares version `0.4.0` for a public GitHub source
> repository without changing runtime skill-management semantics.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Publish under `github.com/dees91/agent-skill-manager` with macOS bundle identifier
  `io.github.dees91.skillmanager`.
- Use the MIT license and include public security, privacy, and contribution
  policies.
- Publish source for Apple Silicon macOS. CI, notarization, Developer ID
  signing, universal binaries, installers, and release automation are deferred.
- Keep current fixed provider paths. Configurable paths and supported
  non-macOS behavior require a later product iteration.
- Preserve plans and the LLM wiki after removing workstation-specific history,
  paths, inventories, fingerprints, and external reference captures.
- Generate public screenshots from synthetic demo data only.
- Create one parentless public root commit. Preserve the prior development
  history only in a private bundle stored outside the repository.
- Do not add GitHub Actions in this iteration.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P11-T01 | Public identity, licensing, and user documentation | done | - |
| P11-T02 | Clean-checkout build and verification | done | P11-T01 |
| P11-T03 | Synthetic demo data and repository screenshots | done | P11-T01 |
| P11-T04 | Documentation and history anonymization | done | P11-T01 |
| P11-T05 | Full verification and single-root history cutover | done | P11-T02, P11-T03, P11-T04 |

## Task Definitions

### P11-T01: Public Identity, Licensing, And User Documentation

- Update both Go modules, internal imports, Wails metadata, and production/dev
  bundle identifiers.
- Add MIT, SECURITY, PRIVACY, and CONTRIBUTING files and replace the README
  with a reusable install, support, safety, and development guide.

### P11-T02: Clean-Checkout Build And Verification

- Build the ignored frontend output before compiling or testing the embedded
  desktop module.
- Verify namespaces, private strings and artifacts, tests, vet, dependencies,
  and CLI installation from a clean tracked-file export.

### P11-T03: Synthetic Demo Data And Repository Screenshots

- Replace provider-specific demo inventory and paths with synthetic examples.
- Capture Dashboard and Discover at 1440×960 and ensure image payloads contain
  no home paths, usernames, or email addresses.

### P11-T04: Documentation And History Anonymization

- Remove external screenshots and workstation-specific verification records.
- Keep authoritative plans and wiki synthesis aligned with the public product
  contract and add this phase to all routing documents.

### P11-T05: Full Verification And Single-Root History Cutover

- Run root, desktop, frontend, packaging, signature, launch, archive, and secret
  checks from isolated or synthetic state.
- Create one parentless `feat: publish Skill Manager 0.4.0` commit, expire local
  reflogs, prune superseded objects, and confirm only the intended public root
  remains reachable. Do not create a remote, tag, release, or push.
