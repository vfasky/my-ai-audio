[简体中文](passport-audio-input-architecture.zh_CN.md) · English

# Go Voice system architecture

AI Passport is the sole audio capture device. It streams microphone audio over BLE to the macOS application, which performs recognition, optional refinement, and text insertion. The Mac opens its own microphone only while the optional hold-to-talk hotkey is held, feeding the same pipeline; audio and transcripts are not saved as history.

```text
AI Passport / ESP32-C3
  ES8311 → I2S PCM → independent IMA-ADPCM frames → BLE
Mac / go-voice
  Swift CoreBluetooth helper → Go validation, reassembly and decoding
    → Swift SpeechAnalyzer / SpeechTranscriber + current vocabulary
    → Optional local FoundationModels or explicitly selected remote refinement
    → TextInjector → foreground application's input field
```

The firmware derives from Vokie commit `ce26ea043251102155e54bc9503eb8133d5cafa8`; the desktop insertion design references [Yap](https://github.com/FrigadeHQ/yap). Source attribution and redistribution requirements remain in the [firmware notices](THIRD_PARTY_NOTICES.md) and [desktop notices](../gui/docs/THIRD_PARTY_NOTICES.md).

## Audio and protocol

The badge samples 16 kHz signed 16-bit mono PCM. Each 20 ms frame contains 320 samples and is independently encoded into 166 ADPCM bytes. The 16-byte BLE envelope makes a 182-byte characteristic value; the connection requires ATT MTU 185 or greater. Independent predictors prevent one lost frame from corrupting later frames.

The [BLE V1 protocol](ai-passport-ble-protocol.md) defines the Go Voice UUIDs, handshake, messages, byte layout and limits. It is the authoritative wire contract. The firmware does not perform ASR or store an entire recording. The host enforces the 15,000-frame/five-minute session bound.

## Session lifecycle

1. The user explicitly selects a peripheral. CoreBluetooth connects, verifies payload capacity, subscribes, reads device information and completes `hello` / `host_ready`.
2. UP starts capture. `ptt_down` captures the foreground application, creates a host generation and starts Speech with current vocabulary. Up to 75 early audio frames wait in memory while Speech starts.
3. Go validates controls and audio, assembles fragments, decodes ADPCM and orders PCM. Missing frames wait 80 ms; recovery is bounded to 100 silent frames per session. A three-second audio stall cancels the session.
4. UP stops capture. `ptt_up(finalSequence)` starts a tail wait bounded to 500 ms before recognition finalizes.
5. The host optionally refines the final ASR text, inserts it or leaves it on the clipboard, and reports terminal state to the badge.

DOWN sends Enter. OK deletes while idle or cancels an active request; a 650 ms long press clears input while idle or cancels the request. Disconnects, device errors, cancellation and stale native results cannot insert text into a later session. Each native request has a host generation separate from the BLE session ID.

## Native helpers and ownership

`gui/internal/app` composes services; the device coordinator owns session state and cancellation. Pure Go protocol code owns validation, codec work and ordering. UI callbacks emit actions and background updates return through `mygo.RunOnMain`.

BluetoothHelper and SpeechBridge are signed nested subprocess applications with bounded NDJSON and canonical Base64. They stop on parent exit and pipe closure. SpeechBridge wraps SpeechAnalyzer/SpeechTranscriber, model preparation, FoundationModels and TextInjector. Audio buffers remain in memory. See the [source layout](project-structure.md).

Firmware hardware access stays in BSP. The audio worker owns I2S and codec lifecycle, UI access obeys the LVGL lock, and button callbacks remain non-blocking. The [standby contract](development/engineering/standby-power-contract.md) describes audio pause, display pause and USB sleep-lock behavior.

## Vocabulary, refinement and insertion

Vocabulary persists terms, provenance, weights and user locks. Each speech session receives at most 100 recognition hints and freezes the complete correction vocabulary separately. A local pinyin index retrieves matching terms after recognition; only those terms, literal matches and the non-Chinese recognition hints reach refinement. Correctly spelled personal terms remain protected, and excessive personal candidates skip model correction without truncation. Separate read-only common and IT packs are enabled by default; they provide bounded, frequency-ranked suggestions without claiming personal-name protection or occupying Top 100. Their precompiled readings and cached index remain local, and settings changes wait for the next session. AI proposals cannot silently replace or delete locked terms. System vocabulary hints increase recognition probability and do not force substitutions. No language-model compilation runs after every utterance.

Apple FoundationModels is the default refinement engine. Failure, timeout, empty output or low fidelity retains the ASR original. Remote processing requires explicit configuration; it sends the final transcript and current vocabulary to the selected HTTPS endpoint, never audio. Local failure never silently switches to remote processing. API keys live only in macOS Keychain.

TextInjector captures the foreground target at session start. It respects Accessibility and Secure Input, waits for modifiers, pastes through System Events with a CGEvent fallback, and restores the original clipboard after 0.5 seconds only if clipboard ownership is unchanged. Missing permission, an unavailable target or Secure Input leaves the result on the clipboard. The HUD hides before insertion and does not take keyboard focus.

## USB installation and protected storage

The separate Rust FirmwareHelper uses pinned `espflash` and writes only the bundled application at `0x10000`, up to 3 MB. Preflight verifies image digest, chip, flash capacity and either of two exact device layouts before any erase/write: the original five-partition template or the measured nine-partition resource layout. Both use the same USB ROM application-write flow, independently of the existing bootloader's Recovery marker. The packaged manifest keeps the template contract; the device table is never replaced. Sector erasure ends no later than `0x310000`. There is no erase-all, bootloader or partition-table write path. See the [helper contract](../gui/firmware-helper/README.md) for layout details.

`cardid` at `0x356000`, permanent Recovery at `0x700000`, and the five-second UP-key bootloader hook are mandatory. The desktop app-only image and mini-program merged image serve different installation paths; see [Recovery compatibility](development/engineering/ble-recovery-compatibility.md).

## Privacy and acceptance

BLE V1 has no pairing, peer authentication or application-layer encryption. A persisted CoreBluetooth peripheral UUID supports selection and reconnects; it does not authenticate the device. The application labels this limitation. Authenticated transport is future work.

Bluetooth, Speech Recognition and Accessibility permissions are required for the full workflow; Automation is optional because insertion can use CGEvent. The Mac Microphone grant is optional and only requested for the hotkey. Settings and vocabulary persist locally; session audio and transcripts do not.

Build, host-test and device results are reported separately. Hardware recognition quality, sustained BLE behavior, permissions, insertion, current consumption and Recovery require the remaining checks in the [desktop acceptance record](../gui/docs/acceptance.md) and [firmware record](go-voice-firmware.md#validation).
