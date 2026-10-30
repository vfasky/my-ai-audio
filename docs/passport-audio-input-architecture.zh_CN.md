简体中文 · [English](passport-audio-input-architecture.md)

# Go Voice 系统架构

AI Passport 是唯一音频采集端，通过 BLE 将麦克风音频连续送到 macOS 应用，由 Mac 识别、按需润色并插入文字。只有按住可选的按住说话热键时，Mac 才开启自身麦克风并送入同一流程；音频与转录不保存为历史记录。

```text
AI Passport / ESP32-C3
  ES8311 → I2S PCM → 独立 IMA-ADPCM 帧 → BLE
Mac / go-voice
  Swift CoreBluetooth helper → Go 校验、重组与解码
    → Swift SpeechAnalyzer / SpeechTranscriber + 当前词库
    → 可选本地 FoundationModels 或用户明确选择的远端润色
    → TextInjector → 前台应用输入框
```

固件基于 Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8`，桌面端插字设计参考 [Yap](https://github.com/FrigadeHQ/yap)。来源归属与再分发要求保留在[固件声明](THIRD_PARTY_NOTICES.zh_CN.md)和[桌面端声明](../gui/docs/THIRD_PARTY_NOTICES.zh_CN.md)中。

## 音频与协议

工牌采样格式为 16 kHz、有符号 16-bit、单声道 PCM。每个 20 ms 帧包含 320 个样本，独立编码为 166 字节 ADPCM；加上 16 字节 BLE 包头后，特征值共 182 字节，要求 ATT MTU 不小于 185。每帧使用独立预测器，丢失一帧不会污染后续帧。

[BLE V1 协议](ai-passport-ble-protocol.zh_CN.md)定义 Go Voice UUID、握手、消息、字节布局与限制，是唯一线协议契约。固件不运行 ASR，也不存储整段录音；15,000 帧、五分钟的会话上限由主机执行。

## 会话生命周期

1. 用户明确选择设备。CoreBluetooth 连接后检查载荷容量、完成订阅、读取设备信息，并完成 `hello` / `host_ready` 握手。
2. UP 开始采集。收到 `ptt_down` 后保存前台应用、创建主机会话代次，并带当前词库启动 Speech；Speech 启动期间最多在内存中缓存 75 帧音频。
3. Go 校验控制与音频消息，重组分片，解码 ADPCM 并排序 PCM。缺帧等待 80 ms，每次会话最多以静音补偿 100 帧；音频停滞三秒则取消会话。
4. UP 停止采集。`ptt_up(finalSequence)` 触发最长 500 ms 的尾帧等待，随后完成识别。
5. 主机按配置润色最终 ASR 文本，插入文字或保留在剪贴板，并向工牌报告终态。

DOWN 发送回车。OK 在闲置时删除，活动请求中取消；长按 650 ms 在闲置时清空，活动请求中取消。断连、设备错误、取消和过期原生结果不得向后续会话插字。每个原生请求使用独立于 BLE session ID 的主机会话代次。

## 原生 helper 与职责归属

`gui/internal/app` 组合服务，设备协调器持有会话状态与取消权；纯 Go 协议层负责校验、编解码与排序。UI 回调发出操作意图，后台更新通过 `mygo.RunOnMain` 返回。

BluetoothHelper 和 SpeechBridge 是签名的嵌套子进程应用，使用有界 NDJSON 与标准 Base64；父进程退出或管道关闭后停止。SpeechBridge 封装 SpeechAnalyzer/SpeechTranscriber、模型准备、FoundationModels 与 TextInjector；音频缓冲只存在于内存中。目录对应关系见[源码布局](project-structure.zh_CN.md)。

固件硬件访问留在 BSP。音频任务独占 I2S 与 codec 生命周期，UI 访问遵循 LVGL 锁规则，按键回调保持非阻塞。[待机契约](development/engineering/standby-power-contract.zh_CN.md)说明音频暂停、显示暂停与 USB 睡眠锁行为。

## 词库、润色与插字

词库存储词条、来源、权重和用户锁定状态。每次语音会话最多注入 100 个识别提示词，并另行固定完整纠错词库。识别结束后，本地拼音索引检索命中的词条，只有这些候选、字面命中和非纯中文识别提示词进入润色。正确写出的个人词条受保护；个人候选过多时跳过模型纠正，不截断候选。独立的只读通用库和 IT 包默认开启，仅提供按频率排序且数量受限的建议，不获得个人专名保护，也不占用 Top 100。预编译读音及索引缓存保留在本机，设置修改在下一次会话生效。AI 建议不得静默替换或删除已锁定词条。系统词库提示用于提高识别概率，不保证强制替换；不会每句结束就重新编译语言模型。

Apple FoundationModels 是默认润色引擎。失败、超时、空输出或保真度不足时保留 ASR 原文。远端处理必须显式配置，只向所选 HTTPS 接口发送最终转录与当前词库，不发送音频；本地失败不会静默切换远端。API key 仅存 macOS Keychain。

TextInjector 在会话开始时捕获前台目标，检查辅助功能与 Secure Input，等待修饰键松开，经 System Events 粘贴并以 CGEvent 兜底；仅在剪贴板所有权未变时，于 0.5 秒后恢复原剪贴板。缺少权限、目标不可用或 Secure Input 开启时，结果仅留在剪贴板。HUD 在插字前隐藏且不抢键盘焦点。

## USB 安装与保护存储

独立 Rust FirmwareHelper 使用固定版本 `espflash`，仅向 `0x10000` 写入随应用提供的镜像，最大 3 MB。任何擦写前，预检验证镜像摘要、芯片、Flash 容量，并精确匹配两种设备布局之一：原有五分区模板，或实测九分区资源布局。两者使用相同的 USB ROM 应用写入流程，不依赖现有 bootloader 的 Recovery 标记。随包 manifest 保持模板契约，绝不替换设备分区表；扇区擦除终点不超过 `0x310000`。不提供整片擦除、bootloader 或分区表写入路径。布局细节见 [helper 契约](../gui/firmware-helper/README.zh_CN.md)。

必须保留 `0x356000` 的 `cardid`、`0x700000` 的永久 Recovery，以及上电长按 UP 五秒的 bootloader hook。桌面端应用镜像与小程序合并镜像用于不同安装流程，详见 [Recovery 兼容规范](development/engineering/ble-recovery-compatibility.zh_CN.md)。

## 隐私与验收

BLE V1 不提供配对、对端认证或应用层加密。持久化的 CoreBluetooth peripheral UUID 用于选择和重连，不能证明设备身份；应用明确展示此限制。认证传输仍属后续工作。

完整流程需要蓝牙、语音识别和辅助功能权限；自动化为可选，插字可由 CGEvent 完成。Mac 麦克风权限为可选项，只为热键申请。设置和词库在本地持久化，会话音频和转录不持久化。

构建、host tests 和实机结果分别报告。真实识别质量、持续 BLE 传输、权限、插字、功耗和 Recovery 仍须完成[桌面端验收记录](../gui/docs/acceptance.zh_CN.md)与[固件记录](go-voice-firmware.zh_CN.md#验证)中的检查。
