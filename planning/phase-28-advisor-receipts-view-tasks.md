# Phase 28: Advisor Receipts View - Task Breakdown

> **Source**: user-approved plan (2026-10-05). An advisor receipt stays
> recorded when an agent session ends before its cleanup. Before this phase,
> the only way to find and release such receipts was the CLI
> (`advisor status`, `advisor cleanup`). A receipt whose cleanup stops on
> drift stayed recorded with no remedy.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before you start it. Set it to `done` only after the
> verification is successful.

## Product Decisions

- The desktop **Advisor** view gets a **Receipts** section below the provider
  settings. It shows each recorded receipt.
- For each receipt, the section shows:
  - The tool.
  - The time of the activation, as a relative time and as a local date and
    time.
  - Each skill of the receipt with the action that a cleanup will do:
    `disable`, `release` (another receipt shares the lease), or `already off`.
  - A short form of the receipt ID.
- The section gets the actions from a cleanup dry-run of each receipt. When
  the dry-run stops on drift or on a conflict, the receipt is **blocked**. A
  conflict includes an unrecorded entry at the disabled destination of a skill
  that cleanup must turn off. A blocked receipt shows the cause and no cleanup
  action. A dry-run that stops on another error, for example a scan failure,
  also shows the receipt as blocked, with a fixed message. Forget of that
  receipt stops with an error until the cause is fixed.
- Actions:
  - **Clean up** releases one receipt. This is the same operation as
    `skill-manager advisor cleanup --receipt <id>`.
  - **Clean up all** releases each receipt that is not blocked, in the order of
    the list. It stops at the first failure and shows the receipts that it
    released. It does not touch blocked receipts.
  - **Forget** is available only on a blocked receipt. It removes the receipt
    record and its claims on the leases. It does not change a skill link, a
    disabled entry, or `state.json`. A lease without claims is removed. Each
    skill stays in its current state.
- Each action has a confirmation. The confirmation tells you that an open
  agent session can still use these skills, and that a cleanup disables them
  for that session.
- The Forget confirmation tells you that Skill Manager will not disable these
  skills, and that you must toggle them yourself in **Skills** if necessary.
- Advisor actions use the exclusive source lane. Skills toggles that you did
  not apply stop them, as for the source operations. After each action, the snapshot is
  loaded again.
- No filesystem path crosses the desktop bridge. A blocked cause names the
  tool and the skill only. Other receipt failures, such as scan or filesystem
  errors, show a fixed message that points to the CLI dry-run for details.
- The confirmation keeps keyboard focus inside the dialog (Tab and Shift+Tab)
  and returns focus to the control that opened it.
- CLI: `skill-manager advisor forget --receipt <id> [--dry-run] [--json]`. The
  command stops with an error when the cleanup of the receipt is possible. For
  that receipt, use `advisor cleanup`.
- The first-party `skill-advisor` skill does not change. It never uses
  `forget`. The rule that an agent cleans only its own receipt does not change.
- Skill Manager does not clean or forget a receipt automatically, and does not
  use the age of a receipt as a cause for an action.

Rejected alternatives:

- Forget on a receipt that is not blocked. The activated skills will then stay
  ON with no owner. Use **Clean up** for that receipt.
- An automatic expiry. A receipt can belong to a long session that is still in
  progress.

## Interfaces

GUI service and desktop `App`:

- `ListAdvisorReceipts() ([]AdvisorReceipt, error)` returns each receipt with
  its tool, creation time, skill actions, and blocked cause. It does not
  change files.
- `CleanupAdvisorReceipt(receiptID string, includeReadOnly bool) AdvisorReceiptResult`
- `CleanupAllAdvisorReceipts(includeReadOnly bool) AdvisorReceiptResult`
- `ForgetAdvisorReceipt(receiptID string, includeReadOnly bool) AdvisorReceiptResult`
- `AdvisorReceiptResult` contains a message, the released or forgotten receipt
  IDs, an optional failure, the fresh receipt list, and a fresh snapshot.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P28-T01 | Advisor domain: forget for blocked receipts, path-free blocked causes | done | - |
| P28-T02 | CLI `advisor forget` | done | P28-T01 |
| P28-T03 | GUI backend: list, cleanup, cleanup all, forget, bindings | done | P28-T01 |
| P28-T04 | Desktop: Receipts section and confirmations | done | P28-T03 |
| P28-T05 | AGENTS, usage, DESIGN, and wiki documentation | done | P28-T02, P28-T04 |

## Task Definitions

### P28-T01: Advisor domain

- Add `Forget(receiptID string, dryRun bool)` to `internal/advisor.Service`.
  Under the advisor lock, run the cleanup plan. When the plan is possible,
  stop with an error. Otherwise, remove the receipt, its claims, and the
  leases without claims, and save the file one time.
- Classify the cleanup plan errors (drift, conflict, missing skill) so that a
  caller can show a cause without a filesystem path.
- Verification: `go test ./internal/advisor/...`. Include these tests:
  - Forget of a blocked receipt keeps the skill links and `state.json`.
  - Forget of a shared lease removes only the claim of that receipt.
  - Forget of a receipt that cleanup can release stops with an error.
  - Forget of a receipt blocked by an unrecorded entry at the disabled
    destination keeps both entries.

### P28-T02: CLI

- Add `advisor forget --receipt <id> [--dry-run] [--json]` with the same JSON
  error contract as `advisor cleanup`, and the help text.
- Verification: `go test ./internal/cli/`.

### P28-T03: GUI backend

- Add the four methods to the GUI service and to the desktop `App`. Use the
  exclusive source lane and the pending-toggle guard.
- Verification: `go test ./internal/gui/`, `make gui-bindings`, and
  `go test ./... && go vet ./...` in `desktop`.

### P28-T04: Desktop

- Add the Receipts section to the Advisor view: the list, the empty state, the
  blocked cause, and the three confirmations.
- Add the receipts to the demo backend.
- Verification: `make gui-test`. Include tests for Clean up all with one
  blocked receipt and for Forget of a blocked receipt.

### P28-T05: Documentation

- Update `docs/usage.md`, `DESIGN.md`, and the wiki topic pages. Append an
  entry to `docs/wiki/log.md`.
- Verification: the wiki relative links resolve, `git diff --check` is clean,
  and each example uses synthetic names.

## Non-Goals

- A CLI command that cleans all receipts.
- A count or a badge of receipts outside the Advisor view.
- An automatic expiry of old receipts.
- A change to the first-party `skill-advisor` skill.
