<p align="right">
  <a href="CHANGELOG.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# go-voice release notes

Each `## <version>` section is the release note of one signed `go-voice` release. `scripts/release.sh` requires the section in both language files; the Simplified Chinese section is signed into the update manifest and shown in the in-app update window, because the app interface is Chinese. Write user-visible changes only and keep both sections aligned. See [updates and releases](updates.md).

## 0.7.0

- Fix USB firmware installation stopping with `RECOVERY_MARKER_MISSING` on supported five-partition devices. Both supported layouts now use the same bounded USB ROM application-write flow, with image, chip, security, flash-size and partition checks, device checksum verification and reset. Bootloader, device identity, resources and Recovery storage stay in place. The packaged installer completed a real installation on the five-partition device.
- Hold Shift+Option alone anywhere in macOS to dictate with the Mac microphone, then release to recognize and insert using the selected engine, vocabulary and refinement. Choose another chord or turn it off on General. Pressing another key cancels the recording so existing modifier shortcuts keep working. Microphone access is optional and requested on first use; online recognition sends this audio to the selected provider.
- Update the app framework and updater. Release builds check about ten seconds after launch and daily, stage verified updates until the app exits, and relaunch through LaunchServices.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. Bundled firmware remains 0.2.0. Open General to configure the microphone hotkey, or Firmware Installation to install over USB. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.7.0/gui/README.md). Live microphone/hotkey permissions, installed-update permission retention, nine-partition USB writes and interrupted-write recovery remain unverified.

## 0.6.2

- USB firmware installation now supports two exact device layouts: the original five-partition template with its Recovery boot hook, and the measured nine-partition resource layout that keeps the vendor bootloader, pictures and audio. Only the existing factory application is written; identity, resources and Recovery storage stay in place. A refused preflight check attempts a normal reset.

Install the Apple Silicon DMG on macOS 26 or later and replace the previous go-voice in Applications. Bundled firmware remains 0.2.0. Build and setup instructions are in the [application guide](../README.md). Writing firmware on both hardware layouts remains unverified.

## 0.6.1

- Fix the macOS deployment target: 0.6.0 declared macOS 26 but its main executable required macOS 27. The main app is now compiled and linked for macOS 26, including its C/C++ dependencies.
- Check the actual minimum system versions of the main app and all three helpers during packaging and update-archive verification, so a newer build SDK cannot silently raise the supported minimum.

Install the Apple Silicon DMG on macOS 26 or later and replace the previous go-voice in Applications. Bundled firmware remains 0.2.0. Build and setup instructions are in the [application guide](../README.md). Installation, permissions and Bluetooth dictation on a macOS 26 Mac remain unverified.

## 0.6.0

- Add Volcengine Doubao streaming speech recognition as an online engine, in the hour and concurrency plans. Partial text appears while you speak and the reply flagged as last becomes the final transcript. Its API key has its own Keychain account, Test Connection sends only silence, and a failure ends the session without switching engines. Choosing it sends each session's audio to Volcengine, without vocabulary.
- Make structured refinement an editor: it corrects misheard words from context and general knowledge beyond the vocabulary, including real words that do not fit the sentence, keeps only what you finally said after taking something back, and smooths broken grammar before adding paragraphs or lists. It still refuses summaries, answers and invented text. Disabled, proper-name and light modes are unchanged.
- Show when an API key is already saved: an empty key field on the refinement, speech recognition and Jev pages says Keychain holds one and that leaving it empty keeps it. The recognition drop-down lists each provider once and its models are chosen in the provider's card under one shared key, so changing models never asks for the key again. The status pill and device page name the chosen provider.
- Add two options for OpenAI-compatible refinement endpoints: provider routing (only, order and ignore lists, and a switch that forbids other providers) sent as the top-level provider object used by gateways such as DTMaaS, and a stream switch for services and models that only answer streamed requests. Streaming is not faster, because the result is still checked and inserted at once.
- Change the Alibaba Cloud Omni Realtime default model to qwen3.8-omni-flash-realtime and replace its free-form endpoint with a region (Beijing or Singapore) plus workspace ID; Custom still accepts a full WSS address. Existing Omni users must enter a workspace ID once; a saved model name is kept.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. Bundled firmware remains 0.2.0. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.6.0/gui/README.md). The live Doubao service, live DTMaaS provider routing, structured-refinement quality across models, installed-update permission retention and physical-device acceptance remain unverified.

## 0.5.0

