<p align="right">
  <a href="ai-passport-ble-protocol.zh_CN.md">简体中文</a> · <strong>English</strong>
</p>

# AI Passport BLE Voice Protocol

This page defines the Go Voice UUIDs. Protocol behavior retains Vokie V1; Vokie host behavior below describes the reference implementation. Unmodified Vokie built-in clients do not support these new UUIDs. Go Voice requires a compatible host, not a Vokie installation. See [phase-one scope](go-voice-firmware.md).

Protocol version 1 uses one Bluetooth Low Energy GATT service. The AI Passport
is the peripheral; Vokie or another compatible application is the central.

## Service

| Item | UUID | Direction | Properties |
| --- | --- | --- | --- |
| Voice service | `676f0001-766f-6963-652d-617564696f00` | n/a | n/a |
| Control | `676f0002-766f-6963-652d-617564696f00` | both | write-with-response, notify |
| Audio | `676f0003-766f-6963-652d-617564696f00` | device to host | notify |
| Device info | `676f0004-766f-6963-652d-617564696f00` | device to host | read |

The device advertises as `Go Voice` and allows one connection. The host
must negotiate `ATT_MTU >= 185`, subscribe to Control and Audio, receive and
validate `hello`, and then send `host_ready`. The firmware ignores PTT input
until this handshake finishes.

Control accepts acknowledged writes only. Write-without-response and Control
reads are outside the V1 contract; read Device info to retrieve `hello`.

## Control messages

Control values are complete UTF-8 JSON objects with `v: 1` and a maximum size
of 182 bytes. Device-to-host messages are:

```json
{"v":1,"type":"hello","device":"ai-passport","fw":"0.3.0","codec":"ima-adpcm","sampleRate":16000,"channels":1,"frameMs":20}
{"v":1,"type":"ptt_down","sessionId":123,"seq":0}
{"v":1,"type":"ptt_up","sessionId":123,"seq":1,"finalSequence":7}
{"v":1,"type":"button_event","button":"down","event":"click","durationMs":0,"seq":2}
{"v":1,"type":"button_event","button":"ok","event":"long","durationMs":650,"seq":3}
{"v":1,"type":"device_error","sessionId":123,"message":"transport"}
```

- `sessionId` and `seq` are unsigned 32-bit integers.
- Audio sequence numbers restart at zero for each PTT session and are independent
  of the control-event `seq`.
- An empty session uses `finalSequence: 4294967295`.
- `device_error.message` is diagnostic only and must not contain audio or
  transcript data.
- Supported device errors are currently `audio_read`, `mtu`, and `transport`.

Host-to-device messages are:

```json
{"v":1,"type":"host_ready"}
{"v":1,"type":"host_state","state":"processing"}
```

The firmware accepts `ready`, `recording`, `processing`, `success`, and `error`.
The Vokie plugin normally sends `ready`, `processing`, `success`, or `error`.
The optional message field is diagnostic and must not contain transcripts.

## Button semantics

`button_event` covers non-PTT actions. The current Vokie host mapping is:

| Device action | Host behavior |
| --- | --- |
| `UP` click while idle | Send `ptt_down` and begin capture |
| `UP` click while capturing | Finish the current frame and send `ptt_up` |
| `DOWN` click | `send_enter` |
| `OK` click while a request is active | `session_cancel` |
| `OK` click while idle | `delete_char` |
| `OK` long press while a request is active | `session_cancel` |
| `OK` long press while idle | `clear_input` |

The host keeps a request active through capture, ASR, post-processing, return,
and paste, until it receives a terminal `session_state`. The firmware recognizes
an `OK` long press at 650 ms.

Short clicks are emitted on debounced key release, without waiting for the driver's 180 ms double-click window. Rapid consecutive taps are separate actions. Delayed single/double-click callbacks are ignored by Vokie. OK uses an independent 650 ms hold timer, including when held immediately after a short tap; releasing after a long action does not also delete a character. Presses that started before the host handshake or on another connection do not produce an action.

## Audio fragment envelope

Every Audio notification contains one complete fragment. Multi-fragment frames
are keyed by `(sessionId, sequence)`:

| Offset | Size | Value |
| ---: | ---: | --- |
| 0 | 2 | magic `0x5041`, little-endian (`41 50`) |
| 2 | 1 | envelope version `1` |
| 3 | 1 | reserved, zero |
| 4 | 4 | session ID, little-endian |
| 8 | 4 | audio sequence, little-endian |
| 12 | 1 | fragment index |
| 13 | 1 | fragment count |
| 14 | 2 | payload length, little-endian |
| 16 | variable | ADPCM payload bytes |

The payload length must match the notification length. The sender fragments
against the negotiated `ATT_MTU - 3 - 16` payload size. Hosts must support up to
64 fragments, ignore duplicate fragments, and may receive fragments out of
order. With MTU 185, one 166-byte ADPCM frame fits in one notification.

Control notifications are not fragmented: each notification contains exactly
one complete JSON value.

## IMA ADPCM frame

Each independent 20 ms frame represents 320 samples of 16 kHz, 16-bit mono
PCM. The encoded payload is exactly 166 bytes:

| Offset | Size | Value |
| ---: | ---: | --- |
| 0 | 2 | initial predictor, signed int16 little-endian; sample 0 |
| 2 | 1 | IMA step index, `0..88` |
| 3 | 2 | sample count, unsigned little-endian; currently `320` |
| 5 | 1 | reserved, zero |
| 6 | 160 | 319 low-nibble-first IMA ADPCM nibbles plus one zero pad nibble |

The decoder clamps the predictor to int16 and the step index to `0..88`. It
ignores the final high pad nibble according to `sampleCount`.

## Lifecycle and loss handling

1. The host connects, validates MTU, and subscribes to Control and Audio.
2. The device sends `hello`; after validation, the host sends idempotent
   `host_ready`.
3. The user clicks `UP`; the device sends `ptt_down` and starts capture.
4. The device sends audio frames in ascending sequence order.
5. The user clicks `UP` again; the device finishes the current frame and sends
   `ptt_up` with the final sequence or the empty sentinel.
6. The host waits for complete frames through `finalSequence`, decodes them, and
   sends its recording-session stop command.
7. A compatible host may replace a missing frame with 320 silent samples, but
   recovery must be bounded. The Vokie plugin permits at most 100 missing frames.
8. A disconnect or terminal device error cancels the matching host request and
   clears buffers.
9. A repeated valid `hello` after resubscription causes the host to resend
   `host_ready` without changing an active session identity.

The Vokie plugin bounds a session to 15,000 audio frames (five minutes), waits
1.2 seconds for session acceptance, treats three seconds without valid audio as
a stall, and waits at most 500 ms for announced tail frames after `ptt_up`.
These are host implementation limits, not additional BLE fields.

## Security and device selection

Protocol V1 provides no BLE pairing, peer authentication, or application-layer
encryption. A CoreBluetooth peripheral UUID is only a routing preference and
cannot prove device identity. Deployments that need protection from nearby
impersonation or eavesdropping must add pairing or application-layer
cryptography in a future protocol version.

The Vokie first-party host currently uses a macOS Apple Silicon CoreBluetooth
helper and can persist a preferred peripheral UUID. Other hosts may implement a
different selection policy while preserving the wire contract above.
