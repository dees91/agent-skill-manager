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
installed skill names, timestamps, and commits. Advisor state additionally
stores opaque receipt IDs, tool/skill claims, and exact restore fingerprints;
it never stores prompt or task content. Saved Skill Sets and favorites store
only skill basenames plus their feature-specific recipe metadata; favorites do
not store tools, source paths, prompts, task text, or usage history.

State directories are restricted to the current user (`0700`). State, backup,
and catalog-cache JSON files use mode `0600`. When a new state backup is made,
rotation retains at most ten copies and removes copies older than 30 days.
Managed Git repositories are cloned
beneath `~/.skill-manager/repos`; their repository file modes are preserved.

The public preview does not expose Discover or make skills.sh requests. If a
cache from an earlier development build exists at
`~/.skill-manager/cache/skills-sh/catalog-v1.json`, the next desktop launch
removes cached search terms/results and upgrades the cache format. Ranking and
detail metadata may remain until the user deletes the cache or state directory.

## Network Access

Network access occurs in this user-visible flow:

- Git install and update invoke the local `git` executable for a repository URL
  selected by the user.

Skill Manager does not send the local skill inventory, home path, state
manifest, or provider configuration to a Skill Manager-operated service.

## Local Provider Diagnostics

The context-budget panel uses local filesystem estimates by default. Only the
explicit **Run provider diagnostics** action may run these installed commands:

- `codex debug prompt-input`
- `codex debug prompt-input -c model_context_window=100000000`
- `claude plugin list --json`

They run from a neutral temporary directory with the current user's home and a
minimal environment containing only path, temporary-directory, locale,
terminal, and no-color variables. Provider config overrides, credentials,
tokens, and proxy variables are not forwarded. Output is bounded, processed in
memory, and not retained or sent by Skill Manager. The provider executables are
third-party software, so their own behavior and privacy terms still apply.
Failure is non-fatal and falls back to the filesystem estimate.

## Retention And Removal

Uninstalling a source removes the state and links owned by that source under the
documented safety checks. It does not remove link-in-place local source folders.
Saved Skill Set and favorite basenames are retained so they can reconnect after
the same skill name is installed again.
To remove all remaining Skill Manager metadata after restoring or uninstalling
managed entries, the user may manually archive and delete `~/.skill-manager`.
Deleting `~/.skill-manager/cache/skills-sh/` removes only dormant catalog cache
data and does not uninstall skills.
