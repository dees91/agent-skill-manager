# Contributing

Contributions are welcome when they preserve Skill Manager's reversible
filesystem and ownership model.

## Development Setup

The complete project requires:

- macOS 13 or newer on Apple Silicon for supported GUI packaging;
- Go 1.26.6 or newer;
- Node.js 22.12 or newer and npm 10 or newer;
- Git;
- Xcode command-line tools for Wails builds.

Clone the repository and run:

```bash
go test ./...
make gui-test
```

`make gui-test` uses `npm ci`, then typechecks, tests, and builds the frontend
before compiling the desktop module. Do not rely on an existing ignored
`frontend/dist` directory.

## Project Contract

Before non-trivial changes:

1. Read [AGENTS.md](AGENTS.md).
2. Read [docs/wiki/README.md](docs/wiki/README.md) and
   [docs/wiki/index.md](docs/wiki/index.md).
3. Open the relevant planning and wiki topic pages.
4. Verify mutable claims against code and tests.

Accepted changes to scope, paths, semantics, or UX must update the appropriate
authoritative documents and then the wiki synthesis and log.

## Safety Rules

- Automated tests must use temporary homes and local or fake Git repositories.
- Never run mutation tests against real `~/.claude`, `~/.agents`, `~/.codex`,
  or `~/.skill-manager` paths.
- Do not overwrite blockers, dereference moved symlinks, edit third-party
  `SKILL.md` files, or mutate external manager lockfiles.
- Keep frontend mutation calls identifier-based; do not accept prebuilt
  filesystem operations from the UI.
- Keep source lifecycle changes all-or-nothing through preflight and preserve
  the documented rollback boundaries.

## Verification

Run focused tests while working, then complete:

```bash
make test-all
```

Before submitting a change, also run `go vet ./...` in the root and desktop Go
modules, `make vulncheck`, and `npm audit --audit-level=high` in
`desktop/frontend`.

For GUI changes, also run `make gui-build` and verify the ad-hoc signature,
ARM64 architecture, launch behavior, relevant viewport sizes, keyboard use,
and accessible names. Regenerate Wails bindings when the bound Go API changes.

## Release Packaging

Release artifacts are built locally on Apple Silicon macOS from a clean
checkout. The packaging command does not create tags, push commits, or change
GitHub releases:

```bash
make release-package RELEASE_VERSION=0.4.1
```

It verifies version metadata, root/desktop/frontend tests and vet, frontend
type checking/build, the npm high-severity audit, Wails packaging, ad-hoc
signatures, thin ARM64 binaries, bundle metadata, isolated-home launch,
re-extracted archives, third-party notices, and SHA-256 sums. Successful output
is ignored under `dist/release/`.

For an approved version, commit first and run the command from that clean
commit. Push `main` without force, create and push an annotated `v<version>`
tag at the verified commit, and create a draft prerelease with
`docs/releases/v<version>.md` and all three generated files. Download the draft
assets into a temporary directory and repeat checksum, archive, signature,
architecture, and version checks before publishing it as a non-latest
prerelease. GitHub Actions and automatic publishing are intentionally absent.

## Pull Requests

Keep changes focused and explain their user-visible behavior, safety impact,
tests, and any deferred follow-up. Do not include state manifests, real home
paths, private repository URLs, credentials, local screenshots, build outputs,
or `.DS_Store` files.
