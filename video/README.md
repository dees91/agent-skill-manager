# Repository Marketing Media

This is the reproducible Remotion source for `.github/assets/demo.gif` and
`.github/assets/social-preview.png`. The editable story is in `STORYBOARD.md`;
`src/storyboard.ts` is its executable projection.

## Build

Requirements: Node.js 22.12 or newer, npm 10 or newer, and FFmpeg/ffprobe.

```bash
cd video
npm ci
npm run check
npm run render
npm run gif
npm run social-preview
npm run verify
```

Generated intermediates are written to `video/out/` and ignored by Git. The
optimized GIF and 1280x640 social preview are written to `.github/assets/` and
are tracked. `npm run verify` checks the video, GIF loop and size contract, and
the social preview's format, dimensions, and 1 MiB size limit.

After changing the social preview, upload `.github/assets/social-preview.png`
manually under the repository's **Settings → General → Social preview**. GitHub
does not use the tracked file automatically for repository cards.

To inspect the composition interactively:

```bash
npm run studio
```

## Refreshing Synthetic UI Captures

The committed captures are enough to render the video. Refresh them only when
the public desktop UI changes.

1. Start the demo frontend:

   ```bash
   cd desktop/frontend
   npm ci
   npm run dev -- --host 127.0.0.1 --port 4173
   ```

2. In a clean browser session with a 1440x960 viewport, capture Dashboard,
   Skills, and Sources from `http://127.0.0.1:4173`. Every capture must show
   the four managed tool columns: Claude, Codex, Muse, and Grok. For Skills,
   capture the
   initial state, click the Codex toggle for `dependency-review`, capture the
   pending state, click **Apply changes**, and capture the final state.
3. Save the five images under `video/public/ui/` using the names listed in
   `STORYBOARD.md`.
4. Confirm that every path, skill, group, URL, and count comes from the
   synthetic demo backend before committing the assets.

The optional capture step uses browser automation, but rendering itself depends
only on the files in this repository plus Node/npm and FFmpeg.
