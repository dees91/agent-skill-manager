# Phase 21: Managed Checkout Diagnosis and Repair - Task Breakdown

> **Source**: user-approved planning session (2026-09-08).
> Iterations 1-20 are complete. Iteration 21 turns the opaque
> `checkout conflict: ... has tracked, untracked, or ignored worktree changes`
> failure into a named diagnosis and adds a confirmed, staging-backed repair for
> the one repairable class across the shared Go core, the CLI, and the macOS GUI.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it and to `done` only after verification passes.

## Product Decisions

- Checkout blockers become typed. `CheckoutConflictError` carries a stable
  `Kind`, the checkout path, and, for a dirty worktree, the exact offending
  entries. `Error()` keeps today's message text verbatim so existing contracts
  and assertions are unchanged.
- Exactly one kind is repairable: `dirty-worktree`. Detached HEAD, missing or
  foreign upstream, origin mismatch, divergence, and managed reference drift are
  diagnosed with readable guidance and are never repaired automatically.
- Repair moves every dirty path into `~/.skill-manager/trash/repair-*` with
  `rename`, restores tracked paths from `HEAD`, and deletes the staging
  directory only after the checkout verifies clean. Staging exists for rollback;
  the staged copies do not survive a successful repair. No `git clean`, no
  `reset --hard`, no in-place deletion.
- Repair never touches `state.json`, symlinks, remote refs, or anything outside
  the managed checkout root. It requires the same ownership audit as update and
  uninstall, refuses when that audit fails, and re-runs it after restoring the
  checkout before deleting recovery data.
- Branch, upstream, and ancestry blockers are rejected before any mutation, even
  on a dirty checkout, so repair never reports success while update still fails.
- The Git index is snapshotted before mutation and restored on rollback, and
  rollback removes paths repair recreated at locations that were absent
  beforehand, so a failed repair leaves the exact pre-operation state.
- Worktree enumeration uses porcelain v2 and preserves path bytes exactly.
- An installed `SKILL.md` is only stageable when `HEAD` holds it as a regular
  file that the restore step can put back.
- Repair is idempotent. A clean checkout succeeds without mutating anything.
- `skill-manager repair <git-url> [--dry-run]` always requires an explicit URL.
  No `--all`, no `--force`, and no interactive prompt, matching `uninstall`.
  `--dry-run` is the CLI preview and must not move a file.
- `update` reports a repairable conflict with the offending paths and the exact
  `repair` command to run. Non-repairable kinds get manual guidance instead.
- The GUI surfaces the diagnosis twice: as a `Needs repair` state in the Sources
  table before any update is attempted, and as a cause plus `Repair...` action in
  the dialog after a failed update. Repair is a separately confirmed source
  operation that runs in the existing exclusive source lane.
- GUI source health is an explicit read-only inspection, not part of the shared
  snapshot reload, because `git status --ignored` across every managed checkout
  is too expensive to pay on Dashboard and Skills refreshes.
- Frontend calls keep using opaque source identifiers. Repair previews report
  checkout-relative paths only; no absolute filesystem path crosses the bridge.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P21-T01 | Typed checkout conflicts and worktree entry diagnosis | done | - |
| P21-T02 | Repair service with trash staging and rollback | done | P21-T01 |
| P21-T03 | CLI `repair` command and update guidance | done | P21-T02 |
| P21-T04 | GUI backend health inspection, preview, and repair | done | P21-T02 |
| P21-T05 | Desktop frontend diagnosis, repair dialog, row state | done | P21-T04 |
| P21-T06 | AGENTS, CLAUDE, README, and wiki documentation | done | P21-T03 |
| P21-T07 | Full validation gates and end-to-end verification | done | P21-T05 |

## Task Definitions

### P21-T01: Typed checkout conflicts and worktree entry diagnosis

- Add `CheckoutConflictKind`, `CheckoutConflictError`, and `WorktreeEntry` to
  `internal/install/managed_checkout.go`.
- Return the typed error from every `checkout conflict:` site in
  `managed_checkout.go` and `checkout.go`, plus `requireFastForward`
  (`diverged`). `Error()` returns the existing message unchanged.
- On the dirty-worktree branch only, run
  `git status --porcelain -z --untracked-files=all --ignored` and parse the
  NUL-separated records into deterministic `WorktreeEntry` values. Handle
  rename records carrying two paths, `--ignored` directory records, and classify
  each entry as `tracked`, `untracked`, or `ignored`.
- Verification: `go test ./internal/install/...` with new parser cases and the
  existing message assertions untouched.

### P21-T02: Repair service with trash staging and rollback

