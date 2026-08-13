# Local Path Install Workflow

## Input And Identity

- `skill-manager install <local-path>` accepts absolute paths, `~/...`,
  explicit relative paths, and bare-relative directories that currently exist.
- Git HTTPS and SCP-style SSH inputs keep the repository workflow. A missing
  bare relative input is not guessed to be local.
- The source identity is its canonical absolute path after resolving symlinks.
  The display group is the canonical root basename and the source label is
  `local path`.
- A source cannot overlap `~/.skill-manager`, Claude/Codex user skill roots,
  Codex system skills, or the Claude plugin cache in either direction.

## Discovery And Install

- A source root containing a regular, non-symlink `SKILL.md` is exactly one
  skill; nested skills are not scanned in that case.
- Otherwise discovery is recursive and shares Git install's ignored-directory,
  duplicate-basename, selection, target-tool, and target-conflict rules.
- Install creates direct symlinks from the selected Claude/Codex user skill
  cells to directories under the canonical source. It never copies or clones
  the source.
- An exact pre-existing matching link may be adopted. Reinstalling a recorded
  source is idempotent and may add newly discovered skills.
- One manifest source owns each skill/tool cell. Git and other local sources
  cannot take over a recorded cell even when its active path has drifted.
- Apply backs up existing state, revalidates source identity and ownership,
  rolls back links created by a failed apply, then persists ownership.

The desktop Sources screen obtains the source through the native macOS folder
picker, then exposes the same discovery as an exact per-skill Claude/Codex
matrix. Review and Apply remain backend-owned and use opaque draft/review IDs;
the frontend never submits a local filesystem path for mutation.

## State And Scan

Manifest version 2 adds `localSources`. Each record contains original and
canonical source paths, group, install time, and installed skill relative
paths/tools. Scanning classifies an active symlink as `local path` only when
the manifest cell and exact target agree; ordinary local directories remain
source/group `local`.

`repos` remains Git-only and there is no local-source list command or TUI source
management action. The GUI Sources table is the manifest-backed local-source
summary.

## Update And Uninstall

- Local sources are link-in-place, so targeted `update <local-path>` reports
  that no update is required. Update-all considers only recorded Git repos.
- `uninstall <local-path>` audits the complete recorded source, including
  active and disabled links. Changed, missing, duplicate, unrecorded, or extra
  references block removal, except that links broken by a missing source remain
  safely identifiable.
- Exact links are staged under `trash/uninstall-local-*`; matching disabled
  records and the local-source record are removed; staging is then deleted.
- The source itself is never staged or deleted. An unrelated active blocker for
  an OFF skill is preserved.
- Pre-save failures roll links back. Incomplete rollback or post-save cleanup
  reports the retained recovery path.
- The GUI exposes the same audit as a preview and requires typing the exact
  group name before apply; it explicitly reports that the source is preserved.

## Dry-Run

Local install and uninstall dry-runs perform resolution, discovery, selection,
ownership audit, and preflight without creating/removing links or writing
state. Unlike a missing Git checkout dry-run, local install discovery is
available because the source must already exist.

See [repository-install-workflow.md](repository-install-workflow.md) for the
separate managed-checkout lifecycle and
[state-safety-and-recovery.md](state-safety-and-recovery.md) for persistence and
recovery guarantees.
