简体中文 · [English](acceptance.md)

# 实现与验收记录

最近验证记录日期：2026-10-30。每个日期小节描述其当时测试的实现与应用包。已通过随包 FirmwareHelper 验证五分区设备上的 USB 应用安装；历史结果不能代替新发布构建的验证。

## USB ROM 应用安装（2026-10-30）

Build：签名的调试应用与三个 helper PASS。Host tests：GUI 门禁（20 个 Bun 测试、15 个 Rust 测试、9 个 Swift 测试、Go 测试/vet、Clippy、固件资源、macOS 兼容性与签名）和仓库静态/主机检查 PASS。根完整门禁因缺少 ESP-IDF 5.5.3 环境停止；未重新构建固件。

Device tests：使用新应用包中的 FirmwareHelper，通过 GUI 使用的同一套 NDJSON 扫描/安装协议验证 PASS。ESP32-C3 / 8 MiB 设备的分区表精确匹配五分区模板，没有预期的 bootloader 日志标记，且 Recovery 镜像头为擦除状态。随包 0.2.0 应用（1,162,848 字节，SHA-256 `30be20312ed0cd6e72935e6a96df47f4029fbafc267f960cb223ac7ae8b325ad`）写入 `0x10000`。helper 依次上报预检、写入、验证、重启阶段，最后返回 complete；设备端校验和复位成功。安装前后独立读取确认 bootloader 与分区表的 SHA-256 摘要相同。未读取设备身份内容。

安装器不再将 bootloader 日志字符串作为 USB 安装前置条件。两种已知布局仍保留精确分区验证与 factory 擦除边界；随包 manifest 和社区合并镜像仍保留 Recovery 契约。本次操作不会修复缺失的 Recovery 功能。

Unverified：九分区布局的真实 USB 安装、写入中断行为、原生界面安装按钮操作、UP 键/Recovery 行为，以及按键和音频验收。此前独立的直接 USB 测试另已观察到固件启动并广播 Go Voice。下方为历史验收记录。

## 两种设备布局验证（2026-10-23）

Build：Apple Development 签名的调试应用与三个 helper PASS，均兼容 macOS 26。Host tests：GUI 门禁（20 个 Bun 测试、15 个 Rust 测试、6 个 Swift 测试、Go 测试/vet、Clippy 与打包检查）和仓库静态/主机检查 PASS。根完整门禁因缺少 ESP-IDF 5.5.3 环境停止；继续使用已有的 0.2.0 随附固件，其摘要与 manifest 检查通过。

只读设备诊断确认了 ESP32-C3、8 MiB Flash、MD5 正确的九项资源分区表，且未发现模板的 Recovery 标记。仅读取 bootloader 和分区表区域，随后请求正常复位，没有擦除或写入 Flash。脱敏主机 fixture 逐字节复现了实测分区表。回归测试保留原有五分区布局及其必需的 Recovery 标记检查，接受精确的九分区布局，拒绝修改/重复/缺失条目、错误 MD5/flags，并将两种布局的扇区擦除范围限制在现有 factory 分区内，终点不超过 `0x310000`。

Device tests：枚举与只读诊断 PASS；应用写入和设备端校验 NOT RUN，因为准备写入测试时系统已无法枚举到 USB 设备。Unverified：两种硬件的真实安装、写入后 bootloader/分区表保持一致、中断恢复，以及资源布局设备原厂 UP 键/Recovery 的行为。兼容性依据实测布局选择，不凭硬件新旧或型号名称推断。下方是历史验收记录。

## Omni WebSocket 与调试身份验证（2026-10-20）

Build：Apple Development 签名的 `go-voice-debug.app` PASS。Host tests：`./tools/validate.sh --gui`、ASR race 测试及根静态检查 PASS。Device tests：NOT RUN。根完整门禁因缺少 `idf.py` / 未激活 ESP-IDF 5.5.3 环境，在固件编译前停止；GUI 门禁单独验证了现有包内固件镜像。

TLS WebSocket fixture 验证 Bearer 鉴权、会话配置、PCM 字节一致性、预览替换、手动提交、最终消息关联、错误、静音连接测试和取消。打包验证检查主应用及三个 helper 的 `.debug` 标识、签名、arm64 可执行文件及固件 manifest。调试元数据在临时 staging 中生成，正式版配置保持不变。Unverified：真实阿里云百炼端点、设备到应用的听写、首次调试授权及重构建后正式版/调试版 TCC 授权保留。下方既有验收证据仍属于历史记录。

## 固件打包验收

集成后的 FirmwareHelper 变更通过 11 个打包 fixture 测试（59 个断言）、9 个 Rust 测试、Bun 到 Rust 的 manifest 契约探针、含 race 的 Go 测试、Swift 测试、Clippy、shell 语法、helper Info.plist、完整 App 打包、arm64 检查及内外签名校验。Build：PASS。Host tests：PASS。Device tests：仅只读 USB 枚举 PASS；预检/写入/复位 NOT RUN。

