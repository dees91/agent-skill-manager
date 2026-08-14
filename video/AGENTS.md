# Demo Video Agent Brief

This directory contains the reproducible source for the README demo animation.

## Source Precedence

When updating the demo, use this order:

1. The current user's request.
2. `STORYBOARD.md`, the human-readable product story and review contract.
3. `src/storyboard.ts`, the executable timing, copy, and asset projection.
4. Scene and component implementation.

Keep `STORYBOARD.md` and `src/storyboard.ts` aligned. Marketing copy and scene
timings belong in the storyboard projection, not scattered through scene code.

## Privacy And Product Accuracy

- Use only synthetic skill names, groups, paths, repository URLs, and counts.
- Never capture the real user's home, installed skills, provider state, email,
  account, or filesystem inventory.
- Capture UI assets from the frontend's built-in demo backend.
- Show only functionality available in the public preview. Do not expose the
  dormant Discover interface until it becomes a supported public feature.

## Output Contract

- Composition: 1920x1080, 30 fps, 600 frames (20 seconds), no audio.
- Master: `out/demo-master.mp4` (generated and ignored by Git).
- README asset: `../.github/assets/demo.gif`, infinite loop, at most 10 MiB.
- The GIF conversion script may reduce frame rate, palette size, or resolution
  in that order, but must not shorten the story without user approval.

## Update Workflow

1. Update `STORYBOARD.md` and increment its revision.
2. Update `src/storyboard.ts` and any synthetic UI captures.
3. Run `npm ci`, `npm run check`, and `npm run render`.
4. Run `npm run gif`, `npm run verify`, and inspect the output.
5. Update the README or wiki when the public product story changes.
