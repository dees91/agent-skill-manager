<!-- to-plan:conversation-plan:v1 id=8852f34c-7098-43d1-b231-627c56e97541 -->

# Phase 24: Optional TypeSafe advisor with BYOK

**Status:** implemented.
**Source:** user-requested implementation plan, corrected to one complete agent run on 2026-09-19.
**Planned against:** `main` at `7afd675`.
**Local state:** committed baseline at `7afd675` with a clean worktree.
Implementation started 2026-09-21. Cloud recommendation sends the full eligible
host catalog in token-budgeted parallel chunks; BM25F remains the
`local_candidates` fallback list only.

This is one end-to-end implementation assignment for one agent run. Phase 24 is
only the repository's planning identifier, not a sequence of separately assigned
phases. Set the single task to `in-progress`, implement the complete feature,
verify it, update documentation, and report the result. Do not stop for approval
between work items or return after delivering only a prototype, CLI, or backend.
Keep this file as the persistent task-status record. This planning request does
not start implementation.

## Approach

Add an optional TypeSafe/Jev recommendation step after local skill retrieval.
Users supply their own API key and explicitly enable cloud recommendations.
The local Go process calls TypeSafe directly; no Skill Manager backend, account,
shared key, or billing service is introduced. The existing local workflow stays
usable without credentials or a network connection.

Implement and test the integration in the same run, including a compact quality
evaluation fixture set. A paid comparative research benchmark is not a
prerequisite for implementation. Do not claim proven selection improvement
without measured evidence. Recommendation never mutates skills.
The calling agent remains responsible for final selection, reading instructions,
activation, and cleanup through the existing receipt/lease API.

## Guardrails

- `advisor search`, legacy list filtering, startup, scan, refresh, and TUI
  remain local. Existing JSON shapes and `ranked_search_v1` remain compatible.
- A present API key alone is not permission to send anything. Default mode is
  `local`; cloud mode requires an explicit user setting or per-call opt-in.
- Send only the caller-supplied task brief plus names and bounded descriptions
  of every toggleable skill of the selected host, in as many parallel chunk
  requests as the token budget requires (never dropping skills; at most 16
  chunks). Request bound 256 KiB; request count is chunks + 1. Do not read
  transcripts, project files, or full skill bodies for this release. Omit home
  paths, Git origins, group/source labels, and provider settings. Metadata can
  still be private; disclose this.
- No persistent task/request/response cache, prompt logging, telemetry, polling,
  automatic source installation, or automatic changes to skill state.
- Return only validated eligible local skill identifiers. Provider text is
  untrusted data, never executable instructions or filesystem operations.
- Keep activation limited to five skills for one tool, preserve baseline-ON
  cells, share existing leases, and clean only the invocation's exact receipt.
- No other model providers, generic provider marketplace, autonomous phase
  detection, transcript monitoring, or claims to unload an agent's context.
- Do not advertise supported behavior before code and tests exist. Update
  README, usage, privacy and security documents as implementation lands.

## Planning decisions

### Existing seams

- `internal/advisor/search.go`: reuse `Search`, `SearchOptions`, eligibility,
  deterministic ordering, and bounds. Current search accepts 256 runes and
  32 tokens; do not send an entire task through that query argument.
- `internal/cli/advisor.go`: extend `App.runAdvisor` with a distinct operation,
  reusing structured errors and path-free inventory projection.
- `internal/advisor/types.go`: add a capability without changing API version 1.
- `internal/advisor/service.go` and `store.go`: activation and receipt ownership
  remain unchanged. Recommendation must not hold the mutation lock across HTTP.
- `internal/gui.Service`, `desktop/app.go`, and `desktop/frontend/src/api.ts`:
  keep provider access and credential ownership in Go. React receives safe
  configuration/status projections, not a stored secret.
- `skills/skill-advisor/SKILL.md`: the agent supplies task context. Desktop
  settings alone cannot observe tasks in other agent applications.

### Recommendation contract

Proposed new command, not available in the baseline:

```text
skill-manager advisor recommend --tool <tool> --query <short-query>
  --task-stdin [--provider local|typesafe] [--json]
```

- Require one existing tool and the existing bounded retrieval query. Read a
  bounded task brief from stdin, at most 8,192 UTF-8 bytes. Reject malformed,
  oversized, or blank input before scanning or networking. Never echo it.
- Provider precedence: explicit flag, saved advisor preference, then `local`.
  `--provider typesafe` is explicit consent for that invocation; the official
  advisor skill must not add it without a user's instruction. A saved cloud
  preference represents standing consent within the disclosed input boundary.
