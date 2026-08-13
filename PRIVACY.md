# Privacy

Skill Manager is a local application. It has no account system, telemetry,
analytics, crash reporting, advertising, or background polling.

## Local Data Read

The application scans the following locations under the current user's home:

- `~/.claude/skills`
- `~/.agents/skills`
- `~/.codex/skills/.system`
- `~/.claude/plugins/cache`
- `~/.agents/.skill-lock.json`
- `~/.skill-manager`

It may read skill metadata, symlink targets, Git origins and commits, provider
settings needed for context-budget estimates, and the Skill Manager manifest.
The desktop interface can display those local paths and source details to the
current user.

## Local Data Written

Skill Manager writes only inside its state directory and the provider user-skill
directories involved in an explicit operation. State can include original and
disabled paths, symlink targets, repository URLs, canonical local source paths,
installed skill names, timestamps, and commits.

Normalized skills.sh catalog metadata is cached at
`~/.skill-manager/cache/skills-sh/catalog-v1.json`. Raw downloaded skill files
are not retained in that cache. Managed Git repositories are cloned beneath
`~/.skill-manager/repos`.

## Network Access

Network access occurs in these user-visible flows:

- Git install and update invoke the local `git` executable for a repository URL
  selected by the user.
- Discover sends anonymous JSON `GET` requests to `https://www.skills.sh` for
  rankings, search terms, and skill detail. A fresh detail response is required
  before a Discover installation.
- Opening an external catalog link asks the operating system to open the
  corresponding skills.sh page.

Skill Manager does not send the local skill inventory, home path, state
manifest, or provider configuration to a Skill Manager-operated service.

## Local Provider Diagnostics

The context-budget panel may run an installed `codex` executable with fixed,
read-only diagnostic arguments from a neutral temporary directory. The process
uses the current user's home while explicit `CODEX_HOME` and
`CLAUDE_CONFIG_DIR` overrides are removed. Diagnostic failure is non-fatal and
falls back to local filesystem estimates.

## Retention And Removal

Uninstalling a source removes the state and links owned by that source under the
documented safety checks. It does not remove link-in-place local source folders.
To remove all remaining Skill Manager metadata after restoring or uninstalling
managed entries, the user may manually archive and delete `~/.skill-manager`.
