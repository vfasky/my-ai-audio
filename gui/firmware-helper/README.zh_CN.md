<p align="right"><strong>简体中文</strong> · <a href="README.md">English</a></p>

# FirmwareHelper

Go Voice 独立固件安装 helper，仅支持 macOS 上官方 AI Passport ESP32-C3 原生 USB Serial/JTAG。需要 Rust 1.95 或更新版本。固定 `espflash = 4.6.0`，Cargo.lock 固定解析后的依赖图。

```sh
cargo build --locked --release
cargo fmt --check
cargo test --locked
cargo clippy --locked --all-targets -- -D warnings
```

可执行文件为 `target/release/FirmwareHelper`。stdin/stdout 使用持久 NDJSON；stdout 仅输出事件。请求必须以换行结束，每行含换行最多 16 KiB。超长行会被完整丢弃并返回错误，然后继续处理下一行。请求串行执行，shutdown 在当前安装结束后生效，EOF 退出。不使用交互提示或 shell 命令。

```json
{"type":"scan"}
{"type":"install","port":"/dev/cu.usbmodem123","firmwarePath":"/absolute/app.bin","manifestPath":"/absolute/manifest.json"}
{"type":"shutdown"}
```

事件为 `ready`、`ports`（`devices` 仅含 `id`、`name`）、`progress`（`stage`、`percent`、`message`）、`complete`、`error`（`code`、`message`、`recoverable`）。阶段为 `preflight`、`writing`、`verifying`、`restarting`，百分比按阶段计算。失败安装不发送 complete。recoverable 表示进程可接受下一请求，不保证部分写入的应用能够启动。shutdown 不发送事件。设备名使用固定标签，不向 UI 输出 USB serial-number 字段或原始库错误。端口路径是打开设备所需的操作系统标识，可能包含系统赋予的识别字符。

manifest 所有字段必填，拒绝未知字段；大小、偏移、type、subtype 使用 JSON 整数，大小和偏移单位为字节，不接受十六进制字符串。SHA-256 为 64 位十六进制字符串，不区分大小写。支持的 manifest 为：

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

打开端口前校验 manifest、输入大小、ESP magic、固件 SHA-256 和扇区对齐擦除边界。安装时重新枚举端口，仅接受 VID/PID 为 `303a:1001` 的 `/dev/cu.*`。使用 ROM 模式连接，不上传 stub；确认芯片、安全状态和独立检测到的 8 MiB Flash。任何非零或未知安全标志、非零 Flash Encryption 计数器、安全响应缺失或 Secure Download Mode 都会保守拒绝。

预检仅读取 `[0x8000,0x9000)` 的 ESP-IDF 分区表。所有分区表都必须具有正确 MD5、擦除状态的尾部和零分区 flags。设备布局按两套精确白名单匹配（条目顺序可不同）：

- `partitions.csv` 中原有的五分区模板。
- 下表所列的实测九分区资源布局。

两种布局使用相同的 ROM 应用写入流程，保留设备现有的 bootloader 与 Recovery 存储。USB 写入不以模板专用的 bootloader 日志标记为前置条件。安装器不会检查、恢复或认证设备的 UP 键/Recovery 行为；随包 manifest 与社区合并固件仍保留各自的 Recovery 契约。不会读取、擦除或写入 `0x700000` 区域。

| 名称 | Type/subtype | 偏移 | 大小 |
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

即使 MD5 正确，缺失、重复、修改或额外增加的分区也会被拒绝。随包 manifest 保持原有五分区固件产物契约，绝不会作为新分区表写入设备。`plan_app_write` 独立限制扇区对齐后的擦除终点不超过 `0x310000`，并检查与每个非应用分区不重叠。不读取 cardid 内容。通过 `Connection::command(ReadFlashSlow)` 逐次读取 64 字节，避免 espflash 4.6.0 `read_flash_rom` 的累加偏移问题。ROM 命令不支持时返回 `PREFLIGHT_READ_UNAVAILABLE`，不会回退到整包刷写。连接后的预检失败会先尝试正常硬复位，再返回原始错误；若复位不可用，需要给设备重新上电。

唯一写操作是 `Flasher::write_bin_to_flash(0x10000, app, callbacks)`，开启验证、关闭跳过。其设备端 `FlashMd5` 比对在复位前完成，由 `ProgressCallbacks` 报告验证与复位阶段。最多填充三个 `0xff` 字节，扇区擦除范围保持在 factory 内。库结束 ROM 写入并执行硬复位，只有成功返回后才发送 complete。没有整片擦除、分区表写入、bootloader 写入或设备初始化操作。

稳定错误码包括 `INVALID_REQUEST`、`REQUEST_TOO_LARGE`、`INVALID_PATH`、`FILE_READ_FAILED`、`INVALID_FILE`、`INVALID_MANIFEST`、`INVALID_IMAGE`、`HASH_MISMATCH`、`INVALID_LAYOUT`、`SCAN_FAILED`、`PORT_NOT_ALLOWED`、`CONNECT_FAILED`、`CHIP_MISMATCH`、`SECURITY_UNAVAILABLE`、`UNSAFE_SECURITY_STATE`、`FLASH_SIZE_UNAVAILABLE`、`FLASH_SIZE_MISMATCH`、`UNSAFE_READ`、`PREFLIGHT_READ_UNAVAILABLE`、`FLASH_FAILED`、`VERIFICATION_UNAVAILABLE`、`OUTPUT_FAILED`。

主机测试不代表硬件验收。真实 USB 枚举、ROM 读取支持、安全响应、写入/擦除行为、校验失败、断开连接/断电以及 UP 键 Recovery 需要实机验证。manifest 是完整性元数据，不是签名真实性证明；布局检查无法以密码学方式认证物理设备。本 helper 不实现固件签名或密钥配置。
