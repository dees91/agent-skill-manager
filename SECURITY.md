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

Until tagged releases are published, only the current `main` branch is
supported. The desktop application is supported only on the macOS target
described in [README.md](README.md).

## Security Boundaries

Skill Manager validates and moves filesystem entries, invokes Git for explicit
repository operations, and can download metadata and skill files from
skills.sh. It does not sandbox or attest third-party skills. Treat every skill
source as executable instruction content and review it before installation.