根固件门禁现同时保留已验证的合并镜像和原始应用。GUI 打包校验普通、非空 ESP 镜像（`0xE9`）、3 MB 上限和按 4 KiB 向上取整后不越过 `cardid` 的擦除终点。原子写入 schema-v1 manifest，包含小写 SHA-256 摘要。`partitions` 是包含 NVS、PHY、factory、`cardid` 与 Recovery 完整五项布局的数组；type/subtype/offset/size 使用数字形式 ESP 分区值。构建退出时清理临时 firmware staging。锁定依赖构建的 Rust release 可执行文件作为独立嵌套 app，先于外层 MyGo 包签名。

Rust、Go、Swift、固件打包和签名层的集成主机门禁现已通过。Cargo metadata 显示 121 个已解析第三方 package 均带许可证元数据；实际 arm64 依赖树包含 MPL-2.0 的 `serialport` 4.10.1，应用现随附其完整许可证和精确源码链接。正式分发前仍须完成最终版权归属审计。

真机验收仍须确认仅向 `0x10000` 写入应用，预检拒绝错误芯片/Flash/分区表/Recovery 标记或镜像摘要，不存在 erase-all 路径，并保留 bootloader、分区表、`cardid` 与永久 Recovery。断线/失败后应测试按住 UP 上电五秒进入永久 Recovery，通过小程序重新安装。包内 manifest、USB 枚举或主机测试通过不能推断固件真机成功。Unverified：实际 USB 预检、刷写、复位与 Recovery 行为，以及正式发布前的最终版权归属审计。

## 已实现契约

- 薄 Go 入口、独立 module、MyGo 0.7.3 原生 UI 和标准 debug 打包。正式版应用名称为 `go-voice`，bundle id 为 `yz.sh.mygo-go-voice`；本地构建为 `go-voice-debug` / `yz.sh.mygo-go-voice.debug`，每个 helper 标识也追加 `.debug`。BLE 产品身份仍为 Go Voice。
- 严格控制校验拒绝重复/未知键、尾随 JSON、非法 UTF-8、错误数字类型、不支持的格式、缺失必填字段和超长控制消息。Audio envelope 校验、64 分片组装、去重、确定性有界淘汰、独立 166B IMA-ADPCM、320 样本 PCM 与有界排序均为纯 Go。
- 流处理有 75 帧早到缓存、80ms 缺口等待、累计 100 帧补静音上限、500ms 已声明尾帧等待、3 秒 stall 和 15000 帧/5 分钟上限。不保存音频历史即可计算 RMS、peak、32 个波形 bins、时长和收帧/缺帧/补帧/重复计数。
- 协调器测试覆盖 start、重复 start、早到帧、Speech ready 前 release、finish、润色、插字确认、断线、取消、旧结果、接受超时、连续复用 PTT 序号 0/1 的多段会话，以及使用独立序号空间的按键事件。每个原生 Speech 请求使用不同于 BLE session ID 的宿主代次 ID。设备取消先清除当前请求，再发送原生取消。
- CoreBluetooth helper 仅连接明确选择的 UUID，订阅 Control/Audio，读取 DeviceInfo，确认至少 182 字节 ATT 载荷，支持写重试、重连和忘记设备。处理延迟 MTU 完成。BluetoothHelper 与 SpeechBridge 作为签名嵌套 helper app 打包，使 macOS TCC 能解析各自用途说明和稳定身份；FirmwareHelper 作为不带 TCC 专用 entitlement 的独立签名嵌套 App 打包。父进程退出、stdin EOF、信号与有界命令处理均可终止 helper。Go 限制原生行长、解码载荷和字符串字段。
- 真实 SpeechAnalyzer/SpeechTranscriber 接收 16kHz Int16 单声道 PCM，使用 volatile results 和每会话 contextual strings，准备/保留 zh-CN 模型，发送带会话身份的 partial/final/error/cancel 事件，从不打开 Mac 音频输入。helper 使用 `RunLoop.main.run()` 保持物理主线程存活，因为 Speech 框架不能依赖原主线程已退出的 dispatch main queue。Swift Package 使用已安装 SDK 的真实 API 编译。两个 helper 都以可读事件处理长连接 stdin；真实进程回归测试保持 stdin 打开并要求命令立即生效。
- TextInjector 在 start 时捕获前台目标，检查辅助功能/Secure Input，处理修饰键，优先 System Events、失败使用 CGEvent；0.5 秒后仅在归属标记和 change count 均未变时恢复完整剪贴板快照。目标不存在或已变化则仅留剪贴板。
- 词库持久化所有要求字段，去重，提供手动 CRUD 和锁定，拒绝 AI 删除/覆盖锁定词，注入 Top 100。润色默认使用新的本地 FoundationModels session，云端必须明确选择。模式为 off/names/light/structured，30 秒超时，空输出/错误/低忠实度回退最终 ASR 原文。远端 HTTPS 拒绝 URL 内嵌凭据和重定向，bearer 凭据仅从 Keychain 获取。
- UI 不管理 helper 进程或 Speech 会话。应用组装层连接服务，并使用 `mygo.RunOnMain` 分发后台结果。本次音频/转写状态不持久化。麦克风权限文案和 audio-input entitlement 只用于按住说话热键；只在按住组合键期间打开输入设备。

