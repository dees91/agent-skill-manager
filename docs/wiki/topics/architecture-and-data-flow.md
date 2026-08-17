# Architecture And Data Flow

## Boundaries

- [`internal/model`](../../../internal/model/model.go) is the shared vocabulary.
- [`internal/paths`](../../../internal/paths/paths.go) centralizes every fixed
  home-relative path and enables temporary-home tests.
- [`internal/scan`](../../../internal/scan/scan.go) is read-only discovery and
  presentation assembly.
- [`internal/ops`](../../../internal/ops/ops.go) owns toggle planning and moves.
- [`internal/staging`](../../../internal/staging/staging.go) owns pure pending,
  effective-state, and smart batch-toggle behavior shared by interactive UIs.
- [`internal/contextbudget`](../../../internal/contextbudget/) owns best-effort
  provider catalog measurement, character-derived token/budget calculations,
  coverage/accuracy labels, and path-free pending projections.
- [`internal/skillssh`](../../../internal/skillssh/) owns anonymous catalog
  HTTP, response validation/limits, normalization, local search, freshness,
  and the versioned normalized cache. It does not mutate installed skills.
- [`internal/skillsets`](../../../internal/skillsets/) owns the independent
  versioned saved-recipe file, validation, atomic persistence, and bounded
  backups. It stores skill basenames but no paths or tool selections.
- [`internal/favorites`](../../../internal/favorites/) owns the independent
  versioned favorite-basename file, validation, atomic persistence, and bounded
  backups. It stores no paths, tools, source metadata, or usage history.
- [`internal/advisor`](../../../internal/advisor/) owns the versioned
  receipt/lease file, cross-process lock, exact activation/cleanup preflight,
  and path-free public result types. It reuses `scan` and `ops` rather than
  moving entries directly.
- [`internal/gui`](../../../internal/gui/) owns the concurrency-safe desktop
  session, identifier-only action API, source drafts/reviews, snapshots,
  summaries, apply results, and pending state. It has no Wails dependency.
- [`internal/install`](../../../internal/install/) owns Git repository identity,
  checkout, local path identity, discovery, reference audit, install, update,
  uninstall, preflight, apply, staging, and rollback.
- [`internal/state`](../../../internal/state/store.go) is the persistence
  boundary for disabled entries, Skill Manager-managed repositories, and
  link-in-place local sources.
- [`internal/cli`](../../../internal/cli/cli.go) and
  [`internal/tui`](../../../internal/tui/) are interfaces over those backend
  services.
- [`desktop/`](../../../desktop/) is a nested Go 1.26.6 Wails 2.14 module. Its
  root `App` is only a generated-binding adapter over `internal/gui`; React and
  TypeScript live under `desktop/frontend/`.

## Scan Flow

```text
fixed Paths
  -> scan active managed directories
  -> scan disabled manifest entries
  -> optionally scan read-only system/plugin directories
  -> parse SKILL.md metadata and source evidence
  -> classify Source and Group
  -> merge Claude/Codex cells into SkillRow values
  -> CLI tables or filtered TUI model
```

The manifest augments scanning for disabled entries and managed repository
or local-source metadata; it does not replace filesystem observation.

## Runtime Skill Advisor Flow

```text
first-party skill reads list --json
  -> model selects <=5 relevant OFF cells for current host
  -> advisor activate fully preflights names and receipt state under lock
  -> receipt/lease claims are atomically persisted
  -> ops enables each OFF cell through existing state/backup semantics
  -> skill reads the activated SKILL.md files directly
  -> explicit cleanup validates restore fingerprints
  -> shared claims are released; final claims are disabled through ops
```

Cells that were already ON never enter a lease. Persisting claims before an
enable makes interrupted activations discoverable; cleanup can safely finalize
a claim whose cell is already back OFF. The advisor lock serializes advisor
processes, while `ops` still revalidates the live filesystem immediately before
every move.

## Desktop Session Flow

```text
React identifier actions (skill, tool, group, visible names)
  -> generated Wails App bindings
  -> gui.Service mutex and staging.Memory
  -> path-free ActionResult/PendingChange projection
  -> explicit Apply
  -> ops.PlanBatch and ops.Apply
  -> completed prefix removed from pending
  -> fresh scan and structured ApplyResult
```

The frontend never submits filesystem paths or prebuilt operations. Desktop
pending state is owned by the Go process and survives frontend projections but
not application exit. Startup, explicit refresh, read-only opt-in, and Apply
are scan boundaries; filters and staging actions do not rescan.

## Saved Skill Set Flow

```text
opaque Skill Set ID + explicit Claude/Codex tool names
  -> gui.Service resolves saved basenames against current scan rows
  -> staging.ToggleBatch previews against a copy of Pending
  -> confirmed use stages through the same engine in session Pending
  -> overlapping sets receive refreshed effective-state projections
  -> ordinary Apply performs deterministic filesystem moves and rescans
```

Create/update/delete mutate only `skill-sets.json`; they do not apply or clear
Pending. Missing names remain in recipe metadata and project as unavailable
until a matching basename returns. A corrupt recipe file produces a desktop
warning without blocking normal scan/toggle/source operations. Source uninstall
previews cross-check installed names against sets, but never block or rewrite
those recipes.

## Favorite Flow

```text
skill basename + desired boolean state
  -> Wails App forwards identifiers only
  -> gui.Service validates current managed-row eligibility for add
  -> favorites.Store atomically replaces favorites.json when state changes
  -> complete sorted basename list returns to React
  -> current rows update without changing Pending or filesystem visibility
```

