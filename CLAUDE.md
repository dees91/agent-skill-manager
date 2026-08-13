# Claude Code Guidance

This repository uses [AGENTS.md](./AGENTS.md) as the canonical project brief for all agents.

Before making code or documentation changes:

1. Read [AGENTS.md](./AGENTS.md).
2. Read [docs/wiki/README.md](./docs/wiki/README.md) and [docs/wiki/index.md](./docs/wiki/index.md), then open the topic pages relevant to the task.
3. If working on the MVP implementation, read [planning/phase-1-mvp-tasks.md](./planning/phase-1-mvp-tasks.md).
4. If working on group or bulk toggle behavior, read [planning/phase-2-group-bulk-toggle-tasks.md](./planning/phase-2-group-bulk-toggle-tasks.md).
5. If working on repository installation behavior or documentation, read [planning/phase-3-repo-install-tasks.md](./planning/phase-3-repo-install-tasks.md).
6. If working on repository update or uninstall behavior or documentation, read [planning/phase-4-repo-update-uninstall-tasks.md](./planning/phase-4-repo-update-uninstall-tasks.md).
7. If working on local path install or uninstall behavior or documentation, read [planning/phase-5-local-path-install-tasks.md](./planning/phase-5-local-path-install-tasks.md).
8. If working on the desktop app, read the relevant Phase 6-10 planning file.
9. If working on publication or repository hygiene, read [planning/phase-11-publication-readiness-tasks.md](./planning/phase-11-publication-readiness-tasks.md).
10. If working on GitHub release artifacts, read [planning/phase-12-github-release-tasks.md](./planning/phase-12-github-release-tasks.md).
11. Follow the task status rules in the relevant planning file before starting and after finishing work.

After non-trivial work that creates reusable knowledge, update the relevant wiki topic/source pages and append to [docs/wiki/log.md](./docs/wiki/log.md). The wiki is a synthesis layer; verify current behavior against code and tests and keep accepted product decisions in `AGENTS.md` and the relevant planning file.

Do not implement behavior that contradicts the decisions recorded in [AGENTS.md](./AGENTS.md) without first updating the plan with the user.