- Retrieve at most 20 local BM25F candidates for the fallback list only. Cloud
  stage 1 sends every eligible toggleable skill of the host (name plus a
  512-byte description) in token-budgeted chunks. Stage 2 re-judges at most
  five skills with 2,048-byte descriptions. Bound serialized provider requests
  to 256 KiB and enforce the current provider token budgets separately; bytes
  are not token counts. Exceeding 16 chunks returns `local_candidates` with
  `catalog_too_large` before any request.
- Return `apiVersion`, `tool`, `requestedProvider`, `usedProvider`, `outcome`,
  `fallbackReason`, path-free `candidates`, and `recommendedSkills`.
  Outcomes are `recommended`, `none`, and `local_candidates`.
- Local mode and operational fallback return local candidates in original
  ranked order and an empty `recommendedSkills`. They do not pretend that
  BM25F produced a confident model selection. A successful Jev rejection returns
  `none`, not an outage or an automatic fallback to the first candidate.
- Cloud success returns zero to five unique names from the eligible shortlist.
  Revalidate eligibility against current local state before returning. Existing
  activation performs its own fresh filesystem checks later.
- Invalid arguments and local scan failures remain errors. Missing/denied keys,
  HTTP/auth/quota failures, timeouts, malformed model output, or changed local
  candidates return `local_candidates` with a stable, non-sensitive reason.
  Cancellation stops promptly and does not trigger another request.
- Add `semantic_recommendation_v1` to `advisor status`; expose the configured
  provider without reading the OS credential store or making an HTTP request. Do not expose
  a key, task, paths, provider response body, or account identifiers.

### Semantic evaluation

Use the current documented HTTP contract with Go `net/http`; a Python/JS runtime
or SDK is unnecessary. Pin a tested model version and prompt schema revision.
The research starting point is `jev-1.13.0`, not an indefinitely moving alias.
Recheck availability and input limits when implementation begins.

First batch: evaluate candidate relevance to the task using independent narrow
Noul/Score questions over shared state. Keep rubric meanings consistent across
candidates. Second batch, only when justified by the first result: verify up to
five candidates for necessity and distinct contribution with an explicit
no-match outcome. Independent questions cannot read each other's answers; build
new state for the second request. Tune thresholds on development fixtures only.

Use one provider request per catalog chunk plus one stage-2 request, a 10-second
total initial timeout budget, and no automatic retry. Any chunk failure falls
back the whole run; do not return a partial catalog. Measure latency during any
authorized live check. Do not automatically retry a paid evaluation after an
unknown timeout; return local candidates.
Reject redirects, cap response size, validate finite/ranged numeric fields and
known IDs, and sanitize provider errors. Production uses the fixed HTTPS
TypeSafe origin; tests inject a transport, not an end-user arbitrary endpoint.
No assumption that Choice/Score confidence proves correctness or permission.

### Credentials and preferences

- Store only provider choice and configuration schema version in a separate
  `advisor-settings.json` under the existing private state directory, with
  atomic 0600 writes. Do not change `state.json` or receipt schema versions.
- Resolve an explicit `TYPESAFE_API_KEY` environment value first, then an item
  in the OS credential store owned by Skill Manager. The key does not
  imply cloud consent, and environment credentials are never copied to disk.
- Use a narrow injectable credential-store interface and a native macOS
  Security.framework implementation. Provide a non-macOS/non-cgo unavailable
  store for tests and environment-only use; retain the Linux root CI build.
  Do not place the secret in subprocess arguments, shell commands, or logs.
- Add CLI setup/status/removal operations under `advisor provider`. Setup reads
  secrets through hidden terminal input or explicit stdin, never a `--key`
  value. Key validation is a separate explicit action; saving alone is offline.
- Removing the stored key also sets the saved provider to local. An environment
  override is process-owned and cannot be deleted by the application; report
  its presence safely. A denied or locked OS credential store returns local fallback.
- GUI key input is transient, masked, sent once to Go, and cleared after save
  or cancellation. No localStorage, snapshots, demo payloads, or return binding
  contains the saved key. Test access from the packaged CLI and desktop app;
  OS credential store denial, app rebuild, and signature changes must fail safely.

### Desktop and advisor skill

Add a compact Advisor settings panel accessible from the existing app shell,
with Local/TypeSafe selection, data-sharing explanation, masked key entry,
explicit connection check, and remove-key action. Saving cloud mode records
consent. Local remains the first-run default. No standalone chat or automatic
recommendation call on app launch/refresh is part of this phase.

