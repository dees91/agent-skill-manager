# Skill Manager Agent Brief

## Project Intent

`skill-manager` is a local tool for managing globally installed agent skills for Claude Code and Codex. Its established terminal interfaces remain the primary source-management surface, with a macOS desktop interface for visibility and reversible toggles.

The first version is a local, simple, reversible TUI plus a minimal CLI. It must make it easy to see which skills are enabled for each tool and toggle them off and on without losing their original installation method.

This repository is intentionally greenfield. The planning discussion happened before implementation. Preserve these decisions unless the user explicitly changes them.

## Mandatory Wiki Routing

This project has an LLM-maintained wiki under `docs/wiki/`.

Before starting non-trivial work in this repository:

1. Read `docs/wiki/README.md` for the wiki contract, source precedence, and workflows.
2. Read `docs/wiki/index.md`, then open the topic pages relevant to the task.
3. Verify important or mutable claims against the authoritative project files; the wiki is a synthesis layer, not a replacement for this file, the planning documents, code, or tests.

After work that produces a reusable conclusion, correction, decision, implementation insight, or open item, update the relevant `docs/wiki/topics/` or `docs/wiki/sources/` pages, update `docs/wiki/index.md` when pages change structurally, and append an entry to `docs/wiki/log.md` using `## [YYYY-MM-DD] type | Short title`.

If the user accepts a change to product scope, semantics, paths, or UX, update the authoritative files required by the Documentation Rules below before bringing the wiki synthesis up to date.

## Current Target Environment

The supported source release targets Apple Silicon macOS and the standard
global skill layout:

- Claude Code user skills: `~/.claude/skills`
- Codex user skills: `~/.agents/skills`
- Codex system skills: `~/.codex/skills/.system`
- Claude plugin cache: `~/.claude/plugins/cache`
- Skill Manager state: `~/.skill-manager`
- Skill Manager managed repo checkouts: `~/.skill-manager/repos`

For MVP, use these paths from `$HOME` directly. Do not add config file support in the first iteration.

## Discovery Model

The application must not depend on a captured machine inventory. It scans the
filesystem each run and supports any valid skills found under the fixed paths.
Tests, demo data, screenshots, and documentation examples use synthetic homes,
repositories, skill names, and counts.

## Core Product Decisions

### Toggle Semantics

Disable means the skill disappears from the directory scanned by Claude Code or Codex.

Enable means the exact previous entry is restored.

The tool must preserve:

- Whether the original entry was a symlink or directory
- The symlink target, if any
- The original tool-specific path
- The source classification, when detectable

Enable/disable operations must not:

- Delete source repositories
- Edit `SKILL.md`
- Modify git checkout contents
- Rewrite Skills CLI lockfiles
- Rewrite Claude plugin cache contents

### Managed vs Read-Only Sources

Managed and toggleable in MVP:

- Claude Code user skills in `~/.claude/skills`
- Codex user skills in `~/.agents/skills`

Read-only in MVP:

- Codex system skills in `~/.codex/skills/.system`
- Claude plugin skills in `~/.claude/plugins/cache`
- Any plugin marketplace checkout skills

Plugin enable/disable is out of scope for MVP. A future `Plugins` screen may toggle whole Claude plugins through `~/.claude/settings.json.enabledPlugins`, but the first `Skills` screen must not do that.

### Skills CLI Installs

Skills installed through `npx skills` are in scope if they are present under `~/.agents/skills`.

Use `~/.agents/.skill-lock.json` only for source labeling, not as the control surface. Do not edit the lockfile during enable/disable.

Tests and documentation should use synthetic Skills CLI entries such as
`catalog-search` from `example-labs/skills`.

### Invalid Skill Directories

Entries without `SKILL.md` are hidden in MVP.

Do not show or manage `~/.claude/skills/invalid-entry` unless a future plan explicitly adds invalid-entry cleanup.

### Source Labels

The UI should classify skills using source details and a source filter.

Expected source labels:

- `symlink repo`
- `Skills CLI`
- `local`
- `Codex system`
- `Claude plugin`

If a source cannot be determined, use a conservative `unknown` label and do not block listing.

### Groups

`Group` is a first-class display and bulk-toggle concept.

A group is an automatically detected source package, repository, or collection. It is not manually configured in MVP2.

Expected group labels:

- GitHub repo symlinks: `owner/repo`
  - `https://github.com/example-labs/engineering-skills.git` -> `example-labs/engineering-skills`
  - `https://github.com/sample-org/product-skills.git` -> `sample-org/product-skills`
- Non-GitHub git remotes: best stable remote-derived label, falling back to repo root directory name
- Git repo with no remote: repo root directory name
- Skills CLI installs: `Skills CLI`
- Link-in-place local path installs: canonical source root basename
- Local managed directories: `local`
- Codex system read-only skills: `Codex system`
- Claude plugin read-only skills: `Claude plugin`
- Unknown: `unknown`

`Group` and `Source` are different:

- `Source` describes the installation mechanism, e.g. `symlink repo`, `Skills CLI`, `local path`, `local`.
- `Group` describes the collection the skill belongs to, e.g. `example-labs/engineering-skills`, `sample-org/product-skills`.