- Include 96,336 common Chinese words and a 12,854-term IT pack, both enabled by default. Manage them in Vocabulary > Public Dictionaries; saved changes apply to the next dictation. The two packs contain 108,623 distinct terms after deduplication.
- Search the complete personal vocabulary and replacement targets for correction, including entries outside the Top 100 recognition hints. Public dictionaries stay separate from personal terms and do not consume recognition-hint slots.
- Compare public suggestions, personal names and the original wording with explicit source information. Correctly spelled personal terms remain protected; public frequency alone does not trigger a replacement. Public packs require AI refinement or Jev, while explicit replacement rules still work with both off.
- Retrieve public candidates locally from bundled, cached dictionaries. Only matching candidates enter an enabled remote model, never the full public dictionaries. Keep the original when candidate or context limits prevent a complete check.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. Open Vocabulary > Public Dictionaries to manage the default-enabled Common and IT packs. Bundled firmware remains 0.2.0. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.5.0/gui/README.md). Real-dictation correction quality, installed-update permission retention and physical-device acceptance remain unverified.

## 0.4.0

- Show the device and bundled firmware versions on the Firmware page, with an update reminder and a sidebar badge when the connected device is older than the copy packaged in go-voice. Bundled firmware is 0.2.0.
- Correct Chinese vocabulary and replacements on local word boundaries. Unique replacement targets fill unused Top 100 slots; final correction matches configured Chinese sources as complete tokens without crossing punctuation, including when AI refinement and Jev are off. Live previews keep literal replacement.
- Improve Jev vocabulary checks with local segmentation and the full transcript. Each occurrence compares overlapping homophone candidates against keeping the original; a replacement or protection applies only at 70% confidence. Requests send the full transcript, segmentation and matching candidates, never audio or unrelated vocabulary. Long requests are skipped. Live model quality remains unverified.
- Lower firmware standby draw: key scanning slows while the screen is off, the microphone path closes after five quiet seconds, and the display sleeps with the backlight. The first key after screen-off can take about 50 ms longer.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. After connecting a badge, compare versions on the Firmware page and install 0.2.0 when prompted. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.4.0/gui/README.md). Live Jev quality, word-boundary correction on real dictation, measured standby current and a physical firmware update remain unverified.

## 0.3.0

- Choose Alibaba Cloud Omni Realtime WebSocket on the Speech Recognition page. Set its endpoint, model and API key; audio streams while recording and is submitted on key release. Only input transcription is inserted, with credentials stored separately from Alipay MaaS.
- Local builds are named `go-voice-debug` and use separate macOS permission identities for the main app and all helpers, so development builds do not replace the release app's permission records.
- Start a signed watch session with `bun run dev` from the repository root. Go source and configuration changes rebuild and restart the debug app automatically.
- Visit the [product website](https://go-voice.yzsaas.net/) for the latest Apple Silicon download, setup information and release history.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. Select Alibaba Cloud Omni Realtime on the Speech Recognition page and enter an API key for the chosen endpoint and model. Build and setup instructions are in the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.3.0/gui/README.md). Live Omni recognition, physical-device dictation and permission retention across an installed update remain unverified.

## 0.2.0

- Choose online speech recognition on the new Speech Recognition page: Alipay MaaS Fun-ASR and Paraformer v2 stream text while you speak; Qwen3-ASR-Flash transcribes the complete recording after you release the key. Apple on-device recognition remains the default. Online recognition sends session audio to Alipay MaaS, keeps its API key in Keychain, and reports failures without switching engines.
- See live recognition in the floating capsule: new words slide in from the right, and Apple's local recognizer now updates about once a second. The preview bubble appears during AI refinement so you can insert the ASR original directly.
- Optionally enable Jev on the vocabulary page to check Chinese homophone corrections before refinement, including when AI refinement is off. Supports TypeSafe and compatible System One providers such as Alipay MaaS. Only matching text spans, surrounding context and candidate terms are sent; API keys stay in Keychain. The check is off by default and is skipped after four seconds or on failure.
- Enable Open at Login on the new General page. go-voice starts in the menu bar and opens the permissions guide only when a required permission is missing; the switch follows macOS Login Items settings.

Install the Apple Silicon DMG on macOS 26 or later, or choose Check for Updates… in an installed release. See the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/README.md) for setup and the [release guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/docs/updates.md) for building and signing. Physical-device dictation, live provider quality, login-item behavior and an installed update remain unverified for this release.

## 0.1.0

- First signed and notarized release of go-voice for Apple Silicon Macs running macOS 26 or later.
- Receive voice from a selected FoloToy AI Passport over Bluetooth, transcribe it on this Mac, optionally refine it, and insert it into the application you dictated into. A floating capsule previews the text and lets you cancel or insert it directly.
- Manage vocabulary and local replacement rules, and import or export them together as JSON. Replacements also work with AI refinement turned off.
- Choose Apple FoundationModels or an OpenAI-compatible refinement engine with Chat Completions or Responses. Correct vocabulary terms first, remove meaningless fillers in light mode, and test remote settings before saving.
- Follow the permissions guide and install the bundled application firmware over USB while preserving device identity and permanent Recovery.
- Check for Updates… in the menu bar now installs signed updates; release builds also check once a day.

Install the Apple Silicon DMG, drag go-voice to Applications, open it, and complete the permissions guide before selecting your FoloToy AI Passport. See the [application guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/README.md) for setup and the [release guide](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/docs/updates.md) for building and signing.
