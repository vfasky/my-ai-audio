#!/bin/bash
# Release the go-voice version of mygo.json from this Mac: test, build with
# the local Developer ID (.local/signing.env), notarize and staple, sign the
# update with the local update key, verify the result, then upload a draft
# GitHub release tagged go-voice-v<version>. See docs/updates.md.
set -euo pipefail
cd "$(dirname "$0")/.."

usage() {
  cat >&2 <<'EOF'
Usage: ./scripts/release.sh [--publish] [--skip-tests] [--no-upload] [--yes]
  --publish     publish the release after uploading (default: leave a draft)
  --skip-tests  skip ./scripts/validate.sh
  --no-upload   build and verify the release files only; no tag, no upload
  --yes         do not ask before pushing the tag and uploading
EOF
  exit 2
}
publish=false skip_tests=false upload=true assume_yes=false
for argument in "$@"; do
  case "$argument" in
    --publish) publish=true ;;
    --skip-tests) skip_tests=true ;;
    --no-upload) upload=false ;;
    --yes) assume_yes=true ;;
    *) usage ;;
  esac
done
if [[ "$publish" == true && "$upload" == false ]]; then
  echo '--publish needs the upload.' >&2
  exit 2
fi
fail() { echo "release: $*" >&2; exit 1; }
step() { printf '\n==> %s\n' "$*"; }

step 'Checking the release'
[[ "$(uname -m)" == arm64 ]] || fail 'MyGo requires Apple Silicon.'
version="$(plutil -extract version raw -o - mygo.json)"
repo="$(plutil -extract updates.github raw -o - mygo.json 2> /dev/null)" ||
  fail 'mygo.json has no updates.github; see docs/updates.md.'
tag="$(plutil -extract updates.tagPrefix raw -o - mygo.json)$version"
changelog="$(plutil -extract updates.changelog raw -o - mygo.json)"
notary_profile="$(plutil -extract macos.notarize.keychainProfile raw -o - mygo.json)"
helper_version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' resources/FirmwareHelper-Info.plist)"
[[ "$helper_version" == "$version" ]] ||
  fail "FirmwareHelper-Info.plist is $helper_version, mygo.json version is $version; change both."
for notes in docs/CHANGELOG.md docs/CHANGELOG.zh_CN.md; do
  grep -Eq "^## \[?${version//./\\.}([] ]|$)" "$notes" || fail "$notes has no \"## $version\" section."
done
prerelease=false
if [[ "$version" == *-* ]]; then prerelease=true; fi
# The release is built from the commit that the tag names.
[[ -z "$(git status --porcelain -- ..)" ]] || fail 'Commit or stash the changes in the working tree first.'
if [[ "$upload" == true ]]; then
  [[ "$(git branch --show-current)" == main ]] || fail 'Release from main.'
  command -v gh > /dev/null || fail 'Install the GitHub CLI (gh) and run gh auth login.'
  gh auth status > /dev/null 2>&1 || fail 'Run gh auth login first.'
  if state="$(gh release view "$tag" --repo "$repo" --json isDraft --jq .isDraft 2> /dev/null)" && [[ "$state" != true ]]; then
    fail "Release $tag is already published; raise the version."
  fi
fi
if git rev-parse -q --verify "refs/tags/$tag" > /dev/null; then
  [[ "$(git rev-parse "$tag^{commit}")" == "$(git rev-parse HEAD)" ]] || fail "Tag $tag exists on another commit."
fi

# The Developer ID: the environment, else .local/signing.env (ignored by Git).
if [[ -f .local/signing.env ]]; then
  # shellcheck source=/dev/null
  source .local/signing.env
fi
identity="${MYGO_SIGNING_IDENTITY:-}"
team="${MYGO_TEAM_ID:-}"
[[ "$identity" == "Developer ID Application: "* ]] ||
  fail 'Set MYGO_SIGNING_IDENTITY to a Developer ID Application identity (.local/signing.env); see docs/updates.md.'
[[ -n "$team" ]] || fail 'Set MYGO_TEAM_ID (.local/signing.env).'
security find-identity -v -p codesigning | grep -Fq "\"$identity\"" ||
  fail "Missing signing identity in the keychain: $identity"
xcrun notarytool history --keychain-profile "$notary_profile" > /dev/null 2>&1 ||
  fail "No notary credentials in keychain profile $notary_profile; run xcrun notarytool store-credentials $notary_profile."

# The update key: the environment, the keychain backup, or the go-voice
# directory of mygo keygen. Never the default directory, whose key belongs
# to another app.
if [[ -z "${MYGO_UPDATER_PRIVATE_KEY:-}" ]]; then
  key_file="$HOME/Library/Application Support/mygo/update-keys/go-voice/mygo-update.key"
  if MYGO_UPDATER_PRIVATE_KEY="$(security find-generic-password -s 'go-voice update signing key' -a mygo-update.key -w 2> /dev/null)"; then
    echo 'Update key: login keychain'
  elif [[ -f "$key_file" ]]; then
    MYGO_UPDATER_PRIVATE_KEY="$(< "$key_file")"
    echo "Update key: $key_file"
  else
    fail 'No update signing key: set MYGO_UPDATER_PRIVATE_KEY; see docs/updates.md.'
  fi
