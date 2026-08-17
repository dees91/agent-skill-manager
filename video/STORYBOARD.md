# Skill Manager README Demo Storyboard

Revision: 3

Format: 20 seconds, 1920x1080, 30 fps, silent, seamless dark loop

Audience: Claude Code and Codex users considering the public macOS preview

## Purpose

Show the complete public-preview value proposition in one short loop: agent
skills live in several tool-specific locations, Skill Manager makes that local
inventory visible, and a user can stage and apply a reversible visibility
change without deleting the skill or changing its source.

The animation uses the product's own dark visual system and synthetic data.
It must remain understandable with no sound and when embedded at 960x540 in a
GitHub README.

The same source also owns a static GitHub social preview so the repository card
and README demo present one consistent product story.

## GitHub Social Preview

Format: static PNG, 1280x640, at most 1 MiB

- Product label: **Skill Manager**
- Headline: **Agent Skills, under control.**
- Supporting line: **Claude Code + Codex · local · reversible**
- Surface label: **macOS app · TUI · CLI**
- Visual: the repository-owned app icon beside a compact synthetic skill
  inventory using the same dark canvas, source colors, and state language as
  the desktop interface.
  - `release-checklist`: Claude ON, Codex ON
  - `ui-accessibility`: Claude ON, Codex OFF
  - `dependency-review`: Claude OFF, Codex ON
- Intent: make the repository purpose readable at normal GitHub card size and
  at a 640x320 reduced preview without suggesting cloud sync, telemetry, or
  unsupported functionality.

## Timeline

### 1. Scattered skills — 0.0s to 2.8s (frames 0–84)

- Visual: two floating filesystem cards for `~/.claude/skills` and
  `~/.agents/skills`, with small skill chips spread between them.
- Headline: **Agent skills, scattered across tools?**
- Motion: cards enter with restrained perspective and settle toward the center.
- Intent: establish the problem without implying cloud sync or remote control.

### 2. One local inventory — 2.6s to 5.5s (frames 78–165)

- Visual: a code-native terminal panel types the command and result:

  ```text
  $ skill-manager status
  ON: 12
  OFF: 3
  CONFLICT: 0
  RO: 3
  ```

- Supporting line: **One local inventory.**
- Intent: connect the desktop interface to the established terminal product.

### 3. Dashboard context — 5.2s to 8.6s (frames 156–258)

- Visual: synthetic Dashboard capture with a slow zoom toward visibility and
  provider context-budget information.
- Callout: **See visibility and estimated prompt cost.**
- Intent: show practical overview value, not merely a decorative dashboard.

### 4. Reversible toggle — 8.3s to 14.0s (frames 249–420)

- Visual: synthetic Skills captures. The Codex cell for
  `dependency-review` moves from ON, to a staged pending disable, to stable OFF
  after Apply.
- The refreshed workspace also shows favorite stars and the `Favorites` filter
  as discoverable supporting controls; the scene remains focused on the
  reversible toggle and does not imply that favoriting changes visibility.
- Callout sequence: **Stage first.** → **Apply once.** → **Restore anytime.**
- Motion: pointer movement and click rings explain the interaction; the app
  captures supply the exact product state.
- Intent: make the safety model concrete. The skill is hidden from discovery,
  not deleted, rewritten, or uninstalled.

### 5. Source ownership — 13.7s to 17.3s (frames 411–519)

- Visual: synthetic Sources capture showing a managed Git repository and a
  linked local folder, with Claude and Codex counts.
- Callout: **Git repositories and linked folders.**
- Intent: show that installation source and ownership remain explicit.

### 6. Product close — 17.0s to 20.0s (frames 510–600)

- Visual: Skill Manager app icon and name on the dark canvas.
- Headline: **Keep every skill. Load only what you need.**
- Footer: **Claude Code + Codex · local · reversible**
- Motion: the close fades fully to the opening canvas so the GIF loops without
  a flash.

## Visual Direction

- Use the repository's `DESIGN.md` tokens: near-black canvas, dark panels,
  cyan/blue informational accents, and orange for primary actions.
- Use moderate perspective, scale, opacity, and functional cursor pulses.
- Keep key copy inside a generous safe area. Avoid particles, fine noise, and
  gradients that inflate GIF size without improving comprehension.
- Use system sans-serif and monospace stacks only. No web fonts, music, stock
  footage, external logos, or borrowed assets from the reference project.

## Synthetic Capture Contract

Tracked captures live in `public/ui/` and are made from
`desktop/frontend/src/demoBackend.ts`. Required states:

- `dashboard.png`
- `skills-before.png`
- `skills-pending.png`
- `skills-after.png`
- `sources.png`

The captures must use the demo backend's `example-labs`, `sample-org`, and
`/Users/example` fixtures. If a capture contains a real username, home path,
skill inventory, account, or repository, it is invalid and must not be kept.

## Future Update Protocol

When public functionality changes:

1. Increment the storyboard revision and edit the timeline or copy here.
2. Mirror the accepted timing, copy, and asset names in `src/storyboard.ts`.
3. Refresh only captures whose UI state changed.
4. Update scene implementation if the story needs a new visual treatment.
5. Render the master, regenerate the GIF and social preview, and review a
   contact sheet plus the social preview at full and reduced size.
6. Re-run privacy scans and verify the README still describes the released
   public surface accurately.
