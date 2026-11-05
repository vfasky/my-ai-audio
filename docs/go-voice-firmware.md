[简体中文](go-voice-firmware.zh_CN.md) · English

# Go Voice firmware

## Provenance and scope

The firmware derives from Vokie commit `ce26ea043251102155e54bc9503eb8133d5cafa8`. The imported source archive SHA-256 is `f78a1349ec3a02c0fe96a9233946ee0d78e0bb289d8e5cbe02b12a7c5ef88575`. FoloToy MIT attribution, the dependency lock, ESP-IDF 5.5.3 structure, BSP, bootloader hook and partition layout are retained. Internal `vokie_` identifiers preserve the imported implementation structure; public identity is Go Voice.

The badge captures audio and sends it to the [macOS application](../gui/README.md). ASR, vocabulary, refinement and text insertion run on the Mac. Wi-Fi audio, authenticated V2 transport and OTA are outside the current firmware implementation.

## Identity and protocol

| Endpoint | UUID |
| --- | --- |
| Service | `676f0001-766f-6963-652d-617564696f00` |
| Control | `676f0002-766f-6963-652d-617564696f00` |
| Audio | `676f0003-766f-6963-652d-617564696f00` |
| DeviceInfo | `676f0004-766f-6963-652d-617564696f00` |

Advertising, title, readiness and waiting messages use `Go Voice`. `hello.device` remains `ai-passport`. NimBLE UUID arrays use reverse byte order, checked against canonical UUIDs and their consumers by host tests. The [BLE V1 protocol](ai-passport-ble-protocol.md) defines the wire contract.

UP toggles capture; DOWN sends Enter; OK deletes or cancels, with long-press clear/cancel. Audio uses independent 20 ms / 320-sample IMA-ADPCM frames, 166 encoded bytes, and ATT MTU 185 or greater. The Mac enforces the 15,000-frame/five-minute session bound. The [standby contract](development/engineering/standby-power-contract.md) defines audio/display pause, brightness and USB power behavior.

The standalone Vokie plugin is no longer included in this repository. The desktop app builds its own native helpers; upstream attribution and MIT notices are retained. Unmodified Vokie clients using the old service UUID cannot connect.

## Display and resources

The [logo](../assets/brand/go-voice-logo.png) is a white Gopher and lower-right microphone on transparency. The desktop icon places that artwork on a black rounded tile; firmware composites transparency onto the existing `#0B1018` screen background. Original white areas remain white, and black areas become transparent, including facial cutouts.

`tools/generate-go-voice-logo.mjs` uses the retained [source artwork](../assets/brand/yz-logo-source.png) to generate desktop assets and the [firmware header](../assets/brand/go_voice_logo_asset.h). See [asset generation](../assets/README.md). `main/ui_status.c` embeds exactly 8192 constant bytes of 64×64 RGB565 pixels. The title uses LVGL Montserrat 20. No runtime image decoder, resize buffer or extra font allocation is introduced. Rebuild the firmware after regenerating its image header.

## Protected storage and licensing

The application starts at `0x10000` and is limited to `0x300000` bytes. Preserve `cardid` at `0x356000`, permanent Recovery at `0x700000`, and the five-second UP-key bootloader hook. Follow the [Recovery compatibility contract](development/engineering/ble-recovery-compatibility.md); only a successful firmware gate verifies the final merged artifact and linked hook.

The reference startup WAV and PCM remain unchanged. Upstream reports creator permission for its firmware without a standalone sound license. Obtain independent authorization before distributing Go Voice source or binaries containing that sound. Preserve the FoloToy MIT notice, Barlow OFL and [third-party notices](THIRD_PARTY_NOTICES.md). Go Voice artwork grants no rights to the Vokie mark.

## Validation

- `python3 tests/test_go_voice_contract.py`: UUID byte order and consumers, logo hash, storage and LVGL integration.
- `./tools/validate.sh --static`: repository checks, workflow lint and host tests, including bilingual documentation and local links.
- `./tools/validate.sh --firmware`: ESP-IDF 5.5.3 build, merged-image verification and protected-partition checks.
- `./tools/validate.sh`: complete firmware and desktop gate; requires both toolchains.

### Recorded validation for the current white-on-transparent artwork

These are historical results for the tested artifact, not a substitute for validating a new release build.

- Build: PASS. The complete gate passed with ESP-IDF v5.5.3 and macOS host checks. The merged image passed mini-program and protected-partition validation; the application was 1,161,296 bytes, below 3 MB.
- Merged artifact: `build/FoloToy-AI-Passport-full.bin`, 1,226,832 bytes, SHA-256 `a5da135353422d6fcf6b26d7c8cd51488475f27582c71018d934efa40a52d227`.
- Artwork: PASS. PNG SHA-256 `6b946925c2475161c1fbb2f5c181bfc3e617a2a6580078066d1b34404d25f544`; all RGB channels are white. The 8192-byte raster occurred once at merged offset `0x26088`.
- Host tests: PASS. Repository, C/Python, Go/Swift, desktop build and signing checks passed.
- Device tests: PASS for flashing, image readback and startup only. The ESP32-C3/8 MB Flash partition table matched before segmented writes; bootloader, table and application passed device hash checks. Erase/write ranges excluded NVS, PHY, `cardid` and Recovery. All image bytes read back from `0x26088` matched. A 15-second observation after reset showed one boot, successful display/LVGL/audio initialization and Go Voice advertising, without errors or panic lines.
- Unverified: physical display appearance, all buttons, end-to-end BLE voice input, sustained capture and reconnection, power measurements, and physical Recovery entry. Device identifiers and raw logs are not stored in the repository.
