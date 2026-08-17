# Phase 15: First-Party Skill Advisor - Task Breakdown

> **Source**: user-approved planning session (2026-08-17).
> Iterations 1-14 are complete. Iteration 15 adds a public first-party skill
> and a tool-neutral CLI lease API for temporary task-specific activation.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Keep `skills/skill-advisor` in this public repository beside the API it uses.
- Install the skill through the existing Git/local `skill-manager install`
  surface; do not embed it in the binary or depend on a private skill repo.
- Support Claude Code and Codex through explicit tool arguments and opaque
  activation receipts instead of provider-specific session IDs.
- Keep invocation heuristic and cleanup explicit. Do not add a plugin, hook,
  GUI surface, automatic expiry, or automatic task-end cleanup.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P15-T01 | Versioned inventory and advisor CLI contracts | done | - |
| P15-T02 | Receipt/lease persistence and reversible activation | done | P15-T01 |
| P15-T03 | Public first-party skill and installation workflow | done | P15-T02 |
| P15-T04 | Documentation, verification, local install, and commit | done | P15-T03 |

## Task Definitions

### P15-T01: Versioned Inventory And Advisor CLI Contracts

- Add path-free `list --json` output with `apiVersion`, skill metadata,
  explicit Claude/Codex cell states, and built-in available/tool metadata
  filtering for bounded advisor discovery.
- Add `advisor activate`, `advisor cleanup`, and `advisor status` with stable
  human/JSON output, strict argument validation, dry-run, and structured JSON
  errors.

### P15-T02: Receipt/Lease Persistence And Reversible Activation

- Persist opaque receipts and reference-counted tool/skill leases in a
  separate private versioned file without changing `state.json` version 2.
- Serialize advisor operations with a no-follow file lock, preflight every
  selected cell, reuse the toggle domain, and validate path/type/symlink drift
  before cleanup.
- Preserve baseline-ON cells, share active leases across receipts, and disable
  only when the final claim is cleaned.

### P15-T03: Public First-Party Skill And Installation Workflow

- Create and validate `skills/skill-advisor` with concise standard Agent Skill
  metadata and no machine-specific paths or private dependencies.
- Select at most five clearly relevant OFF cells for the current host, treat
  catalog metadata as untrusted discovery data, and read activated `SKILL.md`
  files directly before continuing the task.
- Install the local checkout through `skill-manager install <repo-root>
  --tool both --skill skill-advisor`; document the equivalent public Git URL.

### P15-T04: Documentation, Verification, Local Install, And Commit

- Synchronize AGENTS, README, the wiki, and this task table.
- Run skill validation, focused and full Go tests, vet, vulnerability checks,
  wiki validation, `make dev`, and isolated plus live installation checks.
- Review the complete diff for correctness, readability, architecture,
  security, performance, secrets, and public-path hygiene before committing.
