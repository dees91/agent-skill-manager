# Public Text Review

This repository is public. **Public text** is anything published from it: pull
request titles and bodies, commit messages, issues, release notes,
documentation, wiki pages, planning files, test fixtures, demo data, and
screenshots. Public text describes behavior for every user.

Work usually starts from one concrete case on one machine. That case is useful
evidence while investigating. Public text keeps the lesson from it and leaves
the case behind.

## Steps

Run these before `git push`, and again before creating or editing a pull
request, issue, or release.

1. Collect the public text: the commit messages being pushed, the diff of
   documentation, planning, fixtures, and demo data, and the pull request body
   you are about to send.
2. Review it as a stranger who has never seen your machine. When your tool can
   start a subagent without the session history, give it only that text and
   this file; a reviewer without the investigation in context notices it
   leaking through. Otherwise re-read the text slowly as that stranger.
3. Rewrite every finding against the criteria below.

Done when every sentence would read the same for any user who installed any
source.

## Criteria

- **Problem as a class.** Describe the situation that triggers the behavior
  ("a managed repository gains skills after install"), not the event that
  exposed it.
- **Synthetic examples.** Use `example/agent-skills`, `https://github.com/example/...`,
  `/Users/example`, and skill names such as `alpha`, `beta`, `new-skill`.
  Choose names unrelated to the real sources involved in the investigation.
- **Verification by tests.** Report the commands, suites, and fixture scenarios
  that prove the behavior. Checks run against an author's own installation stay
  in the conversation.
- **Neutral numbers.** Counts, commit hashes, dates, and versions in examples
  come from fixtures or placeholders, never from an author's environment.
- **No local identity.** No real home paths, usernames, hostnames, state
  manifests, private repository URLs, credentials, or unredacted screenshots.

## Git Hook

`make setup` enables the tracked hooks in `.githooks/`. The `pre-push` hook
prints the commit messages and changed public-text files for the pushed range
next to these criteria. It detects nothing and blocks nothing; it marks the
last moment before text leaves the machine, for any agent or person running
`git push`.
