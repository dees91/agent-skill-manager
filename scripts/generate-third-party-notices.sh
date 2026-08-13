#!/usr/bin/env bash

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

fail() {
  printf 'generate-third-party-notices: %s\n' "$*" >&2
  exit 1
}

emit_component() {
  local ecosystem="$1"
  local name="$2"
  local version="$3"
  local directory="$4"
  local files=()
  local file

  while IFS= read -r file; do
    files+=("$file")
  done < <(find "$directory" -maxdepth 1 -type f \( -iname 'LICENSE*' -o -iname 'COPYING*' -o -iname 'NOTICE*' -o -iname 'PATENTS*' \) -print | LC_ALL=C sort)
  ((${#files[@]} > 0)) || fail "no license or notice file found for $name $version in $directory"

  printf '\n================================================================================\n'
  printf '%s: %s %s\n' "$ecosystem" "$name" "$version"
  printf '================================================================================\n'
  for file in "${files[@]}"; do
    printf '\n--- %s ---\n\n' "$(basename "$file")"
    sed 's/[[:space:]]*$//' "$file"
    printf '\n'
  done
}

printf '%s\n' \
  'THIRD-PARTY SOFTWARE NOTICES' \
  '' \
  'Skill Manager includes third-party software listed below. The notices and' \
  'license texts are reproduced from the exact dependency versions selected by' \
  'the root Go module, desktop Go module, and production frontend lockfile.' \
  'Skill Manager itself is licensed separately under the repository LICENSE.'

emit_component "Go runtime and standard library" "Go" "toolchain license" "$SCRIPT_DIR/licenses/go"

while IFS=$'\t' read -r module version directory; do
  [[ -n "$module" ]] || continue
  [[ "$module" == "github.com/dees91/agent-skill-manager" ]] && continue
  emit_component "Go module" "$module" "$version" "$directory"
done < <(
  {
    go -C "$REPO_ROOT" list -deps -f '{{with .Module}}{{if not .Main}}{{printf "%s\t%s\t%s" .Path .Version .Dir}}{{end}}{{end}}' .
    go -C "$REPO_ROOT/desktop" list -deps -f '{{with .Module}}{{if not .Main}}{{printf "%s\t%s\t%s" .Path .Version .Dir}}{{end}}{{end}}' .
  } | sed '/^$/d' | LC_ALL=C sort -u
)

while IFS=$'\t' read -r package version directory; do
  [[ -n "$package" ]] || continue
  emit_component "npm package" "$package" "$version" "$directory"
done < <(
  while IFS= read -r directory; do
    [[ "$directory" == "$REPO_ROOT/desktop/frontend" ]] && continue
    node -e 'const p=require(process.argv[1]+"/package.json"); process.stdout.write(`${p.name}\t${p.version}\t${process.argv[1]}\n`)' "$directory"
  done < <(npm --prefix "$REPO_ROOT/desktop/frontend" ls --parseable --omit=dev --all | LC_ALL=C sort)
  )