The main TUI table uses `Group` instead of `Source`. Details still show both `Group` and `Source`.

### Repository Install (Iteration 3)

Iteration 3 adds CLI-first installation from Git repositories. Do not add a TUI install screen in this iteration.

Install source and layout:

- `skill-manager install <git-url>` clones Git repositories into `~/.skill-manager/repos/<host>/<repo-path>`.
- Example: `https://github.com/addyosmani/agent-skills` clones into `~/.skill-manager/repos/github.com/addyosmani/agent-skills`.
- Installed skills are represented in Claude/Codex user skill directories as symlinks to skill directories inside the managed checkout.
- Moving or toggling those symlinks must not modify the managed checkout.

Supported repository inputs for Iteration 3:

- HTTPS Git URLs, with or without `.git`.
- SSH Git URLs such as `git@github.com:addyosmani/agent-skills.git`.

Out of scope for Iteration 3:

- Branch, tag, or commit selection.
- Local path installs.
- GitHub shorthand such as `addyosmani/agent-skills`.
- Submodules, sparse checkout, and update operations.
- Uninstall operations.
- `--force` or overwrite behavior.
- Mutating group/all CLI commands.

Future repository-management work is planned but intentionally deferred:

- `skill-manager install --branch`
- `skill-manager install --force`

Tool targeting:

- Default target is `both`.
- `--tool claude`, `--tool codex`, and `--tool both` are accepted.
- `--skill <name>` may be repeated to install selected skills only.
- If no `--skill` is provided, install all valid skills discovered in the repository.

Repository reuse:

- If the checkout does not exist, clone it.
- If the checkout exists and its remote matches the requested URL, reuse it without `git pull`.
- If the checkout exists but its remote does not match, fail as a conflict.
- `install` must not update an existing checkout. Repository changes use the separate explicit `update` command from Iteration 4.

Skill discovery:

- A valid installable skill is any directory containing `SKILL.md`.
- Discovery is recursive inside the checkout.
- Ignore heavy or generated directories such as `.git`, `node_modules`, `.venv`, `vendor`, `build`, and `dist`.
- Skill name is the basename of the directory containing `SKILL.md`.
- If two discovered skills have the same basename, fail preflight with a duplicate-name conflict.
- Invalid directories without `SKILL.md` are skipped.

Conflict and idempotency rules:

- Preflight all selected skill/tool symlink targets before creating any symlink.
- If the target path is free, it may be linked.
- If the active target path already is a symlink to the same repo skill directory, treat it as already installed and OK.
- If a disabled state entry exists for the same tool and skill and points to the same repo skill directory, treat it as already installed but currently OFF. Do not implicitly enable it.
- If the tool/skill cell is recorded as owned by another Git or local source, fail preflight even when the active path is free or happens to match.
- If the target path exists and is anything else, fail before creating new symlinks.
- Do not overwrite, merge, delete, or rename blockers.

Install apply behavior:

- After a clean preflight, create symlinks for selected skills and tools.
- If symlink creation fails after some links were created, remove only the links created by that failed install operation.
- Do not remove a checkout created earlier in the operation. Keeping the clone makes retry cheaper and safer.
- Do not write a partial install manifest when symlink creation fails.

Dry-run behavior:

- `install --dry-run` is strict and must not clone repositories.
- If the repo is not already checked out locally, print that the repo would be cloned and that skill discovery is unavailable until real install.
- If the repo is already checked out locally, discovery and preflight may run against the local checkout and print planned symlinks/conflicts.
- Dry-run must not create symlinks or mutate `state.json`.

State and lockfiles:

- Store repository installation metadata in `~/.skill-manager/state.json`.
- Do not edit `.agents/.skill-lock.json`, Claude plugin metadata, Skills CLI lockfiles, or any external manager lockfiles.
- Use the Skill Manager manifest as the source of truth for repositories installed by `skill-manager`.

Runtime visibility:

- `install` changes filesystem state for future Claude/Codex runs.
- Already running Claude/Codex sessions may not see newly installed skills. Documentation should tell users to start a new session for guaranteed detection.

### Repository Update and Uninstall (Iteration 4)

Iteration 4 adds CLI-first update and whole-repository uninstall. Do not add TUI repository actions in this iteration.

Update commands:

- `skill-manager update [<git-url>] [--dry-run]`
- Without a URL, update every repository recorded in Skill Manager state in deterministic host/repo-path order.
- With a URL, normalize HTTPS/SSH forms and update the matching recorded repository only.
- Stop on the first failure and keep earlier successful repository updates reflected in state.

Update safety:

- Require the managed checkout, matching origin, a normal current branch tracking `origin/*`, no local-only commits or divergence, and no tracked, untracked, or ignored worktree changes.
- Audit every recorded active/disabled skill symlink and reject missing, changed, duplicate, or extra managed-directory references into the checkout.
- Fetch `origin`, require a fast-forward, and verify each installed relative skill path still contains a regular `SKILL.md` in the target commit before merging.
- Do not auto-install newly discovered skills. Do not move or rewrite existing active/disabled symlinks during update.
- Persist the successful target commit as `lastSeenCommit`.
- Update dry-run must not fetch or change remote-tracking refs, checkout files, symlinks, or state. It must state that exact remote preflight is unavailable until real update.

