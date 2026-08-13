# Phase 12: GitHub Binary Release - Task Breakdown

> **Source**: user-approved GitHub release planning session (2026-08-13).
> This phase publishes version `0.4.0` as a private GitHub prerelease with
> locally built Apple Silicon desktop and CLI artifacts.

> **MANDATORY FOR ALL AGENTS**
>
> The Summary Table is the source of truth for task status. Set a task to
> `in-progress` before starting it, set it to `done` only after verification
> passes, and unblock dependents when all blockers are done.

## Product Decisions

- Build and verify release artifacts locally; do not add GitHub Actions.
- Publish `v0.4.0` as a prerelease in the private
  `dees91/agent-skill-manager` repository.
- Distribute an ad-hoc-signed Apple Silicon macOS application ZIP and a
  `darwin/arm64` CLI tarball, plus one SHA-256 manifest.
- Keep Developer ID signing, notarization, DMG packaging, universal binaries,
  CI release automation, and auto-update deferred.
- Store the release notes in the repository and document Gatekeeper approval
  in both README and the release notes.
- Add `skill-manager version` and `skill-manager --version`. Release builds
  embed the release version; development builds report `dev` unless Go module
  build information supplies a tagged module version.
- Keep release outputs ignored. `scripts/public-check.sh` must not be restored
  or added to Git history.
- Add one normal commit after the Phase 11 root commit; do not rewrite `main`.

## Summary Table

| ID | Task | Status | Blocked by |
|---|---|---|---|
| P12-T01 | CLI version interface | done | - |
| P12-T02 | Local release packaging | done | P12-T01 |
| P12-T03 | Download and Gatekeeper documentation | done | P12-T01 |
| P12-T04 | Commit, tag, verify, and publish prerelease | done | P12-T01, P12-T02, P12-T03 |

## Task Definitions

### P12-T01: CLI Version Interface

- Add equivalent `version` and `--version` entry points.
- Print `skill-manager <version>` and reject additional arguments.
- Cover development, linker-provided, and Go module build-info versions.

### P12-T02: Local Release Packaging

- Add `make release-package RELEASE_VERSION=<version>` backed by a dedicated
  release-packaging script.
- Require a clean `darwin/arm64` checkout and version agreement across desktop
  and frontend metadata.
- Run root, desktop, and frontend tests, vet, type checking, build, and npm
  high-severity audit checks before packaging.
- Produce and re-extract the desktop ZIP and CLI tarball, then verify their
  architecture, version, signature, bundle metadata, contents, and checksums.

### P12-T03: Download And Gatekeeper Documentation

- Add versioned release notes covering artifacts, checksum verification,
  install steps, compatibility, ad-hoc signing, and Gatekeeper approval.
- Update user, contributor, agent, and wiki documentation for the new release
  surface without implying notarization or platform support beyond macOS
  Apple Silicon.

### P12-T04: Commit, Tag, Verify, And Publish Prerelease

- Commit the verified implementation normally on top of the Phase 11 root and
  push `main` without force.
- Create and push annotated tag `v0.4.0` at that exact commit.
- Create a draft release with the repository-owned notes and the three exact
  assets, download it into a temporary directory, and repeat integrity and
  archive checks.
- Publish only after remote verification succeeds, then confirm the release is
  a non-latest prerelease with the expected tag and assets.
