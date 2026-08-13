# Toggle And State Lifecycle

## Cell States

- `ON`: an active managed entry exists in the tool's user skill directory.
- `OFF`: a restorable disabled entry exists and the active path is free.
- `CONFLICT`: a disabled entry exists, but its original active path is blocked.
- `RO`: a visible system or plugin skill is read-only.
- `-`: the skill has no cell for that tool.

Pending state is an in-memory TUI or GUI overlay. It does not change the
filesystem until apply.

## Disable

1. Find an active, toggleable entry from the managed scan.
2. Resolve `~/.skill-manager/disabled/<tool>/<skill-name>`.
3. Require the active entry to match its recorded type and the destination to
   be free.
4. Move the entry with `rename`.
5. Upsert a manifest record containing its original/disabled paths, entry type,
   symlink target, source, group, and timestamp.

Moving a symlink moves the symlink itself; it does not dereference or modify
the repository it points to.

## Enable

1. Load the disabled record from `state.json`.
2. Require the disabled entry to exist with the expected type.
3. Require the exact original path to be free.
4. Move the entry back with `rename`.
5. Remove the disabled record from the manifest.

Any file, directory, or symlink at the original path blocks restore. Conflict
resolution is manual in the current scope.

## Batch Apply

- Apply order is disable operations first, enable operations second, then tool
  name and skill name.
- The executor stops at the first failed operation.
- The completed prefix is kept and written to state; there is no full batch
  rollback.
- Interactive UIs remove successfully completed cells from pending state,
  rescan, and keep structured failure information visible.

## Smart Pending Toggles

- Single-cell and both-tool row toggles only stage changes. The shared
  `internal/staging` engine prevents TUI/GUI semantic drift.
- Group toggle uses all loaded rows in the selected row's group.
- All-visible toggle uses rows after current text, source, group, and read-only
  filters.
- If every applicable effective cell is `ON`, the batch targets `OFF`.
- Otherwise, it targets `ON` for effective `OFF` cells.
- Read-only, missing, and conflict cells are skipped and counted.
- Repeating a matching batch action can remove existing pending changes; an
  opposite pending operation is replaced.

See [interfaces.md](interfaces.md) for user controls and
[state-safety-and-recovery.md](state-safety-and-recovery.md) for persistence.
