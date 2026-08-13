# Skill Manager LLM Wiki

This directory is the persistent, LLM-maintained knowledge layer for
`skill-manager`. It follows the [LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f): agents compile reusable knowledge into an
interlinked Markdown wiki instead of reconstructing the same context from the
repository on every task.

The wiki is for architecture, behavior, invariants, trade-offs, historical
context, and open questions. It complements the source tree; it does not replace
the project's authoritative product and planning documents.

## Layers

`Raw sources`

- Go source and tests under `internal/` and `main.go`.
- Product decisions in `AGENTS.md`.
- User-facing behavior in `README.md`.
- Iteration scope and task status under `planning/`.
- Git history, command output, and external references used for a task.
- Wiki maintenance may read and summarize these sources, but must not change
  them merely to make the wiki agree with an assumption.

`Wiki`

- LLM-maintained synthesis under `docs/wiki/`.
- `sources/` contains dated or source-specific digests.
- `topics/` contains the current cross-source understanding of a subject.
- Agents own the bookkeeping: links, summaries, contradictions, status labels,
  index entries, and the maintenance log.

`Durable decisions`

- The wiki is not the final authority for product scope or implementation
  decisions.
- When the user accepts a change to scope, semantics, paths, or UX, update
  `AGENTS.md` and the relevant planning document as required by the project
  contract. Update `README.md` when user-facing behavior changes.
- Then bring the wiki synthesis up to date and record the change in `log.md`.

## Source Precedence

When sources disagree, use this order:

1. The user's current instruction or correction.
2. `AGENTS.md` for accepted product intent and invariants.
3. The relevant planning file for iteration scope and task status.
4. Current code and tests for what the program actually does.
5. `README.md` for the documented user-facing contract.
6. Wiki source digests and topic pages.
7. Older discussion or assistant synthesis.

Code can prove current behavior without overriding accepted intent. Preserve a
meaningful mismatch as a contradiction or open item instead of silently
choosing one side.

## Claim Labels

Use labels when a claim's status is important:

- `documented`: stated by an authoritative project document.
- `implemented`: verified in current code, preferably with a supporting test.
- `observed`: verified from the current filesystem, command output, or session.
- `user-confirmed`: explicitly confirmed by the user.
- `inferred`: a cautious conclusion assembled from available evidence.
- `planned`: accepted future work that is not implemented yet.
- `open`: unresolved, unverified, or awaiting a decision.
- `superseded`: retained historical understanding replaced by newer evidence.

Do not label every sentence. Use labels where a reader could otherwise confuse
intent, current behavior, inference, and future work.

## Required Files

- `index.md` is the content map. Update it whenever a page is added, renamed,
  removed, or materially repurposed.
- `log.md` is append-only. Add an entry for every ingest, saved query, major
  synthesis update, decision, lint pass, and maintenance operation.

Use this log heading format:

```text
## [YYYY-MM-DD] type | Short title
```

Preferred types are `setup`, `ingest`, `query`, `analysis`, `decision`, `lint`,
`maintenance`, `implementation`, and `verification`.

## Workflows

### Before Non-Trivial Work

1. Read this file and `index.md`.
2. Open the topic pages relevant to the task.
3. Consult source digests for history and routing, then verify important or
   mutable claims against the authoritative raw sources.
4. Read the relevant planning file before changing iteration behavior.

### Ingest Or Update

1. Identify the input: code change, planning decision, user correction,
   external source, command observation, or analysis result.
2. Add or update a page under `sources/` when the input benefits from its own
   traceable digest. Do not duplicate source files wholesale.
3. Update every affected topic page, including contradictions and open items.
4. Update `index.md` when pages changed structurally.
5. Append one concise entry to `log.md`.

A single source may update several topic pages. Prefer enriching existing pages
over creating a new page for every conversation.

### Query

1. Read `index.md`, then the smallest relevant set of topic and source pages.
2. Verify claims whose answer depends on current code, tests, filesystem state,
   or recent external information.
3. Cite local pages or raw files with relative links.
4. If the answer creates reusable synthesis, file it into the wiki and log it.

### Lint

Periodically check for:

- wiki claims that conflict with `AGENTS.md`, planning status, code, or tests;
- orphan pages missing from `index.md` or pages with no useful inbound link;
- broken relative links and renamed code paths;
- stale `planned`, `open`, or `superseded` claims;
- important architecture or safety concepts without a topic page;
- duplicate explanations that should be consolidated;
- secrets, credentials, tokens, or unnecessary raw command output.

Record the lint result in `log.md`, including a clean result.

## Style And Boundaries

- Write wiki pages in concise English to match project documentation. Speak to
  the user in Polish unless they request another language.
- Prefer concrete bullets and small diagrams over long narrative.
- Use stable kebab-case filenames and relative Markdown links.
- Link to raw sources instead of copying large code or document fragments.
- Do not store credentials, private keys, tokens, or personal data.
- Wiki maintenance alone must never mutate real global skill directories under
  the user's home. Runtime tests must continue to use temporary homes.
- Do not commit, push, publish, or upload the wiki unless the user explicitly
  asks for that separate action.
