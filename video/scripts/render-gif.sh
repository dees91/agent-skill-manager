#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
video_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${video_dir}/.." && pwd)"
input="${video_dir}/out/demo-master.mp4"
output="${repo_root}/.github/assets/demo.gif"
limit_bytes=$((10 * 1024 * 1024))

command -v ffmpeg >/dev/null || { echo "ffmpeg is required" >&2; exit 1; }
command -v ffprobe >/dev/null || { echo "ffprobe is required" >&2; exit 1; }
test -f "${input}" || { echo "Render the master first: npm run render" >&2; exit 1; }
mkdir -p "$(dirname "${output}")"

render_gif() {
  local fps="$1"
  local size="$2"
  local colors="$3"
  ffmpeg -hide_banner -loglevel warning -y -i "${input}" \
    -filter_complex "fps=${fps},scale=${size}:flags=lanczos,split[source][palette_input];[palette_input]palettegen=max_colors=${colors}:stats_mode=diff[palette];[source][palette]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" \
    -loop 0 "${output}"
}

render_gif 15 "960:540" 192
if (( $(stat -f%z "${output}") > limit_bytes )); then
  echo "GIF exceeds 10 MiB; retrying at 12 fps and 160 colors."
  render_gif 12 "960:540" 160
fi
if (( $(stat -f%z "${output}") > limit_bytes )); then
  echo "GIF still exceeds 10 MiB; retrying at 864x486."
  render_gif 12 "864:486" 160
fi
if (( $(stat -f%z "${output}") > limit_bytes )); then
  echo "Unable to keep ${output} under 10 MiB without changing the storyboard." >&2
  exit 1
fi

duration="$(ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 "${output}")"
size_bytes="$(stat -f%z "${output}")"
echo "Wrote ${output} (${size_bytes} bytes, ${duration}s)."