Uninstall commands and safety:

- `skill-manager uninstall <git-url> [--dry-run]` removes the entire recorded repository installation. An explicit URL is always required.
- Remove only exact validated active/disabled skill symlinks owned by the repository, the matching disabled records, the repository manifest entry, and the managed checkout.
- Require a clean checkout with a recoverable `origin/*` upstream and no local-only commits. Block on missing/changed expected symlinks or extra managed-directory symlinks into the checkout.
- An unrelated blocker at the original path of an OFF skill is not owned by Skill Manager and must be left untouched.
- Stage symlinks and checkout under `~/.skill-manager/trash/`, save state, then delete staging. Roll back paths when a failure happens before state save; report any post-save cleanup residue explicitly.
- Do not add `--force`, uninstall-all, checkout archival, or a separate `repo remove` command. Whole-repository `uninstall` supersedes `repo remove`.
- Update and uninstall dry-runs must not mutate checkout, symlinks, Git refs, or `state.json`.
- Already running Claude/Codex sessions may retain old skill visibility; recommend a new session after update or uninstall.

### Local Path Install and Uninstall (Iteration 5)

Iteration 5 adds CLI-first link-in-place installation from local folders. Do not add TUI source-management actions in this iteration.

Local install commands and discovery:

- `skill-manager install <local-path> [--tool claude|codex|both] [--skill name...] [--dry-run]`
- Accept absolute paths, explicit relative paths (`./` and `../`), `~/` paths resolved from `$HOME`, and bare relative paths only when they currently exist.
- Resolve the source to a canonical absolute directory path. A root containing a regular, non-symlinked `SKILL.md` is exactly one skill; otherwise recursively discover skills with the existing ignored-directory and duplicate-name rules.
- Use link-in-place symlinks directly to skill directories. Never copy, move, edit, update, or delete the local source.
- Reuse Git install tool targeting, skill selection, all-or-nothing preflight, idempotency, blocker handling, apply rollback, and strict dry-run semantics.
- Reject local roots that overlap in either direction with Skill Manager state, Claude/Codex user skill paths, disabled paths, Codex system skills, or Claude plugin cache paths.
- Use source label `local path` and the canonical source root basename as Group.
- Treat canonical source path as identity. Reinstalling the same source may add newly discovered skills, but existing recorded ownership drift blocks the operation.
- A tool/skill cell may be owned by only one Skill Manager Git or local source. An exact matching unmanaged symlink may be adopted.

Local uninstall and update behavior:

- `skill-manager uninstall <local-path> [--dry-run]` removes the whole recorded local installation: exact validated active/disabled links, matching disabled records, and the local-source state entry.
- Never stage or remove the source directory. An unrelated blocker at an OFF skill's active path remains untouched.
- Permit uninstall after the source is missing by validating stored relative targets against exact active/disabled symlink text. Reject changed expected links and extra managed-directory symlinks into the recorded source root.
- Stage owned links under `~/.skill-manager/trash/`, save state, then delete staging. Roll back before-save failures and retain/report recovery data after incomplete rollback.
- Update-all processes only Git repositories. `skill-manager update <local-path>` reports that link-in-place sources are live and do not require update.
- `repos` remains a Git repository summary. Do not add a local-source summary command in this iteration.
- Already running Claude/Codex sessions may retain old skill visibility; recommend a new session after local install or uninstall.

### State Location

Use a global state directory:

```text
~/.skill-manager/
  state.json
  skill-sets.json
  backups/
  disabled/
    claude/
      <skill-name>
    codex/
      <skill-name>
  repos/
    <host>/
      <owner-or-path>/
        <repo>
  trash/
    uninstall-<operation-id>/
```

`state.json` stores enough information to restore disabled entries:

- tool: `claude` or `codex`
- skill name
- original path
- disabled path
- entry type: `symlink` or `dir`
- symlink target, if any
- source label
- group label
- timestamp

`state.json` also stores repository installation metadata for Skill Manager managed installs:

- original URL
- normalized host and repo path
- checkout path
- group label
- installed skills with name, relative path, and tools
- install timestamp
- last seen commit, if available

Manifest version 2 also stores local-source installation metadata:

- original normalized absolute input path
- canonical absolute source path
- group label
- installed skills with name, relative path, and tools
- install timestamp

Version 1 manifests migrate in memory with no local sources and are written as version 2 on the next mutation. Reject unknown newer manifest versions.

If `state.json` is missing, later versions may recover from `disabled/`, but MVP does not need full disaster recovery.

### File Operations

Disable operation:

1. Validate original entry exists.
2. Validate destination under `~/.skill-manager/disabled/<tool>/` is free.
3. Move the entry with `rename`.
4. Update `state.json`.
5. Rescan.

Enable operation:

1. Validate disabled entry exists.
2. Validate original target path is free.
3. Move the entry back with `rename`.
4. Update `state.json`.
5. Rescan.

For symlinks, moving the symlink itself is correct. Do not dereference it.

Before the first apply in a process, write a backup copy of the current state manifest to `~/.skill-manager/backups/state-<timestamp>.json` if the manifest exists.

Install operation:

1. Normalize and validate the Git URL.
2. Resolve the managed checkout path under `~/.skill-manager/repos`.
3. For non-dry-run install, clone missing checkouts or reuse matching existing checkouts without pulling.
4. Discover valid skills from the checkout.
5. Apply optional `--skill` selection and fail if any requested skill is missing.
6. Preflight all selected skill/tool targets for conflicts and idempotent already-installed cases.
7. Create missing symlinks.
8. If symlink creation fails, remove only symlinks created by this install operation.
9. Update the install manifest in `state.json` only after successful symlink creation.
10. Rescan.

Local install follows the same discovery/preflight/link/state sequence without a clone step. Local uninstall stages only exact owned symlinks; it never stages or deletes its source directory.

### Conflicts

If restore would overwrite an existing file, directory, or symlink, block the restore.

Do not auto-overwrite. Do not auto-merge. Do not auto-delete.

The TUI should show `CONFLICT` for the affected cell and details should explain:

- Original path
- Disabled path
- Existing blocker path
- Suggested manual resolution

Conflict resolution actions are out of scope for MVP.

### Batch Apply

The TUI uses pending changes.

`Space` creates or removes a pending toggle. Changes are not applied immediately.

Apply order must be deterministic:

1. Disable operations first
2. Enable operations second
3. Within each group, sort by tool, then skill name

If a batch operation fails, stop the batch, keep already completed operations reflected in state, and show the error. Full transactional rollback is out of scope for MVP.

## TUI Decisions

Build the TUI in Go using Bubble Tea.

The main view is one row per skill, with separate Claude and Codex state columns.

Example:

```text
Skill                                      Claude  Codex  Group
release-checklist                          ON      ON     example-labs/engineering-skills
catalog-search                             -       ON     Skills CLI
local-only-skill                                   ON      -      local
imagegen                                   RO      RO     Codex system
codex-cli-runtime                          RO      -      Claude plugin
```

Default view:

- Show only toggleable skills from `~/.claude/skills` and `~/.agents/skills`.
- Hide read-only skills by default.

Read-only view:

- Show or hide with a key such as `o`.
- Show Codex system skills and Claude plugin skills.

Expected controls:

- `Tab`: switch active tool column between Claude and Codex
- `Space`: toggle pending state for the active cell
- `b`: toggle both tools where possible
- `g`: smart-toggle the selected row's group for both tools
- `A`: smart-toggle all visible toggleable rows for both tools
- `G`: cycle group filter
- `a` or `Enter`: apply pending changes
- `u`: undo pending change for active cell
- `U`: clear all pending changes
- `/`: text filter
- `s`: source filter
- `o`: show/hide read-only entries
- `d`: details panel
- `r`: rescan from disk
- `q`: quit, warning if pending changes exist

The UI language is English.

Details panel is read-only in MVP and should show:

- Skill name
- Description from `SKILL.md`, if easy to parse
- Group label
- Claude active path and state
- Codex active path and state
- Disabled path, if off
- Entry type
- Symlink target, if any
- Source label
- Repo origin and commit, if detectable
- Conflict details, if any

### Group and Bulk Toggle Semantics

Group and all-visible toggles must use the same pending-change safety model as single-cell toggles. They must never apply immediately.

Keys:

- `g`: smart-toggle every toggleable cell in the selected row's group, across both Claude and Codex.
- `A`: smart-toggle every visible toggleable cell across both Claude and Codex.
- `G`: cycle group filter for the current table.

Scope:

- `g` uses all currently loaded rows that belong to the selected row's group, not only rows currently visible after text/source/group filters. If a group filter is active, this is naturally the same visible group.
- `A` acts on the current visible rows after text/source/group/read-only filters.
- Read-only cells are always skipped.
- Missing cells with no disabled restore state are skipped.
- One-sided skills are valid: `local-only-skill` toggles only Claude, while
  `catalog-search` and `decision-review` toggle only Codex.
- Conflicts are skipped and reported in the status message.

Smart-toggle target:

- If every toggleable, non-conflict cell in the scope is effectively `ON`, add pending disables for the scope.
- Otherwise, add pending enables for cells that are `OFF`.
- For mixed scopes, do not create missing entries for tools where a skill never existed.

Pending merge behavior:

- If a cell already has the same pending operation, batch toggle removes that pending operation.
- If a cell has the opposite pending operation, batch toggle replaces it.
- If every applicable cell already has the same pending operation, the batch toggle acts as batch undo and removes those pending operations.

Status messages must report how many pending changes were updated and how many cells were skipped for read-only/missing/conflict reasons.

## CLI Decisions

The binary name is `skill-manager`.

Minimal commands:

```bash
skill-manager tui
skill-manager list
skill-manager status
skill-manager groups
skill-manager disable --tool claude release-checklist
skill-manager enable --tool codex catalog-search
skill-manager disable --tool claude release-checklist --dry-run
```

`go run .` should launch the TUI by default or behave the same as `skill-manager tui`.

`--dry-run` support is required for CLI mutating commands. It prints the planned filesystem operations without moving anything.

`skill-manager groups` is a read-only command that summarizes group labels, row counts, sources, and Claude/Codex state counts. Do not add mutating group/all CLI commands in Iteration 2.

