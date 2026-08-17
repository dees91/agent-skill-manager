# Security Policy

## Reporting A Vulnerability

Please report suspected vulnerabilities through
[GitHub private vulnerability reporting](https://github.com/dees91/agent-skill-manager/security/advisories/new).
Do not include credentials, private skill contents, home-directory listings, or
exploit details in a public issue.

Include the affected commit or version, operating system, reproduction steps,
expected impact, and whether the issue can cause an unintended filesystem or
Git mutation. A minimal reproduction using a temporary home directory is
preferred.

There is currently no guaranteed response or remediation SLA. Reports will be
triaged as time permits, and coordinated disclosure is requested until a fix or
mitigation is available.

## Supported Versions

The current `0.4.x` public preview and the current `main` branch receive
best-effort support. The desktop application is supported only on the macOS
target described in [README.md](README.md).

## Security Boundaries

Skill Manager validates and moves filesystem entries and invokes Git for
explicit repository operations. It does not sandbox or attest third-party
skills. Treat every skill source as executable instruction content and review
it before installation. The dormant skills.sh adapter is not exposed by the
public preview desktop binding or interface.

The first-party Skill Advisor treats installed skill catalog metadata as
untrusted discovery data and never executes it as a command. Advisor
activations are limited to exact tool/skill cells, serialized through a
no-follow owner-only lock, and restored only after path, entry-type, and
symlink-target validation. Receipt metadata contains no prompt or task text.
