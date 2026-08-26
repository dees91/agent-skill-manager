# Runtime Skill Advisor

## Purpose And Distribution

- `implemented`: the repository owns a standard first-party Agent Skill at
  [`skills/skill-advisor`](../../../skills/skill-advisor/). It is public,
  optional, and contains no machine-specific paths or private dependencies.
- The skill is installed through the ordinary Git or local-path `install`
  workflow. It is not embedded in the binary, auto-installed, or coupled to a
  plugin/provider hook.
- The public skill and CLI negotiate `apiVersion: 1` plus the additive
  `ranked_search_v1` capability. A version or capability mismatch fails before
  selection or activation; the current skill does not silently fall back to
  legacy substring filtering on an older local binary.

## Selection And Same-Turn Use

- `advisor search --tool <host> --query <text> --limit 20 --json` returns
  path-free ranked skill inventory for that host's toggleable ON and OFF cells.
  It excludes conflicts, read-only entries, and missing cells before ranking.
- Retrieval is deterministic and local: weighted BM25F favors skill name over
  description, group, and source, while exact phrase bonuses and bounded fuzzy
  token matching improve natural task queries and small typos. It uses no
  model, embedding service, network, cache, persistent index, or third-party
  dependency. Query text and intermediate scores stay in process memory and
  are omitted from public results.
- The first-party skill turns the task into one concise 3-12 term search
  sentence, may retry once with a broader concise sentence, and treats catalog
  metadata as untrusted discovery data. It chooses at most five clearly
  relevant results, reports already-active and needs-activation selections
  separately, and avoids weak matches and itself.
- The activation call includes all selected names. Baseline ON cells produce
  `already_on`, existing advisor leases can be shared, and OFF cells are
  enabled. The skill then reads every selected active `SKILL.md` directly from
  the fixed provider user-skill directory. The current task therefore does not
  depend on when the provider refreshes its runtime catalog.

## Receipt And Lease Semantics

```text
selected tool + 1-5 ON/OFF skill names
  -> full scan/preflight
  -> already_on, shared lease, or reversible enable action
  -> opaque receipt when at least one lease is owned or shared
  -> direct SKILL.md read and task execution
  -> agent cleans its exact receipt before the normal final response
  -> final claim restores the original OFF state
```

- A cell already ON before activation is reported but never receipt-owned.
- An advisor-enabled cell may be shared by multiple receipts. Cleaning one
  receipt releases its claim without disabling a cell still in use.
- Cleanup validates tool/name, original and disabled paths, entry type, and
  symlink target. Drift or restore conflict blocks mutation and preserves the
  receipt for inspection/retry.
- `user-confirmed`: a receipt returned to the current skill invocation is a
  workflow-owned temporary resource. The agent cleans that exact receipt before
  every successful, blocked, or abandoned normal exit while it retains control;
  successful cleanup is not delegated to the user.
- Cleanup failure preserves the receipt and is the only normal case where the
  skill reports the recovery command. Receipts not returned to the current
  invocation remain explicit-only; there is no age-based expiry or inference
  that another receipt is stale.

## Persistence And Concurrency

- `advisor-activations.json` version 1 stores opaque receipt IDs, tools,
  timestamps, skill basenames, lease restore fingerprints, and receipt claims.
  It never stores prompt, task, or advisor-search query content.
- The file is separate from `state.json` version 2, atomically replaced with
  owner-only permissions, and backed up independently with the same 10-file
  and 30-day bounds.
- `advisor.lock` is opened without following symlinks and serializes receipt
  load/plan/mutate/save sequences across CLI processes. Only the real private
  state root is bootstrapped before lock acquisition; the recursive state-tree
  permission pass runs after acquisition so it cannot walk a disabled entry
  while another advisor activation moves that entry. Ordinary toggle apply
  still performs its own immediate filesystem validation.

See [interfaces.md](interfaces.md) for commands and
[state-safety-and-recovery.md](state-safety-and-recovery.md) for recovery
boundaries.