## 自动验证证据

| 检查 | 结果 |
| --- | --- |
| Cargo fmt/test/Clippy | PASS：9 个 Rust 测试，固定 `espflash` 4.6.0 |
| Bun 打包与 Rust manifest 契约 | PASS：11 个 fixture 测试 / 59 个断言；不写硬件 |
| gofmt 检查 | PASS |
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `swift test --package-path macos` | PASS：4 个测试，不需要真实 BLE 或 Speech 授权 |
| `./scripts/build.sh` | PASS：`go-voice` debug arm64 App、两个 Swift helper、签名 Rust FirmwareHelper 与 app-only 固件资源 |
| 包内固件/manifest 与 helper 签名 | PASS：SHA-256/大小/布局、三个嵌套签名、arm64 与外层 deep signature |
| `codesign --verify --deep --strict build/debug/darwin-arm64/go-voice.app` | PASS |
| 根固件构建及静态门禁 | PASS：官方 ESP-IDF 5.5.3 容器构建和 host 门禁见根固件验收记录 |
| 应用包启动及界面 | PASS：ad-hoc App 已启动，四页均正常显示，模型报告 `zh-CN model warmed`，固件页显示一个匿名兼容 USB 设备 |
| USB 固件行为 | 枚举 PASS；预检/写入/校验/复位及中断写入 Recovery NOT RUN |

Go 测试覆盖严格 JSON、静音和非零 ADPCM 黄金向量、预测值限幅、非法编码字段、分片乱序/重复/64 分片/淘汰、缺帧与尾帧边界、五分钟上限、波形计量、有界 helper 解析/canonical Base64、保持管道打开时发送真实 helper 命令、连续两段复用线路控制序号 0/1 的 PTT 会话、独立按键事件去重、词库锁定/持久化、显式云端选择/本地回退、超时/忠实度、不读取凭据的 Keychain 框架符号及查询绑定，以及协调器生命周期。Swift 测试覆盖 PCM 边界/canonical Base64、代次身份、明确设备选择/MTU 和条件剪贴板恢复。

首次 MyGo 构建暴露其关闭 cgo 的编译路径。最终实现改用 purego Security.framework 与 CoreFoundation 绑定，已通过标准 MyGo 构建，没有引入凭据文件后端。

## 剩余验收

Build：PASS。Host tests：PASS。Device tests：仅 USB 枚举 PASS；固件预检/写入/复位 NOT RUN。

Unverified：真实 CoreBluetooth 发现和无线丢链重连；五分钟 FoloToy AI Passport 音频流；实际外围设备上的 MTU 推断；Speech 授权和首次资产下载；识别质量/延迟；本地 FoundationModels 输出质量；真实 Keychain CRUD 授权；用户明确选择的远端接口；辅助功能/自动化授权；真实粘贴中的 Secure Input 和剪贴板归属；微信、VS Code、Chrome、终端插字；休眠/唤醒和连接中退出。单测及应用包启动/模型预热成功证明了归约器、管道和构建边界，不代表剩余操作系统/设备交互已验证。

已记录的应用为本地开发 ad-hoc 签名。正式版现已使用 Developer ID 签名、公证并 staple，并通过 Ed25519 签名的归档自动更新（[更新、签名与公证](updates.zh_CN.md)）；尚未安装、启动或更新过任何正式版。实际发布、Gatekeeper 首次打开、从已公开旧版本更新且 TCC 授权保留，以及更新后 helper 签名仍待用户检查。改变身份的 debug 重构建可能需要重新授予 TCC 权限。V1 未认证，不得宣传已认证或端到端加密；设备页不再单独显示这条提示。应用主路径没有用 mock 静默替代设备或系统权限行为。

已记录的验收使用官方 ESP-IDF 5.5.3 容器重新构建固件，运行完整 Rust/Go/Swift/打包/签名门禁，检查 App 内 arm64 二进制和资源，启动真实 App、进入第四个固件页面并确认匿名枚举到一个兼容 USB Serial/JTAG 设备；刻意没有选择设备或触发安装。