The first-party advisor skill checks capabilities and configured mode. It keeps
using `advisor search` for local mode. For opted-in cloud mode it supplies a
concise task brief through stdin to `advisor recommend`, treats returned names
as suggestions, can reject them, and retains the existing activation and
cleanup protocol. Older binaries retain the existing local search path.

## Summary Table

| ID | Task | Status |
| --- | --- | --- |
| P24-T01 | Implement and verify optional TypeSafe BYOK recommendations across Go, CLI, desktop, and the first-party advisor | implemented |

## Execution in one run

The following are internal work items for P24-T01, not independent milestones
or approval gates. Complete all of them before handing off.

1. **Recommendation service and provider adapter.** Add
   `internal/advisor/recommend.go` and typed results; reuse `Search` without
   changing local ranking. Add a small `internal/typesafe/` HTTP client with
   injected transport, cancellation, fixed HTTPS origin, bounded input/output,
   response validation, and explicit local fallback. Keep receipts and mutation
   locks outside network calls. Test local, cloud, no-match, stale candidates,
   invalid IDs, malformed responses, 401/429/5xx, redirects and timeouts using
   fake transport or `httptest`. Never automatically retry an unknown paid call.
2. **Credentials, preferences and CLI.** Add `internal/advisor/settings.go`, a
   narrow `internal/credentials/` store with native macOS implementation and
   non-macOS/non-cgo fallback, and provider configuration operations in
   `internal/cli/advisor_provider.go`. Wire `recommend` in `App.runAdvisor` with
   bounded stdin input and additive capability metadata. Test environment/key
   precedence, explicit consent, removal, denied OS credential store access, corrupted
   settings, output redaction, and compatibility with existing search/status.
3. **Desktop configuration.** Add `internal/gui/advisor.go`, thin bindings in
   `desktop/app.go`, and an `AdvisorSettings.tsx` panel accessible from the
   existing shell. Update `api.ts`, test/demo adapters, and generated bindings.
   Share the Go settings/store service with CLI. Cover local default, masked
   transient input, explicit connection check, removal and clearing input on
   save/cancel. No cloud request or OS credential store read during ordinary refresh.
4. **First-party advisor.** Update `skills/skill-advisor/SKILL.md` and
   `internal/advisor/skill_contract_test.go` for configured-mode/capability
   negotiation, a concise stdin task brief, and local compatibility. Returned
   names remain suggestions; preserve same-turn instruction reads, at-most-five
   selection, baseline-ON behavior, lease sharing and exact-receipt cleanup.
5. **Validation and documentation.** Run focused tests during implementation,
   fix failures, then run the complete applicable checks below. Synchronize
   AGENTS, CLAUDE, usage, privacy, security, design, wiki, and dependency notices.
   Add a concise README route once implemented. Record actual test results and
   limitations, mark P24-T01 appropriately, and return one completion report.

Use the existing package-level tests as the primary seams. New tests belong in
`internal/advisor/recommend_test.go`, `internal/advisor/settings_test.go`,
`internal/typesafe/client_test.go`, `internal/credentials/store_test.go`,
`internal/cli/advisor_recommend_test.go`, `internal/gui/advisor_test.go`, and
`desktop/frontend/src/components/AdvisorSettings.test.tsx`, or equivalent local
files. Add tests for observable contracts; avoid tests that only mirror helpers.

## Quality evaluation within the assignment

Create a compact synthetic fixture set under
`internal/advisor/testdata/recommendation/`, covering clear matches, no-match,
several valid skills, near-duplicate descriptions, Polish/English requests,
misleading metadata, and skills absent from the retrieved shortlist. Label
acceptable sets and exclusions before prompt adjustment. Keep a held-out subset.

Provide an opt-in live evaluation entry point and a short methodology. Reuse
the production provider adapter; do not build a separate evaluation platform.
Report selection errors, useful-selection recall, no-skill false selections,
retrieval recall, billed usage, and latency when observed. Compare against the
existing local-plus-agent workflow only when that baseline can actually run;
do not present raw BM25F candidates as the agent's final decisions.

Routine tests use fake responses and make no paid calls. If live credentials and
explicit authorization are available, run the small synthetic check in the same
run. Otherwise finish the implementation and all independent verification,
report live quality/cost/latency as unverified, and do not claim improvement.
Lack of an API key does not block the implementation. Provider behavior that
violates the contract must fall back safely; do not loosen safety checks to make
live results look successful. Broader statistical benchmarking is optional
follow-up, not a second mandatory implementation assignment.

## Acceptance coverage

