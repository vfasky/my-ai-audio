<p align="right"><a href="README.zh_CN.md">简体中文</a> · <strong>English</strong></p>

# FirmwareHelper

Standalone Go Voice installer helper for the official AI Passport ESP32-C3 native USB Serial/JTAG connection on macOS. Requires Rust 1.95 or newer. The library is pinned to `espflash = 4.6.0`; Cargo.lock pins the resolved dependency graph.

```sh
cargo build --locked --release
cargo fmt --check
cargo test --locked
cargo clippy --locked --all-targets -- -D warnings
```

The binary is `target/release/FirmwareHelper`. It reads persistent NDJSON on stdin and writes only NDJSON events to stdout. Each request must end with a newline and occupy at most 16 KiB including that newline. Oversized lines are drained and rejected; the process accepts the next line. Requests run sequentially; shutdown takes effect after the active installation finishes. EOF exits. No interactive prompts or shell commands are used.

```json
{"type":"scan"}
{"type":"install","port":"/dev/cu.usbmodem123","firmwarePath":"/absolute/app.bin","manifestPath":"/absolute/manifest.json"}
{"type":"shutdown"}
```

Events are `ready`, `ports` (`devices` with only `id` and `name`), `progress` (`stage`, `percent`, `message`), `complete`, and `error` (`code`, `message`, `recoverable`). Stages are `preflight`, `writing`, `verifying`, `restarting`; percentages are per stage. No completion is emitted on a failed install. `recoverable` means the process can accept another request, not that a partially written application can boot. Shutdown emits no event. Device names are fixed labels, and no USB serial-number property or raw library error reaches the UI. A device path may contain OS-assigned identifying characters, as it is needed to open the port.

Manifest fields are mandatory, unknown fields are rejected, and sizes/offsets/type/subtype are JSON integers in bytes (no hex strings). The SHA-256 is a 64-character hex digest, case insensitive. The supported manifest is:

```json
{
  "schemaVersion": 1,
  "product": "Go Voice",
  "chip": "esp32c3",
  "flashSize": 8388608,
  "appOffset": 65536,
  "appMaxSize": 3145728,
  "appSize": 123456,
  "appSha256": "REPLACE_WITH_64_HEX_CHARACTERS",
  "partitionTableOffset": 32768,
  "partitionTableSize": 3072,
  "partitions": [
    {"name":"nvs","type":1,"subtype":2,"offset":36864,"size":24576},
    {"name":"phy_init","type":1,"subtype":1,"offset":61440,"size":4096},
    {"name":"factory","type":0,"subtype":0,"offset":65536,"size":3145728},
    {"name":"cardid","type":1,"subtype":2,"offset":3497984,"size":16384},
    {"name":"recovery","type":0,"subtype":32,"offset":7340032,"size":1048576}
  ],
  "recoveryMarker": "UP held: booting permanent recovery"
}
```

Before opening a port, the helper validates the manifest, input size, ESP magic byte, firmware SHA-256 and sector-rounded write boundary. It re-enumerates ports and accepts only `/dev/cu.*` with USB VID/PID `303a:1001`. It connects in ROM mode without uploading a stub and confirms chip, security status and independently detected 8 MiB flash. Unknown/nonzero security flags, any nonzero flash encryption counter, missing security response or secure download mode are refused conservatively.

Preflight reads only `[0x8000,0x9000)` for the ESP-IDF partition table. Every table needs a valid MD5 record, erased trailing bytes and zero partition flags. Device layouts are matched against two exact allowlists (entry order may differ):

- The original five-partition template from `partitions.csv`.
- The measured nine-partition resource layout below.

Both layouts use the same ROM application-write flow, preserving their existing bootloader and Recovery storage. The template-specific bootloader log marker is not a prerequisite for USB writes. This installer does not check, restore or certify the device's UP-key/Recovery behavior. The packaged manifest and merged community firmware retain their separate Recovery contract. Nothing at `0x700000` is read, erased or written.

| Name | Type/subtype | Offset | Size |
| --- | --- | --- | --- |
| nvs | 1/2 | `0x9000` | `0x6000` |
| phy_init | 1/1 | `0xf000` | `0x1000` |
| factory | 0/0 | `0x10000` | `0x300000` |
| imgstore | 1/130 | `0x310000` | `0x20000` |
| imgframe | 1/130 | `0x330000` | `0x26000` |
| cardid | 1/2 | `0x356000` | `0x4000` |
| imguser | 1/130 | `0x35a000` | `0x20000` |
| audio | 1/130 | `0x37a000` | `0x80000` |
| imgava | 1/130 | `0x3fa000` | `0x100000` |

Missing, duplicate, modified or additional partitions are refused, even with a valid MD5. The packaged manifest remains the original five-partition artifact contract; it is never written to the device as a new table. `plan_app_write` separately bounds the sector-rounded erase end to `0x310000` and checks non-overlap with every non-application partition. No cardid contents are read. The explicit 64-byte `Connection::command(ReadFlashSlow)` loop avoids the cumulative-offset defect in espflash 4.6.0 `read_flash_rom`. Unsupported ROM commands return `PREFLIGHT_READ_UNAVAILABLE`; there is no fallback to a full flash image. A preflight failure after connection requests a normal hard reset before returning its original error; if reset is unavailable, power-cycle the device.

The sole mutation is `Flasher::write_bin_to_flash(0x10000, app, callbacks)` with verification enabled and skip disabled. Its `FlashMd5` comparison runs on device before reset, and `ProgressCallbacks` reports verification and restart stages. Padding is at most three `0xff` bytes; sector erasure stays inside factory. The library finalizes the ROM write and performs a hard reset; completion is sent only after it returns successfully. No erase-all, partition-table write, bootloader write, or provisioning action exists.

Stable error codes include `INVALID_REQUEST`, `REQUEST_TOO_LARGE`, `INVALID_PATH`, `FILE_READ_FAILED`, `INVALID_FILE`, `INVALID_MANIFEST`, `INVALID_IMAGE`, `HASH_MISMATCH`, `INVALID_LAYOUT`, `SCAN_FAILED`, `PORT_NOT_ALLOWED`, `CONNECT_FAILED`, `CHIP_MISMATCH`, `SECURITY_UNAVAILABLE`, `UNSAFE_SECURITY_STATE`, `FLASH_SIZE_UNAVAILABLE`, `FLASH_SIZE_MISMATCH`, `UNSAFE_READ`, `PREFLIGHT_READ_UNAVAILABLE`, `FLASH_FAILED`, `VERIFICATION_UNAVAILABLE`, and `OUTPUT_FAILED`.

Host tests do not establish hardware acceptance. Real USB enumeration, ROM read support, security responses, write/erase behavior, checksum rejection, disconnection/power loss and UP-key recovery need device validation. The manifest is integrity metadata, not a signed authenticity proof; layout checks do not cryptographically authenticate a physical board. The helper does not implement firmware signing or key provisioning.
