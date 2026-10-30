<p align="right">
  <a href="updates.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# Updates, signing, and notarization

`go-voice` updates itself through MyGo's signed updater and ships as a Developer ID signed, notarized app. These are separate mechanisms, and both are required:

| Mechanism | Proves | Key |
| --- | --- | --- |
| Developer ID signature and hardened runtime | Gatekeeper and TCC identity: the code comes from team `92ZWDWJJNB` and was not modified | Developer ID Application certificate in the login keychain |
| Apple notarization, stapled | Apple scanned this exact build; Gatekeeper opens it offline | `notarytool` credentials in the keychain profile `NOTARY` |
| Ed25519 update signature | An installed `go-voice` accepts only archives signed by the release key | `mygo-update.key`, never in the repository |

A stable Developer ID also keeps the designated requirement unchanged across updates, so Bluetooth, Speech Recognition, Accessibility, and Automation grants survive an update. Ad hoc and Apple Development builds do not provide this.

## How updates work

Release builds embed the update feed and the public key from `mygo.json`:

```json
"updates": {
  "publicKey": "kf9mFvBQMctgyCFHkeCemCVejf31Xx2+gNkoFuWFqr4=",
  "github": "vfasky/my-ai-audio",
  "tagPrefix": "go-voice-v",
  "changelog": "docs/CHANGELOG.zh_CN.md"
}
```

- The repository's latest release belongs to the firmware, so desktop releases use their own tag prefix. The app finds the newest published `go-voice-v*` release that is neither a draft nor a prerelease through the GitHub API (60 unauthenticated requests per hour per address), reads `update-darwin-arm64.json`, and downloads the archive or a delta named there.
- The manifest holds the version, release notes, date, and the URL, size, and Ed25519 signature of the archive and each delta. The app verifies the signature with its built-in public key before replacing `go-voice.app`; a delta that fails falls back to the full archive.
- MyGo's native update window (Simplified Chinese) checks in the background about 10 seconds after each launch and then once a day, shows the release notes, and offers to install the update, skip this version, or remind the user later. Menu bar › Check for Updates… opens it immediately. Installation stages the verified update. Relaunch quits through the normal `OnBeforeQuit` path, so helpers and any firmware installer stop before the bundle is replaced after exit and the new version starts through LaunchServices.
- Only release builds update. Debug builds from `build.sh` and `dev.sh` never read the update key, never write update files, and say so when Check for Updates… is used. A release build must be able to write where it is installed: `/Applications` for an administrator or `~/Applications`. Otherwise the menu explains that it cannot update itself there.
- The app is not sandboxed, so the bundle an update writes carries no sandbox quarantine and needs no repair helper.

## One-time setup

1. **Developer ID.** `security find-identity -v -p codesigning` must list `Developer ID Application: … (92ZWDWJJNB)`. `gui/.local/signing.env`, which Git ignores, sets `MYGO_SIGNING_IDENTITY` and `MYGO_TEAM_ID`; existing environment variables take precedence.
2. **Notary credentials.** Store them once under the profile named by `macos.notarize.keychainProfile` (`NOTARY`):

   ```sh
   xcrun notarytool store-credentials NOTARY
   ```

3. **Update key.** The key pair was created with the pinned MyGo in a directory of its own, so that it does not overwrite another app's default key:

   ```sh
   cd gui
   go run github.com/egoist/mygo/cmd/mygo keygen -o "$HOME/Library/Application Support/mygo/update-keys/go-voice"
   ```

   Its `mygo-update.pub` is `updates.publicKey`. Back up `mygo-update.key` in a password manager, or in the login keychain where the release script also looks:

   ```sh
   security add-generic-password -s 'go-voice update signing key' -a mygo-update.key -w
   ```

   (`-w` without a value prompts for the key, keeping it out of the process list.) Never commit, print, or log the private key. Losing it strands every installed copy; anyone holding it can ship code to users. Never run `keygen -force` for a released app; a new key requires every user to reinstall manually.

## Release a version

