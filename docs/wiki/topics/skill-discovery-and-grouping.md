# Skill Discovery And Grouping

## Discovery Sets

`Managed and toggleable`

- Claude user skills: `~/.claude/skills`.
- Codex user skills: `~/.agents/skills`.
- Disabled entries recorded in `~/.skill-manager/state.json` and stored under
  `~/.skill-manager/disabled/<tool>/`.

`Read-only`

- Codex system skills: `~/.codex/skills/.system`.
- Claude plugin cache skills below `~/.claude/plugins/cache`.

Read-only sources may be displayed but never produce enable/disable operations.

## Validity And Metadata

- A scanned entry is visible only when it resolves to a regular `SKILL.md`.
- Direct user-skill entries preserve whether the entry itself is a symlink or
  directory.
- `SKILL.md` frontmatter parsing is opportunistic: name and description improve
  display, but missing or malformed metadata does not block a valid skill.
- `~/.agents/.skill-lock.json` is read only to recognize Skills CLI installs.
- Git origin, commit, and repository root are collected when available; failure
  to collect them is non-fatal.

## Source Versus Group

- `Source` describes the installation mechanism: `symlink repo`, `Skills CLI`,
  `local path`, `local`, `Codex system`, `Claude plugin`, or `unknown`.
- `Group` describes the collection: a repository label such as `example-labs/engineering-skills`,
  a link-in-place source basename, `Skills CLI`, `local`, `Codex system`,
  `Claude plugin`, or `unknown`.

For GitHub remotes, groups use `owner/repo`. Other remotes use a stable
remote-derived label when possible, and repositories without a remote fall back
to their root directory name. Manifest-owned local links use the canonical
source root basename; direct unmanaged directories keep the generic `local`
group.

## Row Assembly

- One `SkillRow` represents one skill basename and may contain a Claude cell, a
  Codex cell, or both.
- A one-sided skill is valid; a missing cell is not an invitation to create a
  skill for the other tool.
- Compatible cell groups merge into the row group. Mixed incompatible groups
  conservatively become `unknown`.
- The CLI `list` table exposes the row source. The TUI table exposes the row
  group and keeps source details in its details panel.

## Safety Consequences

- Invalid directories without `SKILL.md` are hidden rather than offered for
  cleanup.
- Unknown classification never blocks listing, but mutation still requires a
  valid managed entry or disabled manifest record.
- Scanner code is observational. Filesystem changes belong to the toggle or
  install apply services.

## Skills CLI Interoperability

`observed (2026-08-14)`: the upstream `skills add -g` workflow copies the
selected skill into the canonical `~/.agents/skills/<name>` directory and, for
non-universal agents, normally links an agent-specific skill path to that copy.
It records source/update metadata in `~/.agents/.skill-lock.json` (or the XDG
state equivalent). Skill Manager reads that lock only for source labeling.

The upstream installer copies the files contained in the selected skill, but
does not install runtimes or project dependencies described by its
instructions. `npx` separately downloads the `skills` CLI and its own Node
dependencies into the npm execution cache when they are not already available.
The upstream CLI also sends install telemetry for eligible public sources by
default; `DISABLE_TELEMETRY=1` disables it.
