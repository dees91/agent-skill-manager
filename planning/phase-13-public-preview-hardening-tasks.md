# Phase 13: Public Preview Hardening - Task Breakdown

> **Status:** done (2026-08-13). This phase publishes `v0.4.1` as the first
> public binary preview and addresses the privacy/licensing audit.

## Accepted Decisions

- Publish a preview, not a stable release.
- Replace the private `v0.4.0` prerelease/tag with public `v0.4.1`.
- Hide experimental Discover from the public desktop build while preserving
  and testing its Go adapter/domain code.
- Run local provider diagnostics only after an explicit Dashboard action.
- Keep at most 10 state backups and 30 days.
- Continue as free, non-monetized MIT software; re-audit before commercialization.
- Keep manual local release builds and manual GitHub publication.
- Defer Developer ID signing, notarization, universal binaries, SBOM,
  provenance, auto-update, and release automation until before stable.

## Task Status

| ID | Task | Status |
|---|---|---|
| P13-T01 | Owner-only state/cache permissions and bounded backup retention | done |
| P13-T02 | Remove persisted catalog searches and migrate legacy cache | done |
| P13-T03 | Make provider diagnostics explicit, allowlisted, and env-minimized | done |
| P13-T04 | Remove Discover from public Wails bindings and React navigation | done |
| P13-T05 | Generate and package dependency notices for CLI and desktop | done |
| P13-T06 | Update privacy, security, release, README, agent brief, and wiki | done |
| P13-T07 | Add CI, Dependabot, templates, and repository security settings | done |
| P13-T08 | Run clean release packaging, replace tag/release, and publish repo | done |

## Completion Criteria

- Default desktop startup does not execute provider CLIs or contact skills.sh.
- Diagnostics use only documented read-only commands and receive no secrets or
  proxy variables from the parent environment.
- Existing state metadata is repaired to owner-only permissions without
  changing checkout source modes; backup pruning is deterministic and tested.
- Search queries are absent from newly written caches and removed from legacy
  caches on first desktop snapshot.
- Public Wails bindings and frontend production assets expose no Discover API
  or navigation.
- Both archives reproduce required third-party license notices; packaging
  fails if the notice file is stale or missing after extraction.
- The public `v0.4.1` prerelease is built from and points to the verified public
  `main` commit, with all repository security controls documented below enabled.