| Acceptance criterion | Verification |
| --- | --- |
| Local and cloud recommendation paths are implemented end to end | CLI/service integration tests with injected provider |
| Existing search/status/startup remain offline | Failing-on-use transport/store doubles |
| Key presence does not imply consent | Environment-only and first-run tests |
| Keys remain outside files, arguments and returned bindings | Store/CLI/GUI sentinel-secret assertions |
| User can revoke cloud mode and remove the stored key | Settings and removal tests |
| Cloud receives only bounded approved input | Serialized request assertions |
| Unavailable, uncertain and no-match results remain distinct | Typed outcome/error fixtures |
| Untrusted output cannot mutate skills or select foreign IDs | Invalid-output and lease-isolation tests |
| Desktop and CLI share settings and work without credentials | GUI/CLI integration tests and desktop walkthrough |
| Existing source ownership and receipts remain valid | Existing advisor/CLI regression suites |
| Evaluation is reproducible and claims match the evidence | Labeled fixtures, opt-in driver, accurate validation report |

## Final validation and completion

- During implementation run the relevant package tests, such as
  `go test ./internal/advisor/... ./internal/cli/...`, plus new provider,
  credentials, and GUI suites as they are added.
- Run `go test ./...` and `go vet ./...` from the root.
- Run `make gui-bindings`, `make gui-test`, `make gui-build`, and
  `go vet ./...` from `desktop/` after the frontend build.
- Run `make notices-check`, `make vulncheck`, and frontend
  `npm audit --audit-level=high` when dependencies change.
- Preserve root Linux and macOS desktop CI compatibility. Do not publish a PR
  merely to run CI; report cross-platform checks not executed locally.
- Exercise native OS credential store storage and packaged CLI/desktop access with a
  temporary synthetic key item, including denied access and removal. Never use
  real provider credentials in storage tests. Remove the exact test item.
- Run strict wiki validation, relative-link checks, `git diff --check`, and the
  repository public-text review. Preserve unrelated worktree changes.
- If an external service or OS permission prevents a check, finish all
  independent implementation and checks, then name the exact remaining check
  and its impact. Do not claim a failed or unrun check passed. Do not use a
  missing live API key as a reason to stop after an intermediate work item.
- Completion means the complete optional feature, UI, advisor integration,
  documentation, and credential-free automated checks are delivered. Live
  model-quality verification has a separate evidence status in the report.
  A failing required implementation check keeps the task incomplete.
- No commit, push, or release is implied. Return one report with implemented
  behavior, verification results, and any remaining external validation.

Planning-time verification: existing
`go test ./internal/advisor/... ./internal/cli/...` passed. The referenced code
seams, build targets, and CI workflows exist. New files and behaviors above are
implementation targets. No paid inference, OS credential store mutation, application
rebuild, or GUI validation was performed while recording this plan.

## Review focus

Network consent; shortest useful task context; metadata prompt injection;
confusing model confidence with correctness; fallback being mistaken for a
recommendation; credential access across CLI/desktop signatures; cancellation
and duplicate billing; regression in local retrieval or receipt ownership.

## Allowed deviations

Private file/helper names, test layout, equivalent credential-store implementation,
and internal work order may change if behavior, secrecy, local defaults, compatibility,
and acceptance coverage remain intact. Keep one provider and one shared Go
service. Record deviations in this plan and the wiki.

Recorded during implementation:

- Serialized request bound is 256 KiB (full-catalog chunks) instead of 64 KiB.
- Noul `criteria` is the live API object `{true,false}` rather than a string.
- Cloud input is the full eligible host catalog in parallel chunks, not a
  BM25F shortlist. Request count is chunks + 1.

## Re-plan triggers

Need to upload full instructions, code or sessions;
a hosted proxy, shared billing or additional provider becoming necessary;
OS credential store support requiring secrets in command arguments; breaking local search
or activation semantics; endpoint/model changes invalidating the measured
contract; unsupported platform claims. Do not silently expand scope to solve
these conditions. Routine implementation choices and test failures are work
to resolve within the same run, not requests for intermediate approval.

## External references

Read live docs again before implementation; the following were checked during
planning research on 2026-09-19:

- [HTTP API](https://docs.typesafe.ai/api)
- [Models and limits](https://docs.typesafe.ai/models)
- [Skill suggestion cookbook](https://docs.typesafe.ai/cookbooks/skill_suggestion)
- [Confidence](https://docs.typesafe.ai/confidence)
- [Jev limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13)

The provider cookbook supports trying this design; its benchmark results do
not establish quality on this application's catalog or agent workflow.
