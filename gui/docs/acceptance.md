[简体中文](acceptance.zh_CN.md) · English

# Implementation and acceptance record

Latest recorded validation: 2026-10-30. Each dated section describes its tested implementation and package. USB application installation is verified on the five-partition device through the packaged FirmwareHelper. Historical results do not replace validation of a new release build.

## USB ROM application installation (2026-10-30)

Build: PASS for the signed debug application and its three helpers. Host tests: PASS for the GUI gate (20 Bun tests, 15 Rust tests, 9 Swift tests, Go tests/vet, Clippy, firmware resources, macOS compatibility and signatures) and repository/static host checks. The complete root gate stops at the unavailable ESP-IDF 5.5.3 environment; the firmware was not rebuilt.

Device tests: PASS using the new application bundle's FirmwareHelper and the same NDJSON scan/install protocol used by the GUI. The ESP32-C3 / 8 MiB device has the exact five-partition template layout, no expected bootloader log marker, and an erased Recovery image header. The packaged 0.2.0 application (1,162,848 bytes, SHA-256 `30be20312ed0cd6e72935e6a96df47f4029fbafc267f960cb223ac7ae8b325ad`) was written at `0x10000`. The helper reported ordered preflight, writing, verifying and restarting stages, followed by complete. Device-side checksum verification and reset succeeded. Separate reads before and after installation confirmed identical bootloader and partition-table SHA-256 digests. No identity contents were read.

The installer no longer uses a bootloader log string as a USB installation prerequisite. Both known layouts retain exact partition validation and factory erase bounds; the packaged manifest and merged community image retain their Recovery contract. This operation does not restore missing Recovery functionality.

Unverified: physical USB installation on the nine-partition layout, interrupted-write behavior, native Install-button interaction, UP-key/Recovery behavior, and button/audio acceptance. The earlier direct USB test separately observed the firmware starting and advertising Go Voice. Historical acceptance records follow.

## Dual device-layout validation (2026-10-23)

Build: PASS for the Apple Development-signed debug app and its three helpers, all compatible with macOS 26. Host tests: PASS for the GUI gate (20 Bun tests, 15 Rust tests, 6 Swift tests, Go tests/vet, Clippy and package checks) and repository/static host checks. The root complete gate stopped at the missing ESP-IDF 5.5.3 environment; the existing bundled firmware remains 0.2.0 and passed its digest/manifest checks.

Read-only device diagnostics confirmed an ESP32-C3 with 8 MiB flash and a valid nine-entry resource partition table, with no template recovery marker. Only the bootloader and partition-table ranges were read, followed by a normal-reset request; no flash erase/write occurred. The sanitized host fixture reproduces that measured table byte for byte. Regression tests retain the original five-partition layout and its required recovery marker, admit the exact nine-partition layout, reject modified/duplicate/missing entries and invalid MD5/flags, and bound sector erasure for both layouts to the existing factory slot ending at `0x310000`.

Device tests: PASS for enumeration and read-only diagnostics; application writing and device-side checksum verification NOT RUN because the USB device was no longer enumerated when preparing the write test. Unverified: actual installation on both hardware variants, post-write bootloader/table preservation, interrupted-write recovery, and the resource-layout device's vendor UP-key/Recovery behavior. Compatibility is selected by measured layout, not by an assumed board age or model name. Historical acceptance records follow.

## Omni WebSocket and debug identity validation (2026-10-20)

Build: PASS for the Apple Development-signed `go-voice-debug.app`. Host tests: PASS for `./tools/validate.sh --gui`, ASR race tests, and the root static checks. Device tests: NOT RUN. The root complete gate stopped before firmware compilation because `idf.py` / the activated ESP-IDF 5.5.3 environment was unavailable; the GUI gate separately verified the existing packaged firmware image.

The TLS WebSocket fixture verifies Bearer authentication, session configuration, exact PCM payloads, preview replacement, manual commit, final item correlation, errors, silent connection testing and cancellation. Package validation checks the main `.debug` identifier and all three helper `.debug` identifiers, their signatures, arm64 executables and firmware manifest. Debug metadata is generated in temporary staging; release configuration remains unchanged. Unverified: a live Alibaba Cloud endpoint, device-to-app dictation, first debug grants and retention of release/debug TCC grants across rebuilds. The earlier acceptance evidence below remains historical.

## Firmware packaging acceptance

