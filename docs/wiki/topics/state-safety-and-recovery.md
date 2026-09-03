# State, Safety, And Recovery

## Managed Layout

```text
~/.skill-manager/
  state.json
  advisor-activations.json
  advisor.lock
  skill-sets.json
  favorites.json
  backups/
  disabled/
    claude/<skill-name>
    codex/<skill-name>
    muse/<skill-name>
  repos/<host>/<repo-path>
  trash/uninstall-<operation-id>/
```

All paths are derived from `$HOME` by [`internal/paths`](../../../internal/paths/paths.go).
Configuration-file overrides are intentionally absent in the current scope.

## Manifest Responsibilities

`state.json` version 2 stores three collections:

- Disabled entries: tool, skill name, original and disabled paths, entry type,
  optional symlink target, source, group, and disable timestamp.
- Managed repositories: original/canonical URL, normalized host/path, checkout,
  group, installed skill relative paths/tools, install timestamp, and optional
  last seen commit.
- Local sources: original and canonical absolute paths, group, installed skill
  relative paths/tools, and install timestamp.

Version 1 manifests load in memory with an empty local-source collection and
are written as version 2 on the next mutation. Unknown newer versions are
rejected. Repository identities, local source paths, skills, and tool lists are
normalized into deterministic order during load/save operations.

`skill-sets.json` version 1 is an independent recipe store. Each set contains a
stable opaque ID, unique name, optional description, sorted unique skill
basenames, and created/updated timestamps. It intentionally contains no tool
selection, source ownership, or filesystem path, and it does not change the
`state.json` version 2 schema.

`favorites.json` version 1 is an independent sorted list of unique skill
basenames. It intentionally contains no tool, source, path, timestamp, prompt,
or usage history and does not change the `state.json` version 2 schema.

`advisor-activations.json` version 1 is an independent receipt/lease store.
Receipts contain opaque IDs, one explicit tool, creation time, and up to five
skill basenames. Leases add only the exact restore fingerprints and receipt
claims needed to share and later restore advisor-owned cells. Prompt and task
content are never persisted, and the file does not change the `state.json`
version 2 schema.

The dormant skills.sh catalog cache is separate from `state.json` at
`cache/skills-sh/catalog-v1.json`. Its internal format is version 2: ranking
pages and parsed detail metadata may remain, but search terms/results are never
persisted. First desktop launch upgrades legacy version 1 caches and removes
their query data. Deleting the cache loses only offline catalog metadata;
installed-state projection still comes from the filesystem and manifest.

## Persistence Guarantees

- A missing manifest loads as an empty versioned manifest.
- Save writes indented JSON to a same-directory temporary file and replaces the
  manifest with `rename`.
- Skill Manager state directories use mode `0700`; state, backup, and cache
  JSON use `0600`. Startup/mutation repairs metadata permissions without
  following symlinks or changing managed checkout file modes.
- Before the first apply in a toggle, install, update, or uninstall service
  process, an existing manifest is copied to a timestamped backup.
- Backup rotation preserves the newest valid backup, retains at most 10, and
  removes valid backups older than 30 days while leaving foreign files alone.
- Skill Set mutations atomically replace their separate file and write an
  independent `skill-sets-*.json` backup before each replacement with the same
  owner-only, 10-file, 30-day bounds.
- Favorite mutations atomically replace their separate file and write an
  independent `favorites-*.json` backup before each changed replacement with
  the same owner-only, 10-file, 30-day bounds. Idempotent writes do not create a
  replacement or backup.
- Advisor mutations are serialized through a no-follow owner-only lock,
  atomically replace their separate file, and write independent
  `advisor-activations-*.json` backups with the same retention bounds.
- Toggle apply persists the successfully completed prefix when a later move
  fails.
- Install symlink apply updates repository metadata only after link creation
  succeeds.
- Update writes `lastSeenCommit` only after a successful fetch, target-tree
  preflight, and fast-forward (or confirmed up-to-date checkout).
- Uninstall stages only audited owned paths, saves the reduced manifest, and
  then removes staging. Pre-save failures restore staged paths in reverse order.

## Mutation Boundary

The tool may:

- move user skill entries between active and disabled directories;
- create managed checkout directories through `git clone`;
- create user-skill symlinks into those checkouts;
- create user-skill symlinks into user-owned local sources;
- fetch and fast-forward clean Skill Manager-managed checkouts;
- remove a complete audited managed repository installation through staging;
- remove an audited local installation's links and state through staging;
- write Skill Manager state and backups.
- write saved Skill Set metadata and its independent backups.
- write favorite basename metadata and its independent backups.
- write advisor receipt/lease metadata, its process lock, and independent
  backups;
- reuse exact reversible toggle operations for receipt-scoped activation and
  cleanup.

The tool must not:

- delete or edit skill source repositories or `SKILL.md`;
- copy, update, move, stage, or delete a link-in-place local source directory;
- rewrite Skills CLI or plugin metadata/lockfiles;
- mutate Codex system skills or Claude plugin cache skills;
- overwrite, merge, delete, or rename a conflict blocker;
- implicitly enable a matching skill that is already disabled.

## Recovery Limits

- Restore requires both the manifest record and the disabled filesystem entry.
- Full recovery by scanning `disabled/` without a manifest is deferred.
- Toggle batches do not provide transactional rollback; their completed prefix
  is the recoverable truth.
- Install rollback covers symlinks created by the failed apply, not a checkout
  cloned earlier in the operation.
- Local install rolls back its newly created links if state persistence fails.
- If state save fails after a Git fast-forward, the checkout is already updated
  and the error reports that state persistence failed.
- Uninstall rollback is available only before state save. If final staging
  cleanup fails after state save, the logical uninstall is complete and the
  reported `trash/` path requires manual cleanup.
- Conflict remediation is manual and should preserve both the disabled entry
  and the unexpected blocker until the user chooses a resolution.
- Malformed or unsupported Skill Set metadata disables recipe CRUD/toggling
  but is isolated from core scan, Pending, Apply, and source lifecycle. A valid
  backup may be restored manually without changing `state.json`.
- Malformed or unsupported favorite metadata disables only favorite controls.
  It remains isolated from core scan, Pending, Apply, and source lifecycle; a
  valid backup may be restored manually without changing `state.json`.
- Malformed or unsupported advisor metadata blocks advisor commands but remains
  isolated from normal list/status, direct toggles, TUI, GUI, and source
  lifecycle. Cleanup never guesses ownership from the filesystem alone.

Use CLI dry-run before a direct mutation and temporary-home tests during
development. Never validate destructive behavior against the real global skill
directories.
