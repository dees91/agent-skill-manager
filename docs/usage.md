# Skill Manager user guide

For a first launch, follow the [macOS quick start](../README.md#quick-start).
This guide covers installation alternatives, everyday use, and troubleshooting.

[Install CLI](#install-cli) · [Desktop](#desktop-interface) ·
[Terminal](#terminal-interface) · [Skill Advisor](#first-party-skill-advisor) ·
[Paths](#paths-skill-manager-uses) · [Safety](#state-and-safety) ·
[Troubleshooting](#troubleshooting)

## Install CLI

The CLI includes the interactive terminal interface (TUI). Both use the same
local inventory and safety rules as the desktop app; the app and terminal
binary are installed separately.

### 1. Download and verify

Download the [Apple Silicon CLI archive](https://github.com/dees91/agent-skill-manager/releases/download/v0.7.0/skill-manager-cli-0.7.0-macos-arm64.tar.gz)
and [SHA256SUMS.txt](https://github.com/dees91/agent-skill-manager/releases/download/v0.7.0/SHA256SUMS.txt)
from the `v0.7.0` preview into your Downloads folder. In Terminal, run:

```bash
cd "$HOME/Downloads"
grep 'skill-manager-cli-0.7.0-macos-arm64.tar.gz$' SHA256SUMS.txt \
  | shasum -a 256 -c -
```

Continue only if the archive is reported as `OK`. This verifies the archive
against the release checksum; it does not provide a Developer ID signature
or notarization ticket.

### 2. Install

From the same directory:

```bash
tar -xzf skill-manager-cli-0.7.0-macos-arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 0755 \
  skill-manager-cli-0.7.0-macos-arm64/skill-manager \
  "$HOME/.local/bin/skill-manager"
export PATH="$HOME/.local/bin:$PATH"
skill-manager --version
```

The expected output is `skill-manager 0.7.0`. To keep the command available in
new terminals, add this line once to `~/.zshrc` (or your shell's startup file):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

The binary supports Apple Silicon macOS 13 or newer. If macOS blocks it because
the developer cannot be verified, follow the
[first-launch guidance](../README.md#3-open-the-app) after checking its source
and checksum. Git source operations also require the local `git` executable.

### Build from source

See [Contributing: build and install the CLI](../CONTRIBUTING.md#build-and-install-the-cli)
for Go requirements and the `make dev` installation workflow, or
[build the desktop app](../CONTRIBUTING.md#build-the-desktop-app) for Wails setup.

## Desktop interface

The macOS app shares its local inventory and safety rules with the CLI and
TUI. It has four views:

- **Dashboard** shows managed visibility, groups, conflicts, and an approximate
  global skill-catalog context budget for each provider.
- **Skills** keeps active skills open, groups inactive skills by source, stores
  favorite managed skills, and stages reversible toggles for review and Apply.
- **Skill Sets** stores task-oriented combinations with an optional **When to
  use** note, then stages them for Claude, Codex, Muse, Grok, or all tools.
- **Sources** installs exact Claude/Codex/Muse/Grok skill cells, extends every
  recorded source to one more tool, and updates or uninstalls complete Git
  and local sources recorded in the manifest.

Open **Skill Manager → About Skill Manager** in the macOS application menu to
see the app icon, current build version, and product description. The app reads
these values from the same desktop build metadata as the application bundle.

The experimental skills.sh Discover implementation is still under development
and does not appear in the `v0.7.0` public preview.

Skill toggles stay pending until you choose **Apply changes**. Source operations
have their own confirmations and cannot run while a toggle batch is pending.

### Skill Sets

![Two saved Skill Sets showing enabled, mixed, disabled, and unavailable tool states](images/skill-sets.png)

Skill Sets are overlapping recipes, not active profiles. Each use asks for a
tool scope and enters the ordinary Pending/Apply flow. Missing members stay in
the recipe and reconnect when a skill with the same basename is installed
again. The CLI and TUI do not manage Skill Sets yet.

### Favorites

Favorites make large catalogs easier to revisit. Star a managed user skill
from its row or details, then use the **Favorites** availability filter.
Favorites sort ahead of other skills and survive ON/OFF changes or source
removal. Installing the same basename reconnects the favorite. The CLI, TUI,
and Skill Advisor do not manage favorites yet.

### Dashboard and context estimates

![Dashboard showing skill visibility and estimated catalog context use for four tools](images/dashboard.png)

Dashboard estimates the startup skill catalog from local files. These are
approximate metadata costs, not measurements of an entire conversation or of
every skill body an agent might load later.

**Run provider diagnostics** explicitly invokes installed Claude/Codex tools
with fixed, read-only arguments. A failed diagnostic falls back to an estimate.
Muse and Grok always use filesystem estimates. See
[Privacy: local provider diagnostics](../PRIVACY.md#local-provider-diagnostics).

### Install and maintain sources

1. Open **Sources → Install source**.
2. Choose **Git repository** and enter an HTTPS or SSH Git URL, or choose
   **Local folder** to use the native directory picker.
3. Inspect the source, then select the skill/tool cells to install. A column
   toggle selects or clears all eligible cells for that tool, including rows
   hidden by the search filter.
4. Choose **Review**, check the proposed links and conflicts, then **Install**.

Git inspection may clone a missing checkout before the final installation.
Cancelling retains that checkout for a later retry. Local folders stay in
place; installation creates links to their skills.

Sources lists only Git and local installations recorded by Skill Manager.
Other skills still appear in Skills, but their groups do not become owned
sources merely because they were discovered.

| Action | What happens |
| --- | --- |
| Update / Update all | Fetch and fast-forward recorded Git repositories after safety checks. Linked folders are read directly and need no update. |
| Extend to tool | Preview links from every recorded source to one additional tool, then confirm. |
| Repair | Preview files blocking a dirty Git checkout, then confirm their removal and restoration of tracked paths from the current commit. |
| Uninstall | Type the source's group name to remove its entire recorded installation. Git checkouts are removed; local source folders are preserved. |

![Extend to tool dialog with tool selection and a per-source link preview](images/sources-extend.png)

Source actions have their own confirmations. Apply or clear pending Skills
toggles first. While a source operation is running, other mutations and app
close are blocked; progress is shown and the operation cannot be cancelled.
See [source commands](#manage-sources) and [troubleshooting](#troubleshooting)
for safety requirements and failure recovery.

## Terminal interface

Run Skill Manager without a subcommand to open the TUI:

```bash
skill-manager
skill-manager tui
```

Inspect the current installation with read-only commands:

```bash
skill-manager version
skill-manager list
skill-manager list --json
skill-manager list --json --available-for codex --query video --query remotion
skill-manager advisor search --tool codex --query "video remotion rendering"
skill-manager status
skill-manager groups
skill-manager repos
```

### Toggle one skill

Preview a change with `--dry-run`, then toggle one managed skill:

```bash
skill-manager disable --tool claude example-skill --dry-run
skill-manager disable --tool claude example-skill
skill-manager enable --tool claude example-skill
```

### Manage sources

Examples use synthetic repository URLs, skill names, and local folders; replace
them with your own sources. Git operations require Git.

Install every valid skill from a Git repository, or select exact skills and
tools:

```bash
skill-manager install https://github.com/example/agent-skills --dry-run
skill-manager install https://github.com/example/agent-skills --tool both
skill-manager install git@github.com:example/agent-skills.git --tool codex --skill example-skill
```

Install a user-owned folder as live, in-place links:

```bash
skill-manager install ~/Developer/example-skills --tool both
skill-manager install ./skills --tool claude --skill example-skill --dry-run
```

Update a recorded Git source or uninstall a complete recorded source:

```bash
skill-manager update --dry-run
skill-manager update https://github.com/example/agent-skills
skill-manager uninstall https://github.com/example/agent-skills --dry-run
skill-manager uninstall ~/Developer/example-skills --dry-run
```

Tool selection defaults to every supported tool. `--tool all` and the legacy
`--tool both` both mean Claude, Codex, Muse, and Grok. Repeat `--skill` to select
multiple skills. Install never pulls an existing checkout or implicitly enables
a skill already recorded as OFF. Conflicts are checked before any new links
are created.

An install dry-run never clones: for a missing checkout it reports that skill
discovery must wait for a real install. All mutating commands accept
`--dry-run`; update dry-runs do not fetch, so exact remote preflight is only
available during a real update.

Git updates are fast-forward-only and require a clean, audited checkout. Local
path sources are live links, so they do not need an update operation. Uninstall
removes a complete recorded source; Skill Manager does not uninstall one skill
from a source at a time.

When a skill writes generated files into its own checkout, update stops with a
named cause and the exact command to clear it:

```bash
skill-manager repair https://github.com/example/agent-skills --dry-run
skill-manager repair https://github.com/example/agent-skills
```

`repair` lists the blocking checkout-relative paths, moves them into
`~/.skill-manager/trash` so a failed repair rolls back, restores tracked files
from the current checkout commit (`HEAD`), and then deletes the staged copy.
A successful repair permanently discards these worktree changes; staging is
for rollback, not a recycle bin. Inspect the dry-run paths before applying it.
It only clears worktree changes. Detached HEAD, local commits, a changed origin,
and link drift are explained but never resolved for you. The desktop app shows
the same diagnosis as a `Needs repair` state on the Sources screen.

Link every recorded source to one more tool without reinstalling each source.
A skill stays OFF on the new tool when it is OFF for every other recorded tool:

```bash
skill-manager extend --tool muse --dry-run
skill-manager extend --tool muse
```

Extend processes sources in manifest order and stops at the first failure,
keeping earlier sources extended. The desktop Sources screen offers the same
bulk action as "Extend to tool". Start a new session afterwards so the added
tool picks up the new skills.

### TUI controls

| Key | Action |
|---|---|
| `Tab` | Switch active tool column |
| `Up`/`Down` or `k`/`j` | Move selection |
| `Space` | Stage or unstage the active-cell toggle |
| `b` | Smart-toggle all tool cells in the row |
| `g` | Smart-toggle the selected source group |
| `A` | Smart-toggle all visible eligible cells |
| `G` | Cycle group filter |
| `/` | Edit text filter |
| `s` | Cycle source filter |
| `o` | Show or hide read-only skills |
| `d` | Show or hide details |
| `u` / `U` | Undo one / clear all pending changes |
| `a` or `Enter` | Apply the pending batch |
| `r` | Rescan |
| `q` | Quit, with a warning for pending changes |

`Space`, row/group toggles, and all-visible toggles stage changes until Apply.
A group toggle covers the selected group even when some of its rows are
filtered out; all-visible covers only filtered results. Bulk actions skip
read-only, missing, and conflicting cells. If every eligible cell is effectively
ON, the scope is staged OFF; otherwise its OFF cells are staged ON. Repeating
a batch with the same pending intent can undo that batch.

Apply processes disables first, then enables, sorted by tool and skill name.
If a batch fails, completed changes stay applied and the error is reported.

## First-party Skill Advisor

The current source build includes the optional
[`skill-advisor`](../skills/skill-advisor/SKILL.md). Before a non-trivial task, it
can inspect locally installed skills and report the smallest clearly relevant
set in two groups: already active and needing activation. It selects at most
five total for the current Claude Code, Codex, Muse, or Grok host, activates the OFF
skills, and returns an opaque receipt when it owns or shares a lease. The
agent cleans that exact receipt itself before its final response. It needs no
plugin or provider hook.

After reviewing the skill instructions, install it for all tools from the
public repository:

```bash
skill-manager install https://github.com/dees91/agent-skill-manager \
  --tool both --skill skill-advisor
```

Contributors can link the current checkout instead:

```bash
skill-manager install . --tool both --skill skill-advisor --dry-run
skill-manager install . --tool both --skill skill-advisor
```

The skill uses a versioned, path-free inventory and receipt API:

```bash
skill-manager advisor status --tool codex --json
skill-manager advisor search --tool codex \
  --query "video remotion ffmpeg animation rendering" --limit 20 --json
skill-manager advisor activate --tool codex --skill example-skill --json
skill-manager advisor cleanup --receipt <receipt-id> --json
```

`advisor search` ranks only the selected host's toggleable ON and OFF skills.
It uses deterministic local weighted BM25F across name, description, group, and
source metadata, with exact-phrase bonuses and bounded fuzzy token matching.
The default result limit is 20 and the accepted range is 1-50. Search does not
use a model, embeddings, an API, a cache, or a persistent index. The query stays
in process memory; JSON results omit it along with filesystem paths, matching
reasons, and numeric scores.

`activate` accepts one tool and 1-5 unique skill names. The advisor reports the
selected names as already active or needing activation before it calls the API.
Skills that were already ON remain user-owned. Several receipts may share one
advisor-enabled skill; the skill returns to OFF only when the final receipt is
cleaned up. The advisor reads every selected `SKILL.md`, including those already
ON, so the current task does not depend on a provider catalog refresh. Before a
normal final response, the agent cleans the exact receipt created by its own
invocation instead of asking the user to paste a command. If cleanup fails, it
preserves and reports the receipt and recovery command. It never infers that an
unknown receipt is stale.
If a provider cannot see a newly installed advisor, start a new provider
session.

The older `list --json --query` flags remain unchanged as case-insensitive OR
substring filters across skill name, description, group, and source. They are a
general inventory surface, not the advisor's ranking mechanism.

The skill source and CLI API can change at different times on `main`. Before it
selects or mutates anything, it checks for `apiVersion: 1` and the
`ranked_search_v1` capability. A missing capability fails with an upgrade
message; the skill does not fall back to substring lookup.

## Paths Skill Manager uses

Skill Manager uses these global paths. Muse also honors an absolute
`XDG_CONFIG_HOME` when set; other custom provider roots and project-level skill
directories are unsupported:

| Purpose | Path | Behavior |
|---|---|---|
| Claude user skills | `~/.claude/skills` | Toggleable |
| Codex user skills | `~/.agents/skills` | Toggleable |
| Muse user skills | `~/.config/muse/skills` (`$XDG_CONFIG_HOME/muse/skills` when set) | Toggleable |
| Grok user skills | `~/.grok/skills` | Toggleable |
| Codex system skills | `~/.codex/skills/.system` | Read-only |
| Claude plugin cache | `~/.claude/plugins/cache` | Read-only |
| Skills CLI metadata | `~/.agents/.skill-lock.json` | Read-only source label |
| Skill Manager state | `~/.skill-manager` | Owned by Skill Manager |

Skill Manager hides entries that lack a regular `SKILL.md`. It rescans the
filesystem at startup, after a mutation, and when you refresh. The manifest
records ownership and restoration data; it does not replace live discovery.

## State and safety

Skill Manager keeps its state under `~/.skill-manager/`:

```text
~/.skill-manager/
  state.json
  advisor-activations.json
  advisor.lock
  skill-sets.json
  favorites.json
  backups/
  cache/skills-sh/catalog-v1.json
  disabled/claude/
  disabled/codex/
  disabled/muse/
  disabled/grok/
  repos/
  trash/
```

These rules apply to every interface:

- Disable and enable move the original entry. Skill Manager never dereferences
  a symlink during the move.
- Restore and install stop at a blocker. They never overwrite, merge, rename,
  or delete it.
- Skill Manager backs up existing state before the first mutation in a process.
- Skill Set metadata uses a separate atomic, owner-only file with bounded
  `skill-sets-*.json` backups. Malformed recipe data does not block ordinary
  scanning or toggles.
- Favorites use a separate atomic, owner-only basename list with bounded
  `favorites-*.json` backups. Malformed favorite data disables only favorite
  controls; scanning, Pending, Apply, and source actions remain available.
- Advisor metadata uses a separate atomic, owner-only file, a no-follow process
  lock, and bounded `advisor-activations-*.json` backups. It stores the receipt,
  tool, skill, timestamp, and restore fingerprints—never task, prompt, or
  advisor-search query content.
- State directories are limited to the current user. State and cache JSON files
  use mode `0600`; state backups retain at most 10 files and 30 days.
- A failed batch stops at the first error and writes the successfully completed
  prefix to state.
- Git uninstall audits owned links and checkout safety before it stages and
  removes them.
- Repair only clears worktree changes from a managed checkout. It stages every
  path through `~/.skill-manager/trash`, never runs `git clean` or
  `reset --hard`, never leaves the checkout root, and refuses to move an
  installed skill directory.
- Local-source uninstall never stages, edits, moves, or deletes the source
  folder.
- Toggles and link installation do not edit `SKILL.md`. Explicit Git updates
  and repair can change files in managed checkouts. Provider plugin caches and
  external manager lockfiles stay read-only.
- Every mutating CLI command supports `--dry-run`.

Installed skills contain instructions that an agent may follow. Skill Manager
does not audit or sandbox third-party skill contents. Review a source before
you install it.

## Troubleshooting

| Symptom | What to do |
| --- | --- |
| No skills appear | Confirm the folder contains a regular `SKILL.md` and is installed under a supported global path. Use Sources to install a Git or local source, then Refresh. |
| A running agent still sees the old skills | Start a new Claude Code, Codex, Muse, or Grok session after installation, update, uninstall, or an applied toggle. |
| `skill-manager: command not found` | Check the binary exists at `~/.local/bin/skill-manager` and add `~/.local/bin` to `PATH` as shown in [Install CLI](#install-cli). |
| Source buttons are unavailable | Apply or clear pending Skills changes. Wait for any active source operation to finish. |
| A cell shows `CONFLICT` | Open its details and inspect the original path, disabled path, and blocker. Resolve the blocker manually without overwriting your data, then Refresh. |
| Update reports `Needs repair` | Preview `repair --dry-run` and inspect every path. Repair discards worktree changes on success; preserve anything you need outside the checkout first. |
| Update reports another blocker | Follow the named diagnosis. Local commits, detached branches, changed origins, and link drift need manual resolution; repair does not resolve them. |
| A batch fails partway through | Review the fresh state before retrying. Completed toggles or earlier source updates remain applied; there is no full-batch rollback. |
| An operation reports recovery data or cleanup residue | Keep the named recovery paths and inspect the error before retrying or deleting anything. Failed rollback may need manual recovery. |

State backups are manifest backups, not copies of skills or source repositories.
Do not delete `~/.skill-manager` while it still holds disabled entries or
managed installations. Restore disabled skills and uninstall managed sources
first; see [Privacy: retention and removal](../PRIVACY.md#retention-and-removal).
Missing `state.json` is not automatically reconstructed from disabled entries.

For bugs, use [Issues](https://github.com/dees91/agent-skill-manager/issues).
Report vulnerabilities through the [security policy](../SECURITY.md), using
synthetic examples instead of private inventories or credentials.

## Status and compatibility

The current source version is `0.7.0`. It is a public preview, not a stable
release.

- The desktop app supports Apple Silicon Macs running macOS 13 or newer.
- The Go CLI and TUI can compile on other platforms, but the release contract
  does not yet cover filesystem behavior outside macOS.
- Skill Manager uses the [global paths listed above](#paths-skill-manager-uses),
  including Muse's `XDG_CONFIG_HOME` override. Other custom provider roots are
  unsupported.
- Release downloads are built for Apple Silicon and ad-hoc signed. They are not
  Developer ID signed or notarized.
- The preview does not include universal macOS binaries, DMG installers,
  auto-update, or telemetry.

## Network and privacy

Skill Manager has no telemetry, account system, analytics SDK, or background
polling. Explicit Git source operations use the local `git` executable to clone,
fetch, and fast-forward a repository you selected.

Dashboard estimates context use from the filesystem by default. Its explicit
**Run provider diagnostics** action may invoke installed `claude` and `codex`
binaries with fixed, read-only arguments. These are third-party executables;
their own behavior and privacy terms also apply. If either command fails,
Dashboard keeps using an estimate. Muse and Grok have no provider diagnostic
and are always labeled filesystem estimates.

Read [PRIVACY.md](../PRIVACY.md) for the full data flow and [SECURITY.md](../SECURITY.md)
for vulnerability reporting.