- Add `internal/install/repair.go` with `RepairService`, `RepairPlan`,
  `RepairResult`, `Plan`, and `Apply`, mirroring `UninstallService` including
  its injectable `rename`, `removeAll`, `mkdirAll`, and `mkdirTemp` fields.
- Gate on `AuditRepositoryReferences`, then require `inspectManagedCheckout` to
  fail with `Kind == dirty-worktree`; a nil error is a no-op success and any
  other kind is refused with its own diagnosis.
- Re-assert `pathInside(paths.ReposDir, checkout.Path)`, sanitize every entry
  path (no absolute, no `..`, no backslash, inside the checkout, never under
  `.git/`), stage into `<trash>/repair-*/worktree/<rel>`, restore tracked paths
  with `git checkout --quiet HEAD --` and unstage additions with
  `git reset --quiet HEAD --`, re-verify the checkout, then remove staging.
- Roll back staged moves in reverse on any failure and report `CleanupPending`.
- Verification: `internal/install/repair_test.go` covering untracked, ignored
  file and directory, modified, deleted, staged addition, mixed, every refusal
  case, rollback, and a guard test asserting no `clean`, `reset --hard`, or `-f`
  git call is ever issued.

### P21-T03: CLI `repair` command and update guidance

- Add `repair <git-url> [--dry-run]` to the dispatch in `internal/cli/cli.go`,
  reusing the `parseUpdateArgs` and `selectRepositories` shapes, and list it in
  `printUsage`.
- Print the offending paths grouped by class with counts, and state that files
  move to `~/.skill-manager/trash` and are then removed.
- Extend `runUpdate` with an `errors.As` branch printing the named cause, the
  offending paths, and the exact `repair` command for repairable conflicts.
- Verification: `go test ./internal/cli/...` including a `--dry-run` case that
  asserts nothing moved and the extended dirty `update` batch case.

### P21-T04: GUI backend health inspection, preview, and repair

- Extend `SourceMutationFailure` with `SourceID`, `Cause`, and `Repairable`, and
  add `SourceHealth` and `RepairPreview` DTOs in `internal/gui/types.go`.
- Populate the new failure fields in `UpdateSource` and `UpdateAllSources`.
- Add read-only `InspectSources` built on `UpdateService.PlanLocal`, guarded by
  `SourceBusy` like `MeasureContextBudgets`, and deliberately not called from
  `reloadLocked`.
- Add `PreviewRepair` and `RepairSource` inside `runSourceOperation`, returning a
  fresh snapshot like every other source mutation.
- Delegate each new call from `desktop/app.go` and regenerate Wails bindings.
- Verification: `go test ./internal/gui/...` and `cd desktop && go test ./...`.

### P21-T05: Desktop frontend diagnosis, repair dialog, row state

- Add the new calls to the `Backend` interface, `generatedBackend`,
  `wailsBackend`, `demoBackend`, and `mockBackend`.
- Give `ConfirmDialog` an optional remediation slot driven by `failure.cause`,
  and add a `RepairDialog` following `UninstallDialog`'s open-then-load-preview
  shape with a `danger-button` and no typed group name.
- Call `inspectSources` on Sources mount and after each source mutation, merge by
  `sourceId`, and render a `Needs repair` state plus a row-level `Repair` action
  without disturbing the `Managed Git` and `Linked folder` labels.
- Verification: `npm run typecheck`, `npm test` with new failure-to-repair and
  row-state cases, and `make gui-build`.

### P21-T06: AGENTS, CLAUDE, README, and wiki documentation

- Add the Iteration 21 section to `AGENTS.md`, amend the Iteration 4 update and
  uninstall safety bullets to name a dirty checkout as repairable, add `repair`
  to the CLI decisions, extend the validation strategy list, and register this
  file in the documentation rules.
- Add the Phase 21 routing step to `CLAUDE.md`.
- Document `repair` in `README.md` alongside update and in the state and safety
  section.
- Update `docs/wiki/topics/repository-install-workflow.md`,
  `state-safety-and-recovery.md`, `interfaces.md`, `desktop-gui.md`, and
  `docs/wiki/sources/product-decisions-and-plans.md`, then append one
  `implementation` entry to `docs/wiki/log.md`.
- Verification: links resolve and no document contradicts the shipped behavior.

### P21-T07: Full validation gates and end-to-end verification

- Run `make test-all`, `go vet ./...` in both modules, `make vulncheck`,
  `npm audit --audit-level=high`, `make gui-build`, `make gui-test`, `make dev`.
- Verify on the real machine that `repair --dry-run` lists the untracked
  `android/skills` profiler binary, that the real run clears it, and that
  `update` then completes from both the CLI and the desktop Sources screen.
- Verification: every gate passes and the reproduction no longer occurs.