Iteration 3 repository install commands:

```bash
skill-manager install <git-url> [--tool claude|codex|both] [--skill name...] [--dry-run]
skill-manager repos
```

`skill-manager install` installs skills from a Git repository using managed checkouts and symlinks. `--tool` defaults to `both`. `--skill` may be repeated. `--dry-run` is strict and does not clone missing repositories.

`skill-manager repos` is read-only and summarizes repositories recorded in Skill Manager state, including group, URL, checkout path, last seen commit, installed skill count, and tools.

Iteration 4 repository management commands:

```bash
skill-manager update [<git-url>] [--dry-run]
skill-manager uninstall <git-url> [--dry-run]
```

`update` without a URL updates all managed repositories. `uninstall` always requires an explicit URL and removes the full recorded repository installation and checkout. Both commands use the strict safety and dry-run rules above.

Iteration 5 local path commands:

```bash
skill-manager install <local-path> [--tool claude|codex|both] [--skill name...] [--dry-run]
skill-manager uninstall <local-path> [--dry-run]
```

Local installs are live symlinks and are not included in repository update or `repos` output.

## Build and Distribution

MVP is local-only:

```bash
go run .
go build -o bin/skill-manager .
make dev
```

Use `make dev` after code changes to rebuild the persistent global command at `~/.local/bin/skill-manager`.

Do not add Homebrew, installer, auto-update, or release packaging in the first iteration.

## macOS GUI (Iteration 6)

Iteration 6 adds a local Apple Silicon desktop app while preserving the CLI and
TUI contracts above.

- Use Wails 2.14 in a nested `desktop` Go module and call the existing Go
  backend in-process. Do not duplicate filesystem mutation logic in JavaScript
  or another backend language.
- Use React, TypeScript, Vite, npm, generated Wails bindings, and plain CSS
  custom properties. Keep exact dependency resolution in `package-lock.json`.
- The GUI contains `Dashboard` and `Skills`. Git/local install, update, and
  uninstall remain CLI-only in this iteration.
- The GUI uses the same pending, smart-toggle, conflict, deterministic apply,
  backup, and partial-failure semantics as the TUI. Pending state is process
  local and closing with pending changes must warn.
- Frontend calls identify skills and tools only. They must never accept or send
  arbitrary filesystem paths or prebuilt move operations.
- Scan on app launch, after an app-owned Apply, and on explicit Refresh. Do not
  add a watcher or polling loop.
- The visual source of truth is the validated root `DESIGN.md`, maintained from
  the implemented interface and repository-owned screenshots under
  `docs/images/`. The first GUI is dark-only.
- Build a local ad-hoc signed `darwin/arm64` `.app`. Developer ID signing,
  notarization, light theme, universal binaries, publishing, telemetry, and
  auto-update are out of scope. Wails' ad-hoc signature is retained because
  it is required for reliable launch on supported macOS releases.

### GUI Source Management (Iteration 8)

Iteration 8 adds source lifecycle actions to the existing macOS app while
preserving the CLI contracts and the ownership/safety rules above.

- Add a `Sources` screen containing only Git repositories and local sources
  recorded in Skill Manager state. Unmanaged groups remain visible in Skills
  and are not presented as owned sources.
- Install uses an explicit inspect, review, and apply flow. Git inspection may
  clone a missing managed checkout; cancelling after that point retains the
  clean unrecorded checkout for a cheaper retry. Local sources are selected
  through the native macOS directory picker.
- Installation selection is a per-skill Claude/Codex matrix. Extend the shared
  domain planner to accept exact cells while preserving the existing CLI
  `--tool` and `--skill` expansion semantics.
- Each install-matrix tool column has one explicit bulk-selection toggle. It
  reports `ON`, `OFF`, `MIXED`, or `N/A`; `ON` clears every non-conflict target
  in that column, while `OFF` or `MIXED` selects all of them. Its scope is every
  discovered skill, including rows hidden by the text filter. This changes only
  the install selection and never toggles visibility of an installed skill.
- Update is available per Git repository and for all recorded repositories.
  Local link-in-place sources remain live and do not require update.
- The Sources table labels this distinction as `Update mode`: `Managed Git`
  with `Use Update to fetch changes.`, or `Linked folder` with `Changes are
  read directly; no update needed.` Do not use ambiguous standalone states
  such as `Ready` and `Live` for this distinction.
- Uninstall removes a whole recorded source and requires typing its group name
  in the confirmation dialog. Git uninstall removes the managed checkout;
  local uninstall always preserves the user-owned source directory.
- Source actions are immediate, separately confirmed operations. They are
  blocked while Skills toggles are pending, and no other mutation or app close
  is allowed while a source operation is active.
- Frontend mutation calls use opaque draft/review/source identifiers plus skill
  and tool names. They never accept filesystem paths or prebuilt operations.
  A Git URL is the only raw source locator accepted from the frontend.
- Long source operations expose phase progress but are not cancellable in this
  iteration. Keep the existing rollback and cleanup reporting semantics.
- The `Discover` mode backed by `skills.sh` is delivered separately in
  Iteration 9. It does not change the managed-only scope of this screen.

### skills.sh Discover (Iteration 9)

