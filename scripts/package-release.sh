#!/usr/bin/env bash

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly RELEASE_VERSION="${RELEASE_VERSION:-}"
readonly RELEASE_DIR="$REPO_ROOT/dist/release"
readonly DESKTOP_ARCHIVE="skill-manager-desktop-${RELEASE_VERSION}-macos-arm64.zip"
readonly CLI_ARCHIVE="skill-manager-cli-${RELEASE_VERSION}-macos-arm64.tar.gz"
readonly CHECKSUM_FILE="SHA256SUMS.txt"
readonly APP_PATH="$REPO_ROOT/desktop/build/bin/Skill Manager.app"
readonly CLI_PACKAGE_DIR="skill-manager-cli-${RELEASE_VERSION}-macos-arm64"
readonly NOTICE_FILE="THIRD_PARTY_NOTICES.txt"

fail() {
  printf 'release-package: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

assert_clean_checkout() {
  git -C "$REPO_ROOT" diff --quiet || fail "tracked files have unstaged changes"
  git -C "$REPO_ROOT" diff --cached --quiet || fail "tracked files have staged changes"
  [[ -z "$(git -C "$REPO_ROOT" ls-files --others --exclude-standard)" ]] || fail "checkout contains untracked files"
}

assert_version_metadata() {
  node - "$REPO_ROOT" "$RELEASE_VERSION" <<'NODE'
const fs = require('fs');
const path = require('path');

const [root, expected] = process.argv.slice(2);
const readJSON = relative => JSON.parse(fs.readFileSync(path.join(root, relative), 'utf8'));
const wails = readJSON('desktop/wails.json');
const packageJSON = readJSON('desktop/frontend/package.json');
const lock = readJSON('desktop/frontend/package-lock.json');
const versions = [
  ['desktop/wails.json', wails.info && wails.info.productVersion],
  ['desktop/frontend/package.json', packageJSON.version],
  ['desktop/frontend/package-lock.json', lock.version],
  ['desktop/frontend/package-lock.json packages root', lock.packages && lock.packages[''] && lock.packages[''].version],
];

for (const [source, actual] of versions) {
  if (actual !== expected) {
    console.error(`release-package: ${source} version is ${JSON.stringify(actual)}, expected ${expected}`);
    process.exit(1);
  }
}
NODE

  grep -Fq "The current source version is \`$RELEASE_VERSION\`" "$REPO_ROOT/README.md" ||
    fail "README source version does not match $RELEASE_VERSION"
  [[ -f "$REPO_ROOT/docs/releases/v${RELEASE_VERSION}.md" ]] ||
    fail "missing docs/releases/v${RELEASE_VERSION}.md"
}

assert_arm64_binary() {
  local binary="$1"
  local description
  description="$(/usr/bin/file "$binary")"
  [[ "$description" == *"Mach-O 64-bit executable arm64"* ]] ||
    fail "expected thin arm64 Mach-O binary: $description"
  [[ "$description" != *"universal binary"* ]] ||
    fail "unexpected universal binary: $description"
}

assert_cli() {
  local binary="$1"
  local signature
  [[ -x "$binary" ]] || fail "CLI is not executable: $binary"
  assert_arm64_binary "$binary"
  /usr/bin/codesign --verify --strict "$binary"
  signature="$(/usr/bin/codesign -dv --verbose=4 "$binary" 2>&1)"
  [[ "$signature" == *"Signature=adhoc"* ]] || fail "CLI is not ad-hoc signed"
  [[ "$($binary --version)" == "skill-manager $RELEASE_VERSION" ]] ||
    fail "CLI reports an unexpected version"
  "$binary" help >/dev/null
}

assert_app() {
  local app="$1"
  local plist="$app/Contents/Info.plist"
  local executable
  local signature

  [[ -d "$app" ]] || fail "app bundle not found: $app"
  [[ -f "$plist" ]] || fail "Info.plist not found in app bundle"
  /usr/bin/codesign --verify --deep --strict "$app"
  signature="$(/usr/bin/codesign -dv --verbose=4 "$app" 2>&1)"
  [[ "$signature" == *"Signature=adhoc"* ]] || fail "desktop app is not ad-hoc signed"

  [[ "$(/usr/bin/plutil -extract CFBundleIdentifier raw -o - "$plist")" == "io.github.dees91.skillmanager" ]] ||
    fail "unexpected desktop bundle identifier"
  [[ "$(/usr/bin/plutil -extract CFBundleShortVersionString raw -o - "$plist")" == "$RELEASE_VERSION" ]] ||
    fail "unexpected desktop short version"
  [[ "$(/usr/bin/plutil -extract CFBundleVersion raw -o - "$plist")" == "$RELEASE_VERSION" ]] ||
    fail "unexpected desktop bundle version"
  [[ "$(/usr/bin/plutil -extract LSMinimumSystemVersion raw -o - "$plist")" == "13.0" ]] ||
    fail "unexpected desktop minimum macOS version"

  executable="$(/usr/bin/plutil -extract CFBundleExecutable raw -o - "$plist")"
  assert_arm64_binary "$app/Contents/MacOS/$executable"
}

launch_app_with_isolated_home() {
  local app="$1"
  local run_dir="$2"
  local plist="$app/Contents/Info.plist"
  local executable
  local pid

  executable="$(/usr/bin/plutil -extract CFBundleExecutable raw -o - "$plist")"
  mkdir -p "$run_dir/home"
  HOME="$run_dir/home" "$app/Contents/MacOS/$executable" >"$run_dir/app.log" 2>&1 &
  pid=$!
  sleep 3
  if ! kill -0 "$pid" 2>/dev/null; then
    wait "$pid" || true
    fail "desktop app exited during isolated-home launch; see $run_dir/app.log"
  fi
  kill "$pid"
  wait "$pid" 2>/dev/null || true
}

for command in git go node npm make tar ditto shasum codesign plutil file mktemp; do
  require_command "$command"
done

[[ "$RELEASE_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]] ||
  fail "set RELEASE_VERSION to a semantic version, for example 0.4.1"
[[ "$(/usr/bin/uname -s)" == "Darwin" ]] || fail "release packaging requires macOS"
[[ "$(/usr/bin/uname -m)" == "arm64" ]] || fail "release packaging requires Apple Silicon"

cd "$REPO_ROOT"
assert_clean_checkout
assert_version_metadata

if git ls-files --error-unmatch scripts/public-check.sh >/dev/null 2>&1; then
  fail "scripts/public-check.sh must not be tracked"
fi
[[ -z "$(git log --all --format='%H' -- scripts/public-check.sh)" ]] ||
  fail "scripts/public-check.sh is present in Git history"

printf '==> Verifying Go and frontend sources\n'
make notices-check
go test ./...
go vet ./...
(
  cd desktop/frontend
  npm ci
  npm run typecheck
  npm test
  npm run build
  npm audit --audit-level=high
)
(
  cd desktop
  go test ./...
  go vet ./...
)

printf '==> Building desktop application\n'
make gui-build
mkdir -p "$APP_PATH/Contents/Resources"
cp LICENSE "$NOTICE_FILE" "$APP_PATH/Contents/Resources/"
/usr/bin/codesign --force --deep --sign - "$APP_PATH"
assert_app "$APP_PATH"

mkdir -p "$REPO_ROOT/dist"
readonly WORK_DIR="$(mktemp -d "$REPO_ROOT/dist/.release-work.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

case "$RELEASE_DIR" in
  "$REPO_ROOT/dist/release") ;;
  *) fail "refusing to replace unexpected release directory: $RELEASE_DIR" ;;
esac
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR" "$WORK_DIR/$CLI_PACKAGE_DIR"

printf '==> Building CLI\n'
go build -trimpath \
  -ldflags="-s -w -X github.com/dees91/agent-skill-manager/internal/cli.Version=$RELEASE_VERSION" \
  -o "$WORK_DIR/$CLI_PACKAGE_DIR/skill-manager" .
/usr/bin/codesign --force --sign - "$WORK_DIR/$CLI_PACKAGE_DIR/skill-manager"
cp README.md LICENSE "$NOTICE_FILE" "$WORK_DIR/$CLI_PACKAGE_DIR/"
assert_cli "$WORK_DIR/$CLI_PACKAGE_DIR/skill-manager"

printf '==> Creating release archives\n'
/usr/bin/ditto --noqtn -c -k --sequesterRsrc --keepParent \
  "$APP_PATH" "$RELEASE_DIR/$DESKTOP_ARCHIVE"
COPYFILE_DISABLE=1 /usr/bin/tar -czf "$RELEASE_DIR/$CLI_ARCHIVE" \
  -C "$WORK_DIR" "$CLI_PACKAGE_DIR"

mkdir -p "$WORK_DIR/unpacked-desktop" "$WORK_DIR/unpacked-cli"
/usr/bin/ditto -x -k "$RELEASE_DIR/$DESKTOP_ARCHIVE" "$WORK_DIR/unpacked-desktop"
/usr/bin/tar -xzf "$RELEASE_DIR/$CLI_ARCHIVE" -C "$WORK_DIR/unpacked-cli"

assert_app "$WORK_DIR/unpacked-desktop/Skill Manager.app"
assert_cli "$WORK_DIR/unpacked-cli/$CLI_PACKAGE_DIR/skill-manager"
[[ -f "$WORK_DIR/unpacked-cli/$CLI_PACKAGE_DIR/README.md" ]] || fail "CLI archive is missing README.md"
[[ -f "$WORK_DIR/unpacked-cli/$CLI_PACKAGE_DIR/LICENSE" ]] || fail "CLI archive is missing LICENSE"
[[ -f "$WORK_DIR/unpacked-cli/$CLI_PACKAGE_DIR/$NOTICE_FILE" ]] || fail "CLI archive is missing $NOTICE_FILE"
[[ -f "$WORK_DIR/unpacked-desktop/Skill Manager.app/Contents/Resources/LICENSE" ]] || fail "desktop app is missing LICENSE"
[[ -f "$WORK_DIR/unpacked-desktop/Skill Manager.app/Contents/Resources/$NOTICE_FILE" ]] || fail "desktop app is missing $NOTICE_FILE"
launch_app_with_isolated_home "$WORK_DIR/unpacked-desktop/Skill Manager.app" "$WORK_DIR/launch"

(
  cd "$RELEASE_DIR"
  /usr/bin/shasum -a 256 "$DESKTOP_ARCHIVE" "$CLI_ARCHIVE" >"$CHECKSUM_FILE"
  /usr/bin/shasum -a 256 -c "$CHECKSUM_FILE"
)

[[ "$(find "$RELEASE_DIR" -mindepth 1 -maxdepth 1 -type f | wc -l | tr -d ' ')" == "3" ]] ||
  fail "release directory must contain exactly three files"
assert_clean_checkout

printf '==> Release artifacts ready in %s\n' "$RELEASE_DIR"
printf '    %s\n    %s\n    %s\n' "$DESKTOP_ARCHIVE" "$CLI_ARCHIVE" "$CHECKSUM_FILE"
