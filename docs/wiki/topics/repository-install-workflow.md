# Repository Install Workflow

## Supported Inputs

- HTTPS Git URLs, with or without `.git`.
- SCP-style SSH Git URLs such as `git@github.com:owner/repo.git`.
- Tool target `claude`, `codex`, or `both`; default is `both`.
- Repeated `--skill <name>` selection; without it, all discovered skills are
  selected.
- The GUI can instead select exact skill/tool cells after inspection; the CLI
  Cartesian `--tool`/`--skill` behavior is unchanged.

GitHub shorthand, branch/tag/commit selection, submodules, sparse checkout, and
force overwrite are outside the current repository workflow. Local paths use
the separate [local path workflow](local-path-install-workflow.md).

## Identity And Checkout

- The URL is normalized to a host, repository path, canonical URL, and group.
- The checkout path is constrained to
  `~/.skill-manager/repos/<host>/<repo-path>`.
- A missing checkout is cloned during real install.
- An existing checkout is reused only when it is the Git root and its origin
  normalizes to the requested repository identity.
- Reuse never performs `git pull`.

## Discovery

- Every directory containing `SKILL.md` is an installable skill.
- Discovery is recursive and ignores `.git`, `node_modules`, `.venv`, `vendor`,
  `build`, and `dist` directories.
- The install name is the directory basename.
- Duplicate basenames fail before planning because both would target the same
  user-skill name.
- Discovered skills are sorted deterministically.

## Preflight And Idempotency

Every selected skill/tool cell is checked before creating links:

- Free active path: plan a symlink.
- Active symlink to the same discovered directory: already installed `ON`.
- Matching disabled symlink record: already installed `OFF`; do not enable it.
- Any other active entry or disabled target: conflict.
- Any missing requested skill: preflight failure.
- A cell recorded as owned by another Git or local source: ownership conflict.

A conflict fails the entire plan before apply. Nothing is overwritten, merged,
renamed, or deleted to make room.

## Apply And Manifest

- Create the planned symlinks in deterministic plan order.
- If symlink creation fails, remove only links created by this apply and only
  while they still point to the expected skill directory.
- Keep a checkout cloned earlier in the operation so retry remains cheap.
- After successful link creation, upsert repository URL, identity, checkout,
  group, skill paths/tools, install time, and last seen commit in `state.json`.
- Existing matching installed skill/tool records are merged rather than
  duplicated.

## Desktop Workflow

The Sources screen accepts a Git URL and performs checkout plus discovery
before showing an exact Claude/Codex/Muse matrix. A missing checkout may therefore
be cloned before final Apply; cancelling retains that clean unrecorded checkout
for retry. Review and Apply use opaque session IDs and re-run discovery and
preflight in Go. Per-repository Update and deterministic Update all call the
same fast-forward service as the CLI, and typed-confirmed Uninstall calls the
same ownership audit and transactional removal service.

## Strict Dry-Run

- Never clone a missing checkout.
- Report the checkout that would be cloned and that discovery cannot continue.
- If the checkout already exists, perform read-only discovery and preflight and
  print planned links or conflicts.
- Never create symlinks or write `state.json`.

## Fast-Forward Update

- `skill-manager update [<git-url>] [--dry-run]` updates one normalized
  repository identity, or all recorded repositories when the URL is omitted.
- The checkout must be clean, on a branch tracking `origin/*`, and free of
  local-only or divergent commits.
- Every recorded active or disabled symlink is audited. Missing, changed,
  duplicate, or extra managed-directory links into the checkout block update.
- Real update fetches `origin`, verifies fast-forward ancestry, and checks that
  every installed path still contains a regular `SKILL.md` in the target tree
  before merging with `--ff-only`.
- Newly added repository skills are intentionally ignored; update preserves
  the manifest's installed skill/tool set and existing ON/OFF link locations.
- Successful updates persist the target commit as `lastSeenCommit`. Update-all
  is deterministic, stops on the first failure, and keeps the completed prefix.
- Dry-run never fetches or changes remote-tracking refs. It checks the current
  checkout and cached upstream and reports that exact remote preflight remains
  unavailable until a real update.

## Whole-Repository Uninstall

- `skill-manager uninstall <git-url> [--dry-run]` requires one explicit
  repository identity; uninstall-all and force modes do not exist.
- The same reference audit and clean, recoverable checkout checks run before
  removal. An unrelated blocker at an OFF skill's original active path is not
  owned and remains untouched.
- Exact active/disabled links and the checkout are moved under
  `~/.skill-manager/trash/uninstall-*`; matching disabled records and the
  repository record are removed from state; staging is then deleted.
- Failures before state save roll staged paths back. A cleanup failure after a
  successful state save is reported with the staging path instead of implying
  rollback.
- Whole-repository `uninstall` supersedes the earlier `repo remove` idea.

## Deferred Repository Management

`planned`: branch selection and force behavior remain future work. See
[testing-development-and-roadmap.md](testing-development-and-roadmap.md).