Iteration 9 adds an experimental, anonymous catalog integration to the macOS
app. It does not replace the established CLI/TUI source-management contracts.

- Add a top-level `Discover` screen with all-time, trending, and hot rankings,
  debounced search, progressive loading, details, and compact activity history.
- Isolate the observed anonymous skills.sh JSON endpoints behind one Go adapter.
  Do not use OIDC, scrape HTML, or silently switch to scraped responses.
- Cache normalized ranking/search pages and parsed detail metadata under
  `~/.skill-manager/cache/skills-sh/catalog-v1.json`. Cached data may be browsed
  offline, but raw downloaded skill files must not be cached.
- Installation always requires a fresh live detail response and a distinct
  safety confirmation. Explain that third-party skill instructions may be
  unsafe and should be reviewed before use.
- Install exactly one selected GitHub-hosted skill for the selected Claude and/or
  Codex cells. Reuse the existing managed checkout, ownership, exact-cell
  planner, preflight, apply, backup, rollback, progress, and rescan machinery.
- Frontend mutation calls use only catalog session identifiers, skill
  identifiers, and tool names. They must not send repository URLs, skill paths,
  checkout paths, or prebuilt filesystem operations.
- Project every catalog entry against live local state as `available`,
  `installed-on`, `installed-off`, or `conflict` per tool. Individual-skill
  uninstall remains out of scope; installed entries link back to `Skills`.
- Catalog entries from well-known/non-GitHub sources remain visible and
  external-linkable but cannot be installed in this iteration.
- Catalog network failures do not block Dashboard, Skills, Sources, or local
  mutations. Offline cache state must be explicit and installation must remain
  disabled until live revalidation succeeds.
- Do not add telemetry, ratings, comments, publishing, or automatic background
  polling.

### Active-First Skills Workspace (Iteration 10)

Iteration 10 restructures only the macOS Skills screen for large catalogs where
most managed skills remain OFF until a specific task needs them. CLI, TUI,
filesystem, state, and Apply semantics remain unchanged.

- Show `Needs attention` first when conflicts exist, then an always-expanded
  `Active now` section, followed by OFF skills in collapsed `Available by
  source` group accordions. Show read-only skills in their own grouped section
  only after explicit opt-in. A skill appears in exactly one section.
- Determine section placement from applied state. A pending enable or disable
  remains in its current section and shows its transition until successful
  Apply/rescan.
- Keep global search plus `All / Active / Available` and `All tools / Claude /
  Codex` chips visible. Move Group, Source, and Read only controls into an
  advanced `Filters` disclosure.
- Keep both tool columns visible. A selected tool chip changes classification
  and limits row, group, and filtered-result smart-toggle actions to that tool.
- The global action covers every filtered result, including rows inside
  collapsed groups. A group-header action covers the complete loaded group,
  independent of search and filters. Both report eligible cell counts and use
  the existing pending-change safety model.
- Search temporarily expands matching groups and restores the session's manual
  expansion state when cleared. Manual group expansion survives navigation
  during the current app process but is not persisted across restarts.
- Remove the repeated Group column and per-row group action. Active rows retain
  a compact group/source badge; available groups own their bulk action.
- Expose scoped Wails methods using only skill/group and tool identifiers. Go
  validates the scope and reuses `staging.ToggleBatch`; frontend code never
  submits paths or prebuilt operations.

### Public Source Distribution (Iteration 11)

Iteration 11 prepares the source tree for a public GitHub repository without
changing runtime skill-management semantics.

- Use Go modules under `github.com/dees91/agent-skill-manager` and bundle identifiers
  under `io.github.dees91.skillmanager`.
- Publish under MIT with practical security, privacy, and contribution docs.
- Keep public examples, test fixtures, screenshots, plans, and wiki synthesis
  free of real home inventories, private paths, machine fingerprints, and
  credentials.
- Build the ignored frontend `dist` before compiling the embedded desktop
  module so a clean checkout is sufficient.
- GitHub Actions remain out of scope for this iteration.
- Keep the public repository source-only for Apple Silicon macOS. Universal
  binaries, notarization, release automation, configurable provider paths, and
  supported non-macOS behavior remain deferred.

### GitHub Binary Distribution (Iteration 12)

Iteration 12 adds manually published GitHub Release artifacts without changing
runtime skill-management semantics.

- Build and verify releases locally. Do not add GitHub Actions or automatic
  publishing in this iteration.
- Publish `v0.4.0` as a prerelease in the private
  `dees91/agent-skill-manager` repository.
- Package the ad-hoc-signed Apple Silicon macOS app as ZIP and the
  `darwin/arm64` CLI as `tar.gz`; publish one SHA-256 manifest for both.
- The CLI exposes `skill-manager version` and `skill-manager --version`.
  Release builds embed the release version; development builds report `dev`
  unless tagged Go module build information supplies a version.
- The packaging entry point validates a clean checkout, version consistency,
  tests, vet, frontend checks, build output, signatures, architecture, archive
  contents, and checksums before leaving ignored artifacts under
  `dist/release/`.
- Keep versioned release notes in the repository. README and release notes must
  disclose Apple Silicon/macOS 13+ support, ad-hoc signing, lack of
  notarization, and the expected Gatekeeper approval flow.
