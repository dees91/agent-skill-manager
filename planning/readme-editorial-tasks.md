# README Editorial Refresh

User-approved plan, 2026-09-08. Improve the first visit to the GitHub repository
using the existing marketing GIF and synthetic screenshots. Product behavior,
public APIs, release artifacts, and repository visibility are unchanged.

## Decisions

- Keep documentation in English and lead with the macOS application.
- Put the demo and concrete benefits before a numbered quick start, then
  explain reversible toggles, offer the terminal alternative, link to support,
  and finish with license, privacy, and third-party notices.
- Move detailed operator content into `docs/usage.md` and source builds into
  `CONTRIBUTING.md`. Keep README documentation links usable from the CLI archive.
- Preserve checksum checks, first-launch guidance, macOS preview limitations,
  source ownership, partial-failure behavior, and third-party instruction risks.
- Correct CLI directory/PATH setup and the `go install` binary-name mismatch.
- Add Muse/Grok to the privacy path inventory. Support the current public
  preview and `main` on a best-effort basis, without an SLA.
- Preserve the README version sentence consumed by the packaging script.

## Task Status

| ID | Task | Status | Blocked by |
| --- | --- | --- | --- |
| DOC-01 | Rewrite README and extract the complete operator guide | done | - |
| DOC-02 | Align build, privacy, support, and documentation routing | done | - |
| DOC-03 | Verify instructions, rendering, links, and required checks; update wiki | done | DOC-01, DOC-02 |

## Acceptance

- The opening identifies the product, supported tools, and download path.
- The quick start reaches an applied toggle and includes an empty-inventory path.
- Detailed commands, limitations, and recovery information remain reachable.
- Download verification works with either archive alone; installation works
  with a missing destination directory and a PATH that lacks it.
- README renders clearly at desktop and mobile widths, with valid anchors and
  descriptive image alternatives.
- Required repository checks pass; any unavailable verification is recorded.

## Verification Results

- README reduced from 458 to 181 lines; operator details retained in the guide.
- `make test-all`, root/desktop `go vet ./...`, `make vulncheck`, and frontend
  `npm audit --audit-level=high` passed. The frontend suite passed all 30 tests;
  the dependency checks reported no vulnerabilities.
- Real `v0.7.0` downloads passed each documented single-archive checksum check.
  Modified fixture copies were rejected. CLI installation created a missing
  destination, extended a minimal PATH, and reported `skill-manager 0.7.0`.
- A clean tracked-file export passed `make build`, CLI help, and `make dev`
  with an isolated `BIN` destination; that source binary reported `dev`.
- GitHub Markdown rendering was inspected locally at 1280×900 and 390×844.
  Images loaded, navigation anchors resolved, and mobile pages had no document
  overflow. File/anchor links, including full GitHub documentation URLs, were
  checked against the working tree; new guide URLs become live after publishing
  the files to `main`.
- The release packaging version sentence remains valid. The native macOS
  Gatekeeper flow was checked against Apple's guidance and existing app labels,
  but was not re-exercised with a fresh native installation.
