简体中文 · [English](go-voice-firmware.md)

# Go Voice 固件

## 来源与范围

固件基于 Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8`，导入源码归档的 SHA-256 为 `f78a1349ec3a02c0fe96a9233946ee0d78e0bb289d8e5cbe02b12a7c5ef88575`。保留 FoloToy MIT 归属、依赖锁、ESP-IDF 5.5.3 结构、BSP、bootloader hook 和分区布局。内部 `vokie_` 标识符保留导入实现的结构，对外标识为 Go Voice。

工牌采集音频并发送给 [macOS 应用](../gui/README.zh_CN.md)，ASR、词库、润色和插字由 Mac 完成。当前固件不实现 Wi-Fi 音频、V2 认证传输或 OTA。

## 标识与协议

| 端点 | UUID |
| --- | --- |
| Service | `676f0001-766f-6963-652d-617564696f00` |
| Control | `676f0002-766f-6963-652d-617564696f00` |
| Audio | `676f0003-766f-6963-652d-617564696f00` |
| DeviceInfo | `676f0004-766f-6963-652d-617564696f00` |

广播、标题、就绪和等待文案使用 `Go Voice`，`hello.device` 仍为 `ai-passport`。NimBLE UUID 数组使用逆序字节，host tests 将其与标准 UUID 及各使用方比较。[BLE V1 协议](ai-passport-ble-protocol.zh_CN.md)定义线协议契约。

UP 切换采集，DOWN 发送回车，OK 删除或取消，长按清空或取消。音频采用独立 20 ms / 320 样本 IMA-ADPCM 帧、166 编码字节和不小于 185 的 ATT MTU。15,000 帧、五分钟的会话上限由 Mac 执行。[待机契约](development/engineering/standby-power-contract.zh_CN.md)定义音频/显示暂停、背光与 USB 功耗行为。

本仓库不再包含独立 Vokie 插件。桌面应用自行构建原生 helper，并继续保留上游归属和 MIT 声明。使用旧服务 UUID 的未修改 Vokie 客户端无法连接。

## 显示与资源

当前 [Logo](../assets/brand/go-voice-logo.png) 是透明底白色 Gopher 与右下角麦克风。桌面图标将相同图稿放在黑色圆角底上；固件将透明区域叠合到既有 `#0B1018` 屏幕背景。原白色保持白色，原黑色区域变为透明，包括面部镂空。

`tools/generate-go-voice-logo.mjs` 使用保留的[原始图稿](../assets/brand/yz-logo-source.png)，生成桌面资源和[固件头文件](../assets/brand/go_voice_logo_asset.h)，步骤见[资源生成说明](../assets/README.zh_CN.md)。`main/ui_status.c` 嵌入恰好 8192 字节的 64×64 RGB565 常量，标题使用 LVGL Montserrat 20；不引入运行时图像解码器、缩放缓冲或额外字体分配。重新生成图片头文件后须重新构建固件。

## 保护存储与许可

应用起始地址为 `0x10000`，上限 `0x300000` 字节；必须保留 `0x356000` 的 `cardid`、`0x700000` 的永久 Recovery，以及上电长按 UP 五秒的 bootloader hook。遵守 [Recovery 兼容规范](development/engineering/ble-recovery-compatibility.zh_CN.md)，最终合并产物及已链接 hook 只有通过固件门禁才能确认。

参考开机音 WAV 与 PCM 保持不变。上游称创作者允许其固件使用，但未提供独立音效许可证；分发包含该音效的 Go Voice 源码或二进制前，须取得独立授权。保留 FoloToy MIT、Barlow OFL 和[第三方声明](THIRD_PARTY_NOTICES.zh_CN.md)。Go Voice 图稿不构成 Vokie 商标授权。

## 验证

- `python3 tests/test_go_voice_contract.py`：UUID 字节序及使用方、Logo 摘要、存储与 LVGL 接线。
- `./tools/validate.sh --static`：仓库检查、workflow lint 和 host tests，包含双语文档与本地链接检查。
- `./tools/validate.sh --firmware`：ESP-IDF 5.5.3 构建、合并镜像验证与受保护分区检查。
- `./tools/validate.sh`：固件与桌面端完整门禁，需要两套工具链。

### 当前透明白色图稿的验证记录

以下为已测试产物的历史结果，不能代替新发布构建的验证。

- Build：PASS。使用 ESP-IDF v5.5.3 与 macOS 主机检查，完整门禁通过；合并镜像通过小程序与受保护分区校验，应用大小为 1,161,296 字节，低于 3 MB。
- 合并产物：`build/FoloToy-AI-Passport-full.bin`，1,226,832 字节，SHA-256 为 `a5da135353422d6fcf6b26d7c8cd51488475f27582c71018d934efa40a52d227`。
- 图稿：PASS。PNG SHA-256 为 `6b946925c2475161c1fbb2f5c181bfc3e617a2a6580078066d1b34404d25f544`，RGB 通道全部为白色；8192 字节图片在合并镜像偏移 `0x26088` 处出现一次。
- Host tests：PASS。仓库、C/Python、Go/Swift、桌面端构建与签名检查通过。
- Device tests：仅刷写、图片读回与启动 PASS。ESP32-C3/8 MB Flash 设备在分段写入前分区表一致，启动程序、分区表和应用均通过设备摘要检查；擦写范围避开 NVS、PHY、`cardid` 和 Recovery。自 `0x26088` 读回的全部图片字节一致，复位后观察 15 秒，出现一次启动，显示/LVGL/音频初始化及 Go Voice 广播成功，无错误或 panic 日志。
- Unverified：实体显示效果、所有按键、端到端 BLE 语音输入、持续采集与重连、功耗测量和实体 Recovery 入口。仓库不保存设备标识或原始日志。