The integrated FirmwareHelper change passes 11 packaging fixture tests with 59 assertions, nine Rust tests, the Bun-to-Rust manifest contract probe, Go tests including race checks, Swift tests, Clippy, shell syntax, helper Info.plist, complete app packaging, arm64 checks and nested/outer signature verification. Build: PASS. Host tests: PASS. Device tests: PASS for read-only USB enumeration only; preflight/write/reset NOT RUN.

The root firmware gate now retains both the verified merged image and raw application. GUI packaging validates a regular, nonempty ESP image (`0xE9`), a 3 MB maximum and a 4 KiB-rounded erase end before `cardid`. It writes the schema-v1 manifest atomically with a lowercase SHA-256 digest. `partitions` is an array containing the complete five-entry NVS, PHY, factory, `cardid`, and Recovery layout; type/subtype/offset/size are numeric ESP partition values. Temporary firmware staging is removed on build exit. The locked Rust release binary is signed as a separate nested app before the outer MyGo bundle.

The integrated host gate passes for the Rust, Go, Swift, packaging and signature layers. Cargo metadata reports license metadata for all 121 resolved third-party packages; the actual arm64 dependency tree includes `serialport` 4.10.1 under MPL-2.0, whose complete license and exact source link now ship with the app. A final release audit of copyright attributions remains required before distribution.

Device acceptance must still confirm app-only writes at `0x10000`, preflight rejection of wrong chip/flash/partition table/Recovery marker or image digest, no erase-all path, and preservation of bootloader, partition table, `cardid` and permanent Recovery. Test disconnect/failure recovery by holding UP at power-on for five seconds and reinstalling through permanent Recovery with the mini-program. Firmware success must not be inferred from a packaged manifest, USB enumeration or host tests. Unverified: actual USB preflight, flashing, reset and Recovery behavior; the final release copyright-attribution audit.

## Implemented contract

- Thin Go entry point, independent module, MyGo 0.7.3 native UI and standard debug packaging. The release app name is `go-voice`, its bundle id is `yz.sh.mygo-go-voice`; local builds are `go-voice-debug` / `yz.sh.mygo-go-voice.debug`, with `.debug` appended to every helper identifier. The BLE product identity remains Go Voice.
- Strict control validation rejects duplicate/unknown keys, trailing JSON, malformed UTF-8, invalid numeric types, unsupported format, missing mandatory fields and oversize controls. Audio envelope validation, 64-part assembly, duplicate suppression, deterministic bounded eviction, independent 166-byte IMA-ADPCM, 320-sample PCM and bounded ordering are pure Go.
- Streaming has a 75-frame early buffer, 80 ms gap wait, cumulative 100-frame silence limit, 500 ms announced-tail deadline, three-second stall, and 15,000-frame/five-minute cap. RMS, peak, 32 waveform bins, duration and received/missing/recovered/duplicate counts are calculated without retaining audio history.
- Coordinator tests exercise start, duplicate start, early frames, release before Speech readiness, finish, refinement, insertion acknowledgment, disconnect, cancellation, stale results, acceptance timeout, consecutive sessions that reuse PTT sequence 0/1, and button events in their independent sequence space. Every native Speech request uses a host generation ID distinct from the BLE session ID. Device cancellation clears the current request before sending native cancellation.
- CoreBluetooth helper connects only to an explicit selected UUID, subscribes Control/Audio, reads DeviceInfo, verifies at least 182-byte ATT payload, retries writes, reconnects and forgets devices. Delayed MTU completion is handled. BluetoothHelper and SpeechBridge ship as signed nested helper app bundles so macOS TCC can resolve their usage descriptions and stable identities; FirmwareHelper ships as a separately signed nested app without TCC-only entitlements. Parent death, stdin EOF, signals and bounded command handling stop each helper. Go bounds native line length, decoded payloads and string fields.
- Real SpeechAnalyzer/SpeechTranscriber accepts 16 kHz Int16 mono PCM, uses volatile results and per-session contextual strings, prepares/reserves zh-CN assets, emits session-scoped partial/final/error/cancel events and never opens a Mac audio input. The helper keeps the physical main thread alive with `RunLoop.main.run()` because Speech framework operations cannot rely on a dispatch main queue whose original main thread has exited. Swift Package targets compile against the installed SDK, not stub APIs. Both helpers read long-lived stdin with readiness notifications; real-process regression tests keep stdin open and require immediate command handling.
- TextInjector captures the foreground target at start, checks Accessibility/Secure Input, handles modifiers, uses System Events then CGEvent, and restores the full pasteboard snapshot after 0.5 seconds only if its ownership marker and change count still match. Missing/changed targets fall back to clipboard-only.
- Vocabulary persists required fields, deduplicates, supports manual CRUD and locking, rejects AI deletion/replacement of locked terms and supplies Top 100 terms. Refinement uses a fresh local FoundationModels session by default; cloud is explicitly selected. Modes include off/names/light/structured, a 30-second timeout, and empty/error/low-fidelity fallback to final ASR original. Remote HTTPS rejects credentials in URLs and redirects; only Keychain supplies bearer credentials.
- The UI owns neither helper processes nor Speech sessions. The application composes services and dispatches background results with `mygo.RunOnMain`. Current audio/transcript state is transient. The microphone usage description and audio-input entitlement serve only the hold-to-talk hotkey; the input opens only while the chord is held.

