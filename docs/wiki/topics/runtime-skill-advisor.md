# Runtime Skill Advisor

## Purpose And Distribution

- `implemented`: the repository owns a standard first-party Agent Skill at
  [`skills/skill-advisor`](../../../skills/skill-advisor/). It is public,
  optional, and contains no machine-specific paths or private dependencies.
- The skill is installed through the ordinary Git or local-path `install`
  workflow. It is not embedded in the binary, auto-installed, or coupled to a
  plugin/provider hook.
- The public skill and CLI negotiate `apiVersion: 1`. Version mismatch fails
  before activation so a skill updated from `main` cannot silently misuse an
  older local binary.

## Selection And Same-Turn Use

- `list --json` returns path-free skill name, description, group, source, and
  explicit Claude/Codex cell state plus toggle eligibility.
- `--available-for` plus repeated `--query` terms bounds discovery to relevant
  OFF candidates inside Skill Manager, avoiding large raw inventories and an
  otherwise undeclared dependency on `jq`.
- The binary does not choose skills. The first-party skill treats catalog
  metadata as untrusted discovery data, selects at most five clearly relevant
  OFF cells for the current host, and avoids weak matches, conflicts,
  read-only entries, missing cells, and itself.
- After activation the skill reads each selected active `SKILL.md` directly
  from the fixed provider user-skill directory. The current task therefore
  does not depend on when the provider refreshes its runtime catalog.

## Receipt And Lease Semantics

```text
selected tool + 1-5 skill names
  -> full scan/preflight
  -> opaque receipt
  -> one lease claim per advisor-owned tool/skill cell
  -> existing reversible enable operation for OFF cells
  -> direct SKILL.md read and task execution
  -> explicit cleanup by exact receipt
  -> final claim restores the original OFF state
```

- A cell already ON before activation is reported but never receipt-owned.
- An advisor-enabled cell may be shared by multiple receipts. Cleaning one
  receipt releases its claim without disabling a cell still in use.
- Cleanup validates tool/name, original and disabled paths, entry type, and
  symlink target. Drift or restore conflict blocks mutation and preserves the
  receipt for inspection/retry.
- Cleanup is explicit. There is no automatic task-end cleanup, age-based
  expiry, or inference that another receipt is stale.

## Persistence And Concurrency

- `advisor-activations.json` version 1 stores opaque receipt IDs, tools,
  timestamps, skill basenames, lease restore fingerprints, and receipt claims.
  It never stores prompt or task content.
- The file is separate from `state.json` version 2, atomically replaced with
  owner-only permissions, and backed up independently with the same 10-file
  and 30-day bounds.
- `advisor.lock` is opened without following symlinks and serializes receipt
  load/plan/mutate/save sequences across CLI processes. Ordinary toggle apply
  still performs its own immediate filesystem validation.

See [interfaces.md](interfaces.md) for commands and
[state-safety-and-recovery.md](state-safety-and-recovery.md) for recovery
boundaries.