fi
export MYGO_UPDATER_PRIVATE_KEY
echo "go-voice $version ($tag), repository $repo, publish=$publish"
echo "Signing as $identity, notarizing with keychain profile $notary_profile"

if [[ "$skip_tests" == false ]]; then
  step 'Validating'
  # The debug build of validate.sh never reads the update key.
  ./scripts/validate.sh
fi

step 'Building, signing and notarizing'
MYGO_SIGN_IDENTITY="$identity" ./scripts/build.sh --release
out=build/release/darwin-arm64
app="$out/go-voice.app"
dmg="$out/go-voice-$version-arm64.dmg"
archive="$out/go-voice-$version-darwin-arm64.tar.gz"
manifest="$out/update-darwin-arm64.json"
for file in "$app" "$dmg" "$archive" "$manifest"; do
  [[ -e "$file" ]] || fail "The build did not write $file."
done

step 'Verifying compatibility, signatures and notarization'
bash scripts/verify-macos-compatibility.sh "$app"
codesign --verify --deep --strict --verbose=2 "$app"
signature() { codesign --display --verbose=4 "$1" 2>&1; }
check_signed() {
  local details
  details="$(signature "$1")"
  grep -Fxq "Authority=$identity" <<< "$details" || fail "$1 is not signed by $identity."
  grep -Fxq "TeamIdentifier=$team" <<< "$details" || fail "$1 is not signed by team $team."
  grep -Eq '^CodeDirectory .*flags=.*runtime' <<< "$details" || fail "$1 lacks the hardened runtime."
  grep -q '^Timestamp=' <<< "$details" || fail "$1 has no secure timestamp."
}
check_signed "$app"
for helper in BluetoothHelper SpeechBridge FirmwareHelper; do
  check_signed "$app/Contents/Resources/native/$helper.app"
done
xcrun stapler validate "$app"
xcrun stapler validate "$dmg"
spctl --assess --type execute --verbose=2 "$app"
spctl --assess --type open --context context:primary-signature --verbose=2 "$dmg"
# The update installs what the archive holds: check that copy too.
extracted="$(mktemp -d "${TMPDIR:-/tmp}/go-voice-release.XXXXXX")"
trap 'rm -rf -- "$extracted"' EXIT
tar -xzf "$archive" -C "$extracted"
bash scripts/verify-macos-compatibility.sh "$extracted/go-voice.app"
codesign --verify --deep --strict "$extracted/go-voice.app"
xcrun stapler validate "$extracted/go-voice.app"
[[ "$(plutil -extract version raw -o - "$manifest")" == "$version" ]] || fail "$manifest is not version $version."

dist="build/updates/$version"
# Files of an earlier attempt at this version are written again.
rm -rf "$dist"
mkdir -p "$dist"
download="go-voice-$version-macOS-AppleSilicon.dmg"
cp "$dmg" "$dist/$download"
cp "$archive" "$manifest" "$dist/"
deltas=()
for delta in "$out"/*.delta; do
  [[ -e "$delta" ]] || continue
  cp "$delta" "$dist/"
  deltas+=("$dist/${delta##*/}")
done
(cd "$dist" && shasum -a 256 "$download" "${archive##*/}" > SHA256SUMS.txt)
plutil -extract notes raw -o "$dist/notes.md" "$dist/${manifest##*/}"
ls -l "$dist"
if [[ "$upload" == false ]]; then
  printf '\nRelease files are in %s; nothing was uploaded.\n' "$dist"
  exit 0
fi

if [[ "$assume_yes" == false ]]; then
  printf '\nPush tag %s and upload these files to a draft release of %s? [y/N] ' "$tag" "$repo"
  read -r answer
  [[ "$answer" == y || "$answer" == Y ]] || { echo "Stopped; the files stay in $dist."; exit 1; }
fi

step "Pushing $tag"
if ! git rev-parse -q --verify "refs/tags/$tag" > /dev/null; then
  git tag -a "$tag" -m "go-voice $version"
fi
git push origin "refs/tags/$tag"

step 'Uploading the release'
if ! gh release view "$tag" --repo "$repo" > /dev/null 2>&1; then
  # The repository's latest release stays the firmware's: apps find the
  # newest go-voice-v release through the GitHub API.
  arguments=(--repo "$repo" --draft --verify-tag --latest=false --title "go-voice $version" --notes-file "$dist/notes.md")
  if [[ "$prerelease" == true ]]; then arguments+=(--prerelease); fi
  gh release create "$tag" "${arguments[@]}"
fi
# The manifest goes last, so it never names an archive that is missing.
gh release upload "$tag" --repo "$repo" --clobber "$dist/$download" "$dist/${archive##*/}" ${deltas[@]+"${deltas[@]}"} "$dist/SHA256SUMS.txt"
gh release upload "$tag" --repo "$repo" --clobber "$dist/${manifest##*/}"
if [[ "$publish" == true ]]; then
  gh release edit "$tag" --repo "$repo" --draft=false --latest=false
  printf '\nPublished %s: https://github.com/%s/releases/tag/%s\n' "$tag" "$repo" "$tag"
else
  printf '\nDraft %s uploaded. Review it on GitHub, then publish:\n  gh release edit %s --repo %s --draft=false --latest=false\n' "$tag" "$tag" "$repo"
fi