1. Change the version in `mygo.json` and in `CFBundleShortVersionString` of `resources/FirmwareHelper-Info.plist`, and increment its `CFBundleVersion`.
2. Add a `## <version>` section to both [`CHANGELOG.md`](CHANGELOG.md) and [`CHANGELOG.zh_CN.md`](CHANGELOG.zh_CN.md). The Chinese section is signed into the manifest and shown in the update window.
3. Commit, then from `gui/` on `main` with a clean working tree:

   ```sh
   ./scripts/release.sh            # asks before pushing the tag and uploading
   ./scripts/release.sh --no-upload  # build and verify only
   ```

The build pins `MACOSX_DEPLOYMENT_TARGET` and the C/C++ compiler/linker flags to `macos.minimumSystemVersion` in `mygo.json`. Setting Info.plist alone is insufficient when CGO uses a newer SDK; explicit flags also invalidate cached Go C/C++ objects. Every debug and release build runs `bash scripts/verify-macos-compatibility.sh <app>` to check arm64 support, the app/helper plist declarations, and the actual Mach-O minimum system versions. The SDK version may be newer than the deployment target. The release script also checks the app extracted from the update archive. These checks do not replace installation and dictation testing on the oldest supported macOS.

The script:

1. Checks the version, both changelog sections, the branch, the working tree, the tag, and whether the release is already published. It also checks the signing identity, the notary profile, and the update key: `MYGO_UPDATER_PRIVATE_KEY`, else the keychain item `go-voice update signing key`, else `~/Library/Application Support/mygo/update-keys/go-voice/mygo-update.key`.
2. Runs `./scripts/validate.sh` (`--skip-tests` skips it).
3. Runs `./scripts/build.sh --release`. It builds the helpers in release mode, then runs `mygo build -platform darwin/arm64 -o build/release -sign <Developer ID>`. MyGo signs the nested `BluetoothHelper`, `SpeechBridge` (with `entitlements.plist`), and `FirmwareHelper` bundles inside out, then the app, with the hardened runtime and a secure timestamp. It makes `go-voice-<version>-arm64.dmg`, signs it, submits it to `notarytool` and waits, then staples the ticket to the disk image and to the app. Finally it archives the stapled app, signs the archive, writes deltas from up to three published versions, and writes `update-darwin-arm64.json`.
4. Verifies macOS compatibility, `codesign --verify --deep --strict`, the Developer ID authority, team, hardened runtime, and timestamp of the app and every helper, `stapler validate` and `spctl` for the app and disk image, and the compatibility, signature and staple of the app extracted from the update archive.
5. Copies `go-voice-<version>-macOS-AppleSilicon.dmg`, the archive, deltas, `SHA256SUMS.txt`, `notes.md`, and the manifest to `build/updates/<version>/`.
6. After confirmation (`--yes` skips it), pushes the annotated tag `go-voice-v<version>`, creates a draft release that is not marked latest, uploads everything else, and uploads the manifest last.

Drafts never reach installed apps. Review the notes and files, then publish:

```sh
gh release edit go-voice-v<version> --repo vfasky/my-ai-audio --draft=false --latest=false
```

`--publish` does this immediately after uploading. A version containing `-` is published as a prerelease and is not offered as an update. The firmware workflow ignores `go-voice-v*` tags.

Re-running after a failure regenerates the files, overwrites the draft's files, and reuses a tag that names the current commit. A published release is never changed; raise the version instead. Never move or delete a pushed tag.

## Failures

| Symptom | Action |
| --- | --- |
| `Missing signing identity` | Install the Developer ID certificate and its private key in the login keychain. |
| `No notary credentials` | `xcrun notarytool store-credentials NOTARY`. |
| `notarization failed (Invalid …)` | `xcrun notarytool log <submission id> --keychain-profile NOTARY` lists the rejected files. |
| `No update signing key` | Restore the backed-up key; do not generate a new one. |
| `has no "## <version>" section` | Add the section to both changelog files and commit. |
| `Tag … exists on another commit` / `already published` | Raise the version. |

## Acceptance

Unit tests and a successful release build do not prove that an installed copy updates. Before the first public release, and when signing or the update path changes, check on a clean Mac account:

- Download the disk image, drag the app to `/Applications`, and confirm that it opens without a Gatekeeper warning.
- Grant permissions, publish a newer test version, run Check for Updates…, install it, relaunch it, and confirm that the version changed, the permissions remained granted, and Bluetooth dictation and firmware installation still work.
- `codesign --verify --deep --strict` and `spctl --assess --type execute` on the updated app.