## Automated evidence

| Check | Result |
| --- | --- |
| Cargo fmt/test/Clippy | PASS: 9 Rust tests, pinned `espflash` 4.6.0 |
| Bun packaging and Rust manifest contract | PASS: 11 fixture tests / 59 assertions; no hardware write |
| gofmt check | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `swift test --package-path macos` | PASS: 4 tests; no real BLE or Speech permissions required |
| `./scripts/build.sh` | PASS: `go-voice` debug arm64 app, two Swift helpers, signed Rust FirmwareHelper and app-only firmware resources |
| Packaged firmware/manifest and helper signatures | PASS: SHA-256/size/layout, three nested signatures, arm64 and outer deep signature |
| `codesign --verify --deep --strict build/debug/darwin-arm64/go-voice.app` | PASS |
| Root firmware build and static gate | PASS: the official ESP-IDF 5.5.3 container build and host gate remain recorded in the root firmware acceptance report |
| Bundled application launch and UI | PASS: the ad-hoc app launched, all four pages rendered, the model reported `zh-CN model warmed`, and the firmware page showed one anonymous compatible USB device |
| USB firmware behavior | Enumeration PASS; preflight/write/verify/reset and interrupted-write Recovery NOT RUN |

Go tests cover strict JSON, golden silence and nonzero ADPCM vectors, predictor clamping, invalid codec fields, fragmented reorder/duplicates/64-piece assembly/eviction, missing-frame and tail boundaries, five-minute cap, waveform measurements, bounded helper parsing/canonical Base64, real helper commands over persistent open pipes, two consecutive PTT sessions reusing wire control sequence 0/1, independent button-event deduplication, vocabulary locking/persistence, explicit cloud selection/local fallback, timeout/fidelity, Keychain framework symbol/query binding without reading credentials, and the coordinator lifecycle. Swift tests cover PCM bounds/canonical Base64, generation identity, explicit device selection/MTU and conditional clipboard restoration.

The initial MyGo build exposed its cgo-disabled compilation path. The final implementation uses purego Security.framework and CoreFoundation bindings and passes the standard MyGo build. No alternative credential file backend was introduced.

## Remaining acceptance

Build: PASS. Host tests: PASS. Device tests: PASS for USB enumeration only; firmware preflight/write/reset NOT RUN.

Unverified: real CoreBluetooth discovery/reconnection under radio loss; a five-minute FoloToy AI Passport stream; MTU inference on the actual peripheral; Speech authorization and first-use asset download; recognition quality/latency; local FoundationModels output quality; real Keychain CRUD authorization; optional explicitly selected remote endpoint; Accessibility/Automation grants; Secure Input and clipboard ownership during actual paste; insertion into WeChat, VS Code, Chrome and terminals; sleep/wake and app exit while connecting. Unit tests and the successful bundled-app/model-warmup check prove the reducer, pipe and build boundaries, not the remaining operating-system/device interactions.

The recorded application was ad-hoc signed for local development. Release builds are now Developer ID signed, notarized and stapled, and update themselves with Ed25519-signed archives ([updates, signing, and notarization](updates.md)); no release build has been installed, launched, or updated yet. A release run with Gatekeeper first launch, the update from a published older version with TCC grants retained, and the post-update helper signatures remain user checks. TCC grants may need renewal after identity-changing debug rebuilds. V1 is unauthenticated and must not be represented as authenticated or end-to-end encrypted; the device page no longer shows a separate warning about it. No physical-device or permission checks were silently substituted with mocks in the app's main path.

The recorded acceptance pass rebuilt firmware with the official ESP-IDF 5.5.3 container, ran the complete Rust/Go/Swift/packaging/signature gate, inspected the packaged arm64 binaries and resources, launched the real bundle, exercised the fourth firmware page and confirmed anonymous enumeration of one compatible USB Serial/JTAG device. It deliberately did not select the device or trigger installation.