- Developer ID signing, notarization, DMG packaging, universal binaries,
  GitHub Actions, automatic publishing, and auto-update remain out of scope.
- Never restore or commit `scripts/public-check.sh`; release packaging uses the
  dedicated `scripts/package-release.sh` workflow.

### Public Preview Hardening (Iteration 13)

Iteration 13 replaces the private `v0.4.0` prerelease with a public `v0.4.1`
preview and tightens local privacy, distribution notices, and repository
operations without changing skill ownership semantics.

- The public desktop build exposes Dashboard, Skills, and Sources. Keep the
  experimental skills.sh adapter/domain code tested in the repository, but do
  not bind or render Discover in `v0.4.1`.
- Startup and ordinary refresh use filesystem context estimates only. Provider
  diagnostics run only from an explicit user action and only through the fixed
  read-only allowlist: two `codex debug prompt-input` variants and
  `claude plugin list --json`.
- Diagnostic subprocesses receive the current home plus only a minimal
  path/temp/locale/terminal environment. Do not forward credentials, provider
  config overrides, tokens, or proxy variables.
- Use owner-only permissions for Skill Manager state directories (`0700`) and
  state/cache JSON files (`0600`) without rewriting managed checkout file
  modes. Retain at most 10 state backups and at most 30 days.
- Do not persist skills.sh search terms or results. On the next desktop launch,
  sanitize legacy cache version 1 into version 2 while retaining non-query
  ranking/detail metadata.
- Generate `THIRD_PARTY_NOTICES.txt` from exact locked Go/runtime/frontend
  production dependencies. Package it with both binary artifacts and include
  the project license plus notices inside the desktop app resources.
- Build and publish `v0.4.1` manually from a clean local Apple Silicon macOS
  checkout. CI, Dependabot, issue/PR templates, private vulnerability
  reporting, and branch rules are repository operations completed after the
  source becomes public; release automation remains deferred.
- Treat `v0.4.1` as a preview. Developer ID signing, notarization, universal
  binaries, SBOM/provenance, auto-update, and commercialization review remain
  required follow-up before a stable or commercial release.

### Saved Skill Sets (Iteration 14)

Iteration 14 adds reusable, task-oriented recipes to the macOS app without
changing CLI, TUI, source ownership, or filesystem toggle semantics.

- A `Skill Set` has a stable opaque identifier, a unique user-facing name, an
  optional `When to use` description, and a sorted unique list of skill
  basenames. It is distinct from the automatically detected source `Group`.
- Membership is tool-agnostic. Each use explicitly selects Claude, Codex, or
  both and stages changes through the existing Pending/Apply workflow.
- A set uses the existing smart-toggle rule: all eligible effective cells ON
  targets OFF; otherwise eligible OFF cells target ON. Sets may overlap and do
  not own or reference-count active skills.
- Persist sets separately in `~/.skill-manager/skill-sets.json`; do not advance
  the ownership/toggle manifest from version 2. The file contains no source or
  filesystem paths and uses private atomic persistence plus bounded backups.
- Missing members remain in the set as unavailable. Reinstalling the same skill
  basename reconnects it, regardless of source. Source uninstall warns about
  affected sets but neither blocks nor rewrites them.
- Add a dedicated `Skill Sets` screen, `Save as Skill Set` from Pending, and
  `Add to Skill Set` from skill details. Creating, editing, and deleting set
  metadata is immediate and must not change skill state or existing Pending.
- Frontend mutations use set IDs, names, descriptions, skill names, and tool
  names only. They never accept or send filesystem paths or prebuilt toggle
  operations.
- Automatic history suggestions, active-set reference counting, project-local
  sets, import/export, per-skill notes, workflow steps, and CLI/TUI surfaces
  remain out of scope.

## Skill Context Budget Dashboard (Iteration 7)

Iteration 7 adds read-only context-cost visibility to the existing Dashboard.
It does not change toggle, install, update, uninstall, CLI, or TUI semantics.

- Show separate Claude Code and Codex global skill-catalog reports.
- Measure the startup catalog metadata, not full `SKILL.md` bodies. Full skill
  instructions remain an on-demand cost after invocation.
- Estimate catalog tokens as `ceil(characters / 4)` and label token values as
  approximate.
- Codex uses a 2% model-context budget. Prefer its local read-only diagnostic
  interfaces to measure the rendered and untruncated global catalog, including
  user, system, and enabled plugin skills. Run diagnostics from a neutral
  directory so repository-scoped skills are excluded.
- Claude Code uses its configured skill-listing budget, defaulting to 1% of the
  model context. Estimate personal and enabled-plugin entries from local files
  and settings; label the report partial when bundled, managed, or account
  state cannot be established outside an active Claude session.
- If Claude's context window cannot be resolved, use a clearly labeled 200,000
  token fallback. If Codex's context window is unavailable, use its 8,000
  character fallback budget.
- Show both the applied report and a projected `After Apply` report when pending
  GUI toggles change the catalog.
- Context diagnostics are best-effort and read-only. Failure, timeout, missing
  binaries, or malformed output must not block scanning or toggling skills.
