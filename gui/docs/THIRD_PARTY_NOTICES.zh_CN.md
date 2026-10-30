简体中文 · [English](THIRD_PARTY_NOTICES.md)

# 第三方声明

MyGo 是独立实现的 Go Voice 桌面宿主。Go Voice 图稿来自 `assets/brand/go-voice-logo.png`，叠加黑色圆角底后生成 `gui/resources/icon.png` 供打包使用；本声明不授予额外图稿或商标权利。

| 来源 | 用途 | 许可证 / 声明 |
| --- | --- | --- |
| FoloToy AI Passport 及已导入的 Vokie 参考版本 `ce26ea043251102155e54bc9503eb8133d5cafa8` | Bluetooth helper 改编自上游 Vokie 插件的 `AiPassportHelper.swift`；ADPCM 与会话行为参考其 worker 移植。本仓库不再附带独立插件 | MIT，Copyright (c) 2026 FoloToy；完整声明见 `licenses/FoloToy-MIT.txt` |
| MyGo v0.7.3 | 原生应用、控件、打包 | MIT，Copyright (c) 2026 MyGo contributors；`licenses/MyGo-MIT.txt` |
| purego v0.11.1 | Security / CoreFoundation 绑定和 MyGo 依赖 | Apache-2.0；`licenses/purego-Apache-2.0.txt` |
| go-text/typesetting v0.3.5 | MyGo 文本排版依赖 | 原始 LICENSE 和 HarfBuzz 声明保留在 `licenses/` |
| golang.org/x/image v0.46.0 | MyGo 图像依赖 | BSD；`licenses/x-image-BSD.txt` |
| golang.org/x/net v0.59.0 | MyGo 网络依赖 | BSD；`licenses/x-net-BSD.txt` |
| golang.org/x/text v0.43.0 | MyGo 语言区域（i18n）依赖 | BSD；`licenses/x-text-BSD.txt` |
| go-pinyin v0.21.0，<https://github.com/mozillazg/go-pinyin> | 提供拼音读音，用于在润色时找出词库词的同音字 | MIT，Copyright (c) 2016 mozillazg；`licenses/go-pinyin-MIT.txt` |
| gojieba v1.4.7，<https://github.com/yanyiwu/gojieba>，含随附的 CppJieba、limonp 和词典 | 本地中文词边界，以及编译后的 96,336 词通用纠错词库 | MIT；上游声明见 `licenses/gojieba-MIT.txt`、`licenses/cppjieba-MIT.txt` 和 `licenses/limonp-MIT.txt` |
| THUOCL IT，版本 `a30ce79d895d01ab5132a5c74c29703ff7efb4cc`，<https://github.com/thunlp/THUOCL> | 离线 IT 纠错候选：从 16,000 行源数据筛选 12,854 个中文词条，预计算单字读音 | MIT，Copyright (c) 2018 THUNLP；`licenses/THUOCL-MIT.txt`。源文件及生成文件摘要保留在 `internal/wordlists/data/manifest.json` |
| espflash 4.6.0，<https://github.com/esp-rs/espflash> | FirmwareHelper 使用的 USB 刷写 Rust 库；属于 esp-rs 项目，并非 Espressif 官方主要刷写工具 | MIT OR Apache-2.0；上游完整文本见 `licenses/espflash-MIT.txt` 和 `licenses/espflash-Apache-2.0.txt`；版本由 `firmware-helper/Cargo.lock` 固定 |
| serialport 4.10.1，<https://crates.io/crates/serialport/4.10.1> | FirmwareHelper 与 espflash 使用的 macOS 串口枚举和访问库 | MPL-2.0；完整 crate 许可证见 `licenses/serialport-MPL-2.0.txt`；对应精确版本的未修改源码可从 crates.io 获取 |
| Yap，<https://github.com/FrigadeHQ/yap> | SpeechAnalyzer、目标捕获、条件恢复剪贴板、本地润色的架构参考 | 未复制 Yap 源码；依据仓库双语设计文档和已安装 SDK API 实现 |

本机 `mygo-vaultwarden` 仅用于确认 MyGo 公共 API 用法，未复制其 GPL 业务实现。应用不包含 Vokie 图标、名称、品牌音效、固件二进制或专有符号。保留的直接依赖许可证和 serialport 所需的 MPL 声明随 app 资源发布；Rust 精确依赖图由 `Cargo.lock` 固定。Go 工具链和 Apple 系统框架仍遵循各自发行条款。
