#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
video_dir="$(cd "${script_dir}/.." && pwd)"
repo_root="$(cd "${video_dir}/.." && pwd)"
master="${video_dir}/out/demo-master.mp4"
gif="${repo_root}/.github/assets/demo.gif"
limit_bytes=$((10 * 1024 * 1024))

command -v ffprobe >/dev/null || { echo "ffprobe is required" >&2; exit 1; }
command -v xxd >/dev/null || { echo "xxd is required" >&2; exit 1; }
test -f "${master}" || { echo "Missing ${master}" >&2; exit 1; }
test -f "${gif}" || { echo "Missing ${gif}" >&2; exit 1; }

master_meta="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,nb_frames -of csv=p=0:s=x "${master}")"
master_meta="${master_meta%x}"
master_types="$(ffprobe -v error -show_entries stream=codec_type -of csv=p=0 "${master}")"
master_types="${master_types%,}"
master_duration="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "${master}")"
test "${master_meta}" = "1920x1080x30/1x600" || { echo "Unexpected master metadata: ${master_meta}" >&2; exit 1; }
test "${master_types}" = "video" || { echo "Master must contain one video stream and no audio: ${master_types}" >&2; exit 1; }
test "${master_duration}" = "20.000000" || { echo "Unexpected master duration: ${master_duration}" >&2; exit 1; }

gif_meta="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height,r_frame_rate,nb_frames -of csv=p=0:s=x "${gif}")"
case "${gif_meta}" in
  960x540x15/1x300|960x540x12/1x240|864x486x12/1x240) ;;
  *) echo "Unexpected GIF metadata: ${gif_meta}" >&2; exit 1 ;;
esac

gif_duration="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "${gif}")"
gif_size="$(stat -f%z "${gif}")"
test "${gif_duration}" = "20.000000" || { echo "Unexpected GIF duration: ${gif_duration}" >&2; exit 1; }
(( gif_size <= limit_bytes )) || { echo "GIF exceeds 10 MiB: ${gif_size}" >&2; exit 1; }

loop_extension="$(xxd -p "${gif}" | tr -d '\n' | grep -Eo '21ff0b4e45545343415045322e300301[0-9a-f]{4}00' | head -1)"
test "${loop_extension}" = "21ff0b4e45545343415045322e300301000000" || { echo "GIF does not declare an infinite loop" >&2; exit 1; }

echo "Verified master (${master_meta}, silent) and GIF (${gif_meta}, ${gif_size} bytes, infinite loop)."