Missing basenames remain in metadata and reconnect during a later scan. A
corrupt favorite file disables only favorite controls. Source uninstall
previews matching basenames but retains the favorite list.

## Desktop Source Lifecycle Flow

```text
Git URL or native-picked local folder
  -> gui.Service validation, checkout/discovery, and opaque draft ID
  -> React exact skill x tool selection
  -> backend re-discovery/preflight and opaque review ID
  -> explicit Apply through shared install service
  -> state save, fresh scan, and structured result

opaque managed source ID
  -> backend manifest lookup and ownership audit
  -> confirmed update or typed-confirmed whole-source uninstall
  -> shared update/uninstall service
  -> completed-prefix/failure result and fresh scan
```

The GUI lists only repositories and local paths recorded in `state.json`.
Frontend mutation requests contain source/draft/review IDs plus skill/tool
names; only Git inspection accepts a raw URL, and local input comes from the
native directory picker. Source operations share one non-cancellable mutation
lane, block skill staging/apply and app close while active, and are themselves
blocked by pending toggles.

## Dormant skills.sh Discover Flow

```text
ranking tab/search
  -> skillssh adapter with timeout, validation, and response-size limit
  -> normalized page/detail cache
  -> gui.Service projects active/OFF/conflict state per Claude/Codex cell
  -> React catalog, detail drawer, and explicit safety confirmation
  -> fresh live detail revalidation
  -> fixed GitHub repo identity + exact selected skill/tool cells
  -> existing checkout, discovery, preflight, install apply, and rescan
```

The Go adapter and domain tests preserve this experimental flow, but `v0.4.2`
does not expose it through Wails or React and therefore makes no skills.sh
request. Search queries remain in memory only; legacy persisted queries are
removed on the first desktop snapshot. Raw downloaded skill files are never
cached.

## Context Budget Flow

```text
global scan rows + local provider settings
  -> contextbudget.Analyzer (Claude and Codex in parallel)
  -> applied Reports + managed CellKey contributions
  -> gui.Service snapshot
  -> staging action maps pending tool/skill operations to deltas
  -> projected Reports returned with ActionResult
  -> React current bars and After Apply marker
```

Default analysis is filesystem-only. Only the explicit Dashboard action runs
read-only, fixed-argument subprocesses with output limits, timeouts, an exact
argument allowlist, and an injected minimal environment that excludes secrets
and proxies. The analyzer uses a neutral temporary working directory for Codex
so repository/session skills do not enter the global measurement. Provider
failure is report data, not a scan error.

## Toggle Flow

```text
CLI request or TUI/GUI pending cells
  -> ops.PlanBatch
  -> validate current source and free destination
  -> deterministic order: disable, then enable; tool; skill name
  -> backup existing state once per service process
  -> rename one entry at a time
  -> update manifest for the completed prefix
  -> rescan
```

Batch toggle apply intentionally stops at the first failure. Completed moves
remain completed and are persisted; full transactional rollback is outside the
MVP contract.

## Repository Install Flow

```text
Git URL
  -> normalized RepoIdentity and managed checkout path
  -> clone missing checkout or verify/reuse matching checkout
  -> recursively discover directories containing SKILL.md
  -> select tools and optional skill names, or exact GUI skill/tool cells
  -> preflight every target and disabled-state collision
  -> create missing symlinks
  -> rollback links from this apply if symlink creation fails
  -> upsert repository metadata in state.json
  -> future scans see the new active symlinks
```

Dry-run stops before cloning a missing checkout. An existing checkout can still
be discovered and preflighted without mutation.

## Local Path Install Flow

```text
local path
  -> expand and canonicalize existing directory; reject protected overlap
  -> root SKILL.md means one skill, otherwise recursive discovery
  -> select tools and optional skill names, or exact GUI skill/tool cells
  -> preflight targets and manifest source ownership
  -> create exact symlinks into the user-owned source
  -> rollback links if apply or state save fails
  -> upsert local-source metadata in state.json
```

No checkout is created. Reinstall may adopt an exact link or add new skills,
but recorded ownership drift blocks mutation.

## Repository Update Flow

```text
recorded repository (one URL or deterministic all)
  -> audit manifest-owned active/disabled symlinks and reject extras
  -> require clean branch tracking origin/* and fast-forward ancestry
  -> dry-run: stop at cached upstream with remote limitation
  -> real: fetch origin
  -> verify every installed SKILL.md in the fetched target tree
  -> merge --ff-only
  -> persist target lastSeenCommit
```

Update deliberately preserves the installed skill/tool set and ON/OFF link
placement. A newly discovered skill is not an implicit install.

## Repository Uninstall Flow

```text
explicit recorded repository URL
  -> audit references and clean/recoverable checkout
  -> backup existing manifest
  -> move exact owned links and checkout into trash/uninstall-*
  -> remove matching disabled records and repository record
  -> atomically save state
  -> delete staging, or report post-save cleanup residue
```

Before-save failures restore staged paths in reverse order. An unrelated active
blocker for an OFF skill never enters the removal set.

## Local Source Uninstall Flow

```text
explicit recorded local path
  -> audit active/disabled links, allowing a missing source target
  -> backup existing manifest
  -> move exact owned links into trash/uninstall-local-*
  -> remove matching disabled records and local-source record
  -> atomically save state
  -> delete staging; never stage or delete the source
```

## Important Separation

Toggle operations move individual entries, installs create symlinks, Git update
fast-forwards checkout content without relinking, and uninstall stages audited
owned paths. They share paths, model types, and the state manifest, but retain
distinct planners and apply services.
