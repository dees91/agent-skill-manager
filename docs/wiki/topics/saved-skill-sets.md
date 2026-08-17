# Saved Skill Sets

## Purpose And Semantics

- `implemented`: Phase 14 adds reusable, task-oriented recipes to the macOS
  app. A Skill Set has a stable opaque ID, unique name, optional `When to use`
  description, and sorted unique skill basenames.
- Skill Sets are separate from source-derived Groups. A skill may belong to
  several sets, and sets do not own, activate, or reference-count skills.
- Membership is tool-agnostic. Every use explicitly selects Claude, Codex, or
  both and reuses the ordinary smart-toggle plus Pending/Apply boundary.
- Placement and state come from the live scan. A missing member remains saved
  as unavailable and reconnects when the same basename is discovered again,
  regardless of source.
- CLI and TUI management, task-history suggestions, active-profile ownership,
  project-local sets, import/export, per-skill notes, and ordered workflow steps
  remain outside Phase 14.

## Persistence Boundary

[`internal/skillsets`](../../../internal/skillsets/) owns the separate
`~/.skill-manager/skill-sets.json` version 1 file. It never stores tool
selection, installation source, or filesystem paths. Writes use a same-directory
temporary file plus rename, owner-only permissions, and independent
`backups/skill-sets-<timestamp>.json` copies capped at 10 and 30 days.

Missing storage loads as an empty set list. Unsupported versions, malformed
JSON, unsafe non-regular files, duplicate IDs/names, invalid basenames, and
empty sets are rejected. A Skill Set load failure is projected as a warning;
normal scan, staging, Apply, and source lifecycle behavior remain available.
Recipe mutations stay blocked until the metadata error is repaired.

## Desktop Flow

```text
Skill Set ID + explicit tool names
  -> gui.Service resolves saved basenames against current SkillRows
  -> read-only preview runs staging.ToggleBatch on a Pending copy
  -> confirmed use runs staging.ToggleBatch on session Pending
  -> path-free ActionResult projects every overlapping set
  -> ordinary Apply performs the filesystem moves and rescans
```

The dedicated workspace lists set descriptions, members, unavailable counts,
and applied/effective Claude and Codex summaries. Member rows expand in place.
Create/edit/delete dialogs mutate only recipe metadata. Deleting a set leaves
already-staged Pending cells untouched.

Contextual entry points are:

- **Save as set** in the Pending bar, which seeds unique pending skill names.
- **Add to Skill Set…** in skill details, which edits an existing set or starts
  a new one with that basename selected.

Create accepts only currently toggleable managed names. Edit may retain an
already-recorded unavailable member, but cannot introduce an arbitrary new
missing name. Source uninstall previews list affected sets and member names as
a non-blocking warning. Uninstall never rewrites or deletes recipes.

## Verification

Temporary-home Go tests cover persistence validation, permissions, bounded
backups, CRUD, corruption isolation, basename reconnection, overlapping set
projection, preview/staging, Pending preservation, and uninstall impact.
Frontend tests cover navigation, explicit tool selection, contextual creation,
and uninstall warnings. Synthetic browser QA validates the 1440×960 and
1024×720 layouts, keyboard-contained dialogs, console health, and WCAG A/AA
automation.

See [desktop-gui.md](desktop-gui.md),
[state-safety-and-recovery.md](state-safety-and-recovery.md), and
[toggle-and-state-lifecycle.md](toggle-and-state-lifecycle.md) for the shared
session, persistence, and smart-toggle contracts.
