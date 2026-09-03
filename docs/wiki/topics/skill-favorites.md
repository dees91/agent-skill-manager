# Skill Favorites

## Purpose And Semantics

- `implemented`: Phase 16 adds tool-agnostic favorites to the macOS Skills
  workspace for finding recurring skills in large local catalogs.
- Identity is the skill basename. A favorite applies across Claude, Codex, and Muse
  and remains independent from ON/OFF state, Pending, Apply, source ownership,
  Groups, and Skill Sets.
- Managed user rows are eligible while ON, OFF, or conflicted. Read-only-only
  Codex system and Claude plugin rows are excluded.
- A missing basename stays saved but hidden. Reinstalling the same basename
  reconnects it regardless of source.
- CLI, TUI, list JSON, Skill Advisor recommendations, synchronization, tags,
  notes, and project-local favorites remain outside Phase 16.

## Persistence And API Boundary

[`internal/favorites`](../../../internal/favorites/) owns the separate
`~/.skill-manager/favorites.json` version 1 file. It stores sorted unique skill
basenames only. Writes use same-directory temporary replacement, owner-only
permissions, and independent `favorites-*.json` backups capped at 10 and 30
days.

Missing storage loads as an empty list. Unsupported versions, malformed JSON,
unsafe non-regular files, and invalid basenames are rejected. Load failure is a
Skills-local warning: favorite controls are disabled, while scan, toggles,
Pending, Apply, and source lifecycle remain available.

The Wails mutation accepts only `skillName` and the desired boolean state.
Adding resolves the current managed row and validates eligibility; removal is
idempotent and can clear a saved missing basename. The result contains a
path-free sorted favorite list used to update the current React snapshot.

## Desktop Behavior

- Eligible rows and skill details expose an accessible star control.
- `Favorites N` is a fourth mutually exclusive availability chip that still
  composes with search, tool, Group, Source, and read-only filters.
- Favorites keep normal `Needs attention`, `Active now`, and `Available by
  source` placement. Favorite rows and source groups sort before non-favorites.
- Favorites mode temporarily expands matching available groups and restores
  the session's manual accordion state after leaving the filter.
- Filtered-result smart-toggle uses the favorite result scope; whole-group
  smart-toggle still targets the complete loaded group.
- Source uninstall reports matching favorite names, retains their metadata, and
  warns that removed skills may remain unavailable until reinstalled.

## Verification

Temporary-home Go tests cover format and basename validation, idempotency,
permissions, bounded backups, corruption isolation, conflict/read-only
eligibility, Pending independence, reconnection, busy-state exclusion, and
uninstall retention. Frontend tests cover row/details controls, filter/group
behavior, metadata-error isolation, Pending preservation, and uninstall copy.

See [desktop-gui.md](desktop-gui.md),
[state-safety-and-recovery.md](state-safety-and-recovery.md), and
[toggle-and-state-lifecycle.md](toggle-and-state-lifecycle.md).
