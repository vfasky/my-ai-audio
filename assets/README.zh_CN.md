<p align="right">
  <strong>简体中文</strong> · <a href="README.md">English</a>
</p>

# 资源目录（Assets）

## Go Voice 标志与标题

- 来源：[brand/yz-logo-source.png](brand/yz-logo-source.png)（伊智科技 LOGO，519×143，白色透明底）。仅取左侧圆形标志，派生出 [brand/go-voice-logo.png](brand/go-voice-logo.png)（1024×1024 透明）、桌面图标 `gui/resources/icon.png`（黑色圆角底板）、菜单栏图标和官网图标，并转换为固件 64×64 RGB565 头文件 [go_voice_logo_asset.h](brand/go_voice_logo_asset.h)。
- 使用 `"$WAKU_BUN_EXECUTABLE" tools/generate-go-voice-logo.mjs` 生成（需要 FFmpeg），随后运行 `gui/scripts/generate-tray-icon.mjs` 生成菜单栏图标。
- `main/ui_status.c` 通过 `main/CMakeLists.txt` 的品牌目录路径引用头文件。像素常量为 8192 字节，与旧图相同，无运行时解码或缩放缓冲区；高分辨率源图不嵌入固件。
- 标题使用已启用的 LVGL Montserrat 20 字体，包含全部 `Go Voice` 字符。标题位置、宽度、对齐和裁切不变，不新增字体或堆分配。
- 按用户要求制作的品牌素材用于本项目，本说明不主张独立再分发授权。见[第三方声明](../docs/THIRD_PARTY_NOTICES.zh_CN.md)。

## Vokie 开机音

- 来源：Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8` 中的参考 `v2-blip-octave-tail.wav`，保留为 [vokie-boot-v2-blip-octave-tail.wav](music/vokie-boot-v2-blip-octave-tail.wav)。上游称创作者允许其固件使用，但未提供独立音效许可证；发布含该音效的 Go Voice 源码或固件前，须确认独立授权。
- 原始格式：2 秒、44.1 kHz、有符号 16 位单声道 WAV。
- 固件素材：[vokie-boot-16k.pcm](music/vokie-boot-16k.pcm)，16 kHz、有符号 16 位小端单声道 PCM，共 64,000 字节；不做响度归一化或变速。
- 集成：`main/CMakeLists.txt` 仅将 PCM 嵌入闪存。`main/boot_sound.c` 使用 640 字节栈缓冲区，在已有音频任务中以 55/100 输出音量每次开机播放一次；收到录音请求时让位，补零排空播放队列、静音并丢弃开机期间的麦克风采样。保持录音格式及麦克风增益，播放失败仅记录日志，不阻止语音输入。

在仓库根目录重新生成：

```bash
ffmpeg -nostdin -i assets/music/vokie-boot-v2-blip-octave-tail.wav \
  -ar 16000 -ac 1 -c:a pcm_s16le -f s16le \
  assets/music/vokie-boot-16k.pcm
```

本目录集中存放可复用的资源（字库、图片、音乐等），按资源类型分子目录管理。每个资源放在其类型对应的子目录，并记录放置路径、命名方式、集成方式与来源/许可。二进制资源（字体、图片、音频）不属于纯 markdown 文档，请勿与文档混放。涉及版权/授权的资源需注明来源与许可。

## 字库（fonts）

可复用的字库文件与生成的字库源码放在 `fonts/`。

- 命名要能反映字族、字重、字级与格式。
- 记录来源、许可、字符范围、转换命令与目标放置路径。
- 添加字库前评估 Flash 与内部 RAM 影响；ESP32-C3 无 PSRAM。
- 不提交许可不允许分发的字库。

## 硬件购买图片

- 来源：用户提供的 FoloToy 微信小店商品分享图，原图保留为 [images/ai-passport-purchase.jpg](images/ai-passport-purchase.jpg)（1020 × 2208 JPEG）。
- 网站副本：`website/public/img/ai-passport-purchase.jpg`，逐字节复制，保留完整小程序码。硬件购买区链接至完整大图和[官方产品介绍](https://ai-passport.folotoy.cn/)。
- 图片由用户提供用于本网站，本说明不主张独立再分发授权。图中价格和优惠是截图信息，并非实时报价；网站引导用户以店铺实时信息为准。

## 图片（images）

可复用的源图与生成的显示资产放在 `images/`。

- 使用描述性命名，并记录尺寸、像素格式、转换步骤与目标路径。
- 优先采用适合 240 × 320 RGB565 显示的格式，并纳入 Flash 与内部 RAM 考量。
- 许可允许时保留可编辑源文件，并记录来源与许可。
- 图片中不得包含设备二维码秘密、凭证或个人数据。

## 音乐与音效（music）

可复用的音乐与音效源码放在 `music/`。

- 记录来源、许可、采样率、位深、声道、转换命令与目标路径。
- 与当前 BSP 音频路径匹配时优先采用 16 kHz、16 位单声道 PCM。
- 嵌入音频前评估 Flash 与内部 RAM 成本；长录音应流式或分块。
- 无再分发许可不提交媒体文件。

## 历史 Vokie 标题字体

未使用的 Barlow Condensed Bold 源字库与生成的 LVGL 子集已移除。Go Voice 使用 LVGL Montserrat 20，继续保留历史 [Barlow OFL 声明](../LICENSES/Barlow-OFL.txt)以记录来源。
