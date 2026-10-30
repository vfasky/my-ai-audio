简体中文 · [English](README.md)

# Go Voice（my-ai-audio）

AI 驱动的 macOS 语音输入。AI Passport 固件和基于 MyGo 的 `go-voice` macOS 桌面端现已在同一仓库落地，并通过本机固件构建、GUI 构建和 host tests。

AI Passport / ESP32-C3 工牌是唯一音频采集端，连续把 ES8311 音频经 IMA-ADPCM / BLE 送到 Mac；基于 MyGo 构建的 `go-voice` 应用调用系统端侧 ASR，注入动态词库，使用 Apple 本地模型或用户明确选择的 OpenAI-compatible 服务润色，再把文字插入当前输入框。

仓库按 Vokie 固件项目组织：根目录生成 ESP-IDF 固件；`gui/` 是独立 Go module 的 MyGo 技术栈 `go-voice` macOS 应用，负责配对、实时波形与帧诊断、词库、AI 润色、Speech 和跨应用插字。只有按住可选的按住说话热键（默认 Shift+Option）时才使用 Mac 麦克风，音频不保存也不回放。

## 第一阶段固件

产品名与 BLE 广播名为 **Go Voice**。固件导入自 Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8`，保留 FoloToy MIT 归属。使用用户提供的 Go Voice Logo 和自有 UUID，保留参考按键交互、BLE V1 音频帧、状态 UI、功耗、开机音及抢占行为、受保护 Recovery 布局。

Go Voice 不再依赖 Vokie 桌面宿主。采集与文字输入需要实现 [Go Voice UUID 和 BLE V1 协议](docs/ai-passport-ble-protocol.zh_CN.md)的兼容主机，设备自身不执行 ASR。未修改的现有 Vokie 集成不兼容新 UUID。五分钟会话上限继续由参考主机负责。

- [固件范围、来源与验证](docs/go-voice-firmware.zh_CN.md)
- [品牌资源与复现](assets/README.zh_CN.md)
- [第三方声明](docs/THIRD_PARTY_NOTICES.zh_CN.md)：发布前须取得参考开机音的独立授权。
- 验证：`./tools/validate.sh --static`；激活 ESP-IDF 5.5.3 后运行 `./tools/validate.sh --firmware`。不得擦除设备身份或永久 Recovery。

## go-voice macOS 桌面端

`go-voice` 面向 macOS 26+ 和 Apple Silicon，基于 MyGo 构建，使用 Go Voice 自有 BLE UUID，不会自动连接扫描到的第一个设备。四页原生 UI 覆盖设备与本次语音、动态词库、AI 润色和安全 USB 固件安装；API key 仅存 macOS Keychain，本地模型失败不会静默切换云端。

固件页使用签名的 Rust `FirmwareHelper`，底层固定为 `espflash` 4.6.0。它识别 ESP32-C3 原生 USB Serial/JTAG，采用 fail-closed 方式检查芯片、安全状态、Flash 容量、分区表和镜像摘要，只向 `0x10000` 写入随 App 提供的 factory 应用；USB ROM 应用写入不依赖现有 Recovery 启动标记；不存在 erase-all、bootloader、分区表、NVS、PHY、`cardid` 或 Recovery 写入路径。

```bash
# 在仓库根目录启动带签名的调试版，监听变化并自动重构建、重启：
bun run dev
# 或验证后启动构建产物：
./tools/validate.sh --gui
open gui/build/debug/darwin-arm64/go-voice-debug.app
```

- [go-voice 使用、构建与隐私说明](gui/README.zh_CN.md)
- [GUI 实现与验收记录](gui/docs/acceptance.zh_CN.md)
- 构建产物：`gui/build/debug/darwin-arm64/go-voice-debug.app`
- 已验证应用包启动、四页显示、zh-CN 模型预热和真实 USB Serial/JTAG 枚举；安装器实际预检/写入/复位、真实工牌 BLE、Speech/辅助功能/自动化授权、ASR、润色和插字仍需交互式真机验收。

## 致谢

感谢 [FoloToy/ai-passport](https://github.com/FoloToy/ai-passport)、[Vokie 固件项目](https://github.com/alanshen6666-collab/vokie-ai-passport-firmware)及其贡献者，开放分享 AI Passport 的硬件支持、固件、BLE 音频协议与桌面集成参考实现。Go Voice 在这些工作之上继续开发，蓝牙 helper 与音频会话实现参考并改编自上游 Vokie 插件。

本仓库不再包含独立 Vokie 插件；`go-voice` 自行构建并使用原生 helper。上游 MIT 版权与许可证声明继续保留在 [LICENSE](LICENSE)、[第三方声明](docs/THIRD_PARTY_NOTICES.zh_CN.md)和[桌面端声明](gui/docs/THIRD_PARTY_NOTICES.zh_CN.md)中。

## 文档

- [文档索引](docs/README.zh_CN.md) — 使用、构建、验证、硬件与贡献指南。
- [项目结构](docs/project-structure.zh_CN.md) — 当前固件与桌面端源码布局。
- [系统架构](docs/passport-audio-input-architecture.zh_CN.md) — 音频传输、会话归属、识别与插字。