- The report scope is global. Repository, nested-directory, additional-directory,
  and active-session-only skills are excluded.

## Validation Strategy

Use temp directories for tests. Do not test against the real global skill directories.

Backend tests should cover:

- Scanning symlink skills
- Scanning local directory skills
- Source classification from `~/.agents/.skill-lock.json`
- Hiding entries without `SKILL.md`
- State manifest persistence
- Disable and enable round trip for symlink entries
- Disable and enable round trip for directory entries
- Conflict detection on restore
- Dry-run output without filesystem mutation
- Deterministic apply ordering
- Repo URL normalization for HTTPS and SSH Git URLs
- Managed checkout path resolution
- Recursive skill discovery with ignored directories and duplicate-name conflicts
- Install manifest persistence
- Install preflight conflicts and idempotency
- Strict install dry-run without clone
- Symlink apply rollback for links created by a failed install operation
- Read-only `repos` command output without filesystem mutation
- Fast-forward-only repository update with installed-path preflight
- Update dry-run without fetch, Git-ref mutation, or state mutation
- Whole-repository uninstall for active and disabled skills
- Uninstall conflict detection, staging rollback, and cleanup reporting
- Manifest v1-to-v2 migration and local-source persistence
- Local source path normalization and protected-path overlap rejection
- Single-skill and recursive local discovery
- Local install dry-run, ownership, idempotency, conflict, and rollback behavior
- Local uninstall with active/OFF/broken targets, rollback, and source preservation
- Manifest-backed `local path` scan classification
- skills.sh response normalization, validation, cache persistence, offline
  fallback, local search, and rejection of oversized or unsafe responses
- Discover projection for ON/OFF/conflict cells and exact one-skill/selected-tool
  installation through the existing managed-source services
- Scoped desktop smart-toggle validation and active/available grouping,
  pending placement, search expansion, and session-only accordion state
- Skill Set persistence, validation, corruption isolation, missing-member
  retention, scoped smart-toggle preview/staging, overlapping-set projection,
  contextual creation, and non-blocking uninstall impact

TUI can be tested at the model/update layer. Full terminal rendering tests are optional for MVP.

## Documentation Rules

Keep [README.md](./README.md) user-facing and practical.

Keep this file as the agent-facing source of truth for architecture and product decisions.

Keep [planning/phase-1-mvp-tasks.md](./planning/phase-1-mvp-tasks.md) as the source of truth for task status.

Keep [planning/phase-2-group-bulk-toggle-tasks.md](./planning/phase-2-group-bulk-toggle-tasks.md) as the source of truth for Iteration 2 group and bulk-toggle task status.

Keep [planning/phase-3-repo-install-tasks.md](./planning/phase-3-repo-install-tasks.md) as the source of truth for Iteration 3 repository install task status.

Keep [planning/phase-4-repo-update-uninstall-tasks.md](./planning/phase-4-repo-update-uninstall-tasks.md) as the source of truth for Iteration 4 repository update and uninstall task status.

Keep [planning/phase-5-local-path-install-tasks.md](./planning/phase-5-local-path-install-tasks.md) as the source of truth for Iteration 5 local path install and uninstall task status.

Keep [planning/phase-6-macos-gui-tasks.md](./planning/phase-6-macos-gui-tasks.md) as the source of truth for Iteration 6 macOS GUI task status, and keep [DESIGN.md](./DESIGN.md) as the design-system contract for that interface.

Keep [planning/phase-7-skill-context-budget-tasks.md](./planning/phase-7-skill-context-budget-tasks.md) as the source of truth for Iteration 7 skill context-budget task status.

Keep [planning/phase-8-gui-source-management-tasks.md](./planning/phase-8-gui-source-management-tasks.md) as the source of truth for Iteration 8 GUI source-management task status.

Keep [planning/phase-9-skillssh-discover-tasks.md](./planning/phase-9-skillssh-discover-tasks.md) as the source of truth for Iteration 9 skills.sh Discover task status.

Keep [planning/phase-10-skills-workspace-tasks.md](./planning/phase-10-skills-workspace-tasks.md) as the source of truth for Iteration 10 active-first Skills workspace task status.

Keep [planning/phase-11-publication-readiness-tasks.md](./planning/phase-11-publication-readiness-tasks.md) as the source of truth for Iteration 11 public-source preparation task status.

Keep [planning/phase-12-github-release-tasks.md](./planning/phase-12-github-release-tasks.md) as the source of truth for Iteration 12 GitHub binary release task status.

Keep [planning/phase-13-public-preview-hardening-tasks.md](./planning/phase-13-public-preview-hardening-tasks.md) as the source of truth for Iteration 13 public-preview hardening and publication task status.

Keep [planning/phase-14-skill-sets-tasks.md](./planning/phase-14-skill-sets-tasks.md) as the source of truth for Iteration 14 saved Skill Set task status.

Keep [docs/wiki/README.md](./docs/wiki/README.md) as the source of truth for wiki maintenance rules, [docs/wiki/index.md](./docs/wiki/index.md) as the wiki content map, and [docs/wiki/log.md](./docs/wiki/log.md) as the append-only maintenance history.

If a future agent changes scope, semantics, paths, or UX, update this file and the planning task status table in the same change.
