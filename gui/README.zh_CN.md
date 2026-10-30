简体中文 · [English](README.md)

# go-voice macOS 应用

`go-voice` 是 Go Voice 的 macOS 26+ Apple Silicon 桌面宿主，基于 MyGo 构建。独立 Go module 为 `github.com/vfasky/my-ai-audio/gui`，正式版 bundle identifier 为 `yz.sh.mygo-go-voice`。本地构建使用 `go-voice-debug.app` 和 `yz.sh.mygo-go-voice.debug`，三个 helper 的标识也追加 `.debug`，以分离开发版与正式版的 TCC 权限记录。

`go-voice` 原生 UI 提供设备与本次语音、词库、语音识别、AI 润色和固件安装五页，以及权限引导页；侧边栏另有“应用”分区，内含“通用”页。界面固定为暗色，遵循 macOS 26 布局：内嵌红绿灯的悬浮 Liquid Glass 侧边栏、位于柔和滚动边缘上的工具栏，以及分组内嵌区块。石墨卡片、黑色内嵌井、白色主胶囊按钮和仅用于实时音频的橙色强调色均参照设计稿；FoloToy AI Passport 声波以中心为最新峰值向两侧镜像展开，空闲时显示暗色圆点；识别、润色和插字时同一面板显示橙色脉冲。扫描时不会自动选择设备，明确选择后的 peripheral UUID 会持久化用于重连。设备页显示固件、握手状态、波形、RMS/peak、时长、实时转写、最终 ASR 原文、润色结果和帧诊断。不保存、不回放音频。

## 构建与验证

需要 Apple Silicon、macOS 26 或更新版本、Go 1.27.3、Rust/Cargo（含 `aarch64-apple-darwin` target）、Bun（可用 `WAKU_BUN_EXECUTABLE` 指定内置运行时），以及包含 SpeechAnalyzer 和 FoundationModels 的 Xcode SDK。Go 依赖版本固定在 `go.mod` 和 `go.sum`；Rust 依赖必须固定在 `firmware-helper/Cargo.lock`。gojieba 需要 CGO 和 Xcode C++ 编译器；`build.sh` 会启用 CGO，并从固定版本模块复制五个词典到 `Contents/Resources/jieba`，安装后的应用不依赖构建机的 Go 模块缓存。先在仓库根目录运行 `./tools/validate.sh --firmware`，生成两个已验证的固件产物。

在仓库根目录启动带签名、自动重构建和重启的调试应用：

```bash
bun run dev
```

该命令调用 `gui/scripts/dev.sh`，不需要 JavaScript 依赖或执行 `bun install`。Go 源码与配置变化时，会重新构建并重启 `go-voice-debug.app`；按 Ctrl+C 停止。Swift/Rust helper 变化需重新运行命令。

也可直接运行构建和验证脚本：

```bash
cd gui
./scripts/validate.sh
# 或仅构建：
./scripts/build.sh
# 带签名的调试会话（watch）：
./scripts/dev.sh
```

`dev.sh` 在会话期间保留固件与原生 helper staging，构建带签名的 debug app，并通过 LaunchServices（`open`）启动，使 TCC 将蓝牙和语音权限归属于 go-voice 本身。Go 源码、`go.mod`/`go.sum`、`mygo.json`、图标或 entitlements 变化时重新构建；构建成功会先退出正在运行的应用（SIGTERM，使 helper 正常关闭）再启动新版本，构建失败则保留当前应用。应用输出写入 `gui/build/dev.log`。退出应用或按 Ctrl+C 结束会话并清理 staging。Swift 或 Rust helper 变更需重新开始会话。不使用 `mygo dev`，因为它把应用作为自身子进程启动，TCC 会把启动终端视为责任进程并终止 BluetoothHelper。`MYGO_SIGN_IDENTITY` 可指定签名身份；未指定时本地构建优先使用已有的 Apple Development 证书，没有可用证书才回退为 ad hoc。稳定的证书可使调试版跨次构建保持相同的 designated requirement；ad-hoc 重构建仍可能需要重新授权。`scripts/stage-debug-project.mjs` 单独生成临时调试配置，`mygo.json` 保留正式版名称和身份；调试版不包含更新配置。

验证脚本运行固件打包 fixture 测试、Cargo 格式/测试/Clippy、gofmt、`go test ./...`、`go vet ./...` 和 Swift 构建/测试，然后构建三个原生 helper 与 MyGo debug app。检查包内固件摘要和 manifest、内外签名及 arm64 可执行文件。结果位于 `gui/build/debug/darwin-arm64/go-voice-debug.app`。应启动 app 包，不使用 `go run`，以正确定位原生资源和 TCC 身份。构建不会启动应用、请求系统权限、连接 FoloToy AI Passport 或访问 USB 设备。

仓库根目录也可运行 `./tools/validate.sh --gui` 执行同一门禁。GUI 验证读取 `build/FoloToy-AI-Passport.bin`，不改变任一根固件产物。构建临时生成 `gui/firmware/go-voice-app.bin` 和 `manifest.json`，将目录打包至 `Contents/Resources/firmware`，退出时清理 staging，构建失败也会清理。源码树不长期保留固件二进制。`FirmwareHelper.app` 位于 `Contents/Resources/native`，正式版身份为 `yz.sh.mygo-go-voice.firmware-helper`（本地构建为 `yz.sh.mygo-go-voice.firmware-helper.debug`）；内层 helper 先于外层 app 签名。

## 固件安装契约

USB 安装只向 `0x10000` 写入包内应用，最大 3 MB，扇区对齐后的擦除终点不超过 `0x310000`。写入前 helper 验证镜像摘要/大小、ESP32-C3 身份、8 MB Flash 与分区表完整性。支持原有五分区模板，以及实测九分区资源布局；两者使用相同的 USB ROM 应用写入流程，不要求存在 Recovery bootloader 日志标记。[helper 契约](firmware-helper/README.zh_CN.md)列出两套配置，未知布局会被拒绝。预检失败在任何擦除/写入前停止，并尝试正常复位。bootloader、分区表、所有资源分区、设备身份（`cardid` 位于 `0x356000`）和永久 Recovery 存储区（`0x700000`）均保持原样。不提供 erase-all 操作。

安装失败时重新连接 USB 后重试；设备无法启动时使用其原厂恢复流程。USB 应用安装保留现有 bootloader 和 Recovery 存储，不验证或修复缺失的 UP 键/Recovery 实现。小程序/社区产物仍为 `FoloToy-AI-Passport-full.bin`，桌面 app-only 镜像不能替代它。已完成及待完成的真机检查见[验收记录](docs/acceptance.zh_CN.md)。

## 运行与隐私

`internal/app` 是唯一组装层。UI 回调只发送意图，后台事件通过 `mygo.RunOnMain` 回到 UI 线程。协调器管理会话代次、取消、早到音频缓存及终态。Go 把独立 ADPCM 帧解码为 16 kHz、有符号 16-bit 小端单声道 PCM。默认由 Swift 子进程直接接收 PCM，使用 SpeechAnalyzer/SpeechTranscriber 的 volatile results 和当前词库，仅在下文的热键录音时创建 Mac 麦克风输入。

在“语音识别”页选择在线引擎后，由应用自身把这些 PCM 按 100 ms 一包，经 WebSocket（`wss://maas-api.antdigital.com/api-ws/v1/inference`，DashScope 双工任务协议：run-task、二进制音频、finish-task）实时发送到支付宝 MaaS 实时模型（`fun-asr-realtime` 或 `paraformer-realtime-v2`）；中间结果到达即显示在胶囊中，task-finished 后得到最终转写。非流式的 `qwen3-asr-flash` 则把本次音频留在内存中，松开按键后封装为一个 WAV（Base64 data URL）发送到 `https://maas-api.antdigital.com/api/v1/services/aigc/multimodal-generation/generation`；说话时不显示中间文字，请求最长等待 45 秒。此时 SpeechBridge 只登记会话和插字目标（`begin`），AI 润色与插字流程与本地识别相同，本次会话也不需要“语音识别”权限。明确选择并保存在线引擎即允许把每次语音的音频发送到支付宝 MaaS；不发送词库，也不保存音频。开始时最多留 4 秒建立连接，期间早到的音频会先缓存；缺少或无效的 API key、key 无权使用该模型、超时或任务失败都会以中文原因结束本次会话，不会自动改用本地识别。页面上的“测试连接”只发送半秒静音。API key 仅存入 macOS 钥匙串（账户 `asr:alipay-maas`）。

火山引擎豆包选项遵循[大模型流式语音识别 API](https://docs.volcengine.com/docs/DoubaoVoice/LargemodelstreamingautomaticspeechrecognitionAPI)（双向流式模式，`wss://openspeech.bytedance.com/api/v3/sauc/bigmodel`）。应用先发送二进制 full client request（PCM、16 kHz、16 位、单声道），再按 100 ms 一包发送音频，最后一包带结束标志并携带 100 ms 静音；带结束标志的回复文本即最终转写，之前的回复只作预览。只提供流式语音识别 2.0。计费方式（小时版或并发版）由下拉框设置的 `X-Api-Resource-Id` 决定，需与该 API key 已开通的方式一致；火山引擎新版语音控制台的 API key 以 `X-Api-Key` 发送，并以独立账户存入钥匙串。不支持旧版 APP ID 加 Access Token 凭据。不发送词库或热词。目前仅用本地协议对端验证，尚未连接真实服务验证。

新增的阿里云百炼 Omni Realtime 选项遵循 [Omni Realtime 交互协议](https://help.aliyun.com/zh/model-studio/omni-realtime-interaction-process)及其[输入转写事件](https://help.aliyun.com/zh/model-studio/server-events)。默认模型为 `qwen3.8-omni-flash-realtime`，页面支持填写其他 Omni Realtime 模型。地址与工作空间绑定：选择地域（华北2（北京）或新加坡）并填写工作空间 ID，应用即连接 `wss://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime` 或 `wss://{WorkspaceId}.ap-southeast-1.maas.aliyuncs.com/api-ws/v1/realtime`；“自定义”可填写其他 WSS 地址。未填写工作空间时没有默认地址。API key 必须对应所选地域和工作空间。地址不能包含凭据、查询参数或片段，模型名会单独添加。每个 Omni 端点使用独立的钥匙串账户，不复用支付宝密钥。收到 `session.created` 后发送 `session.update`，配置 16 kHz PCM、开启输入转写、设置 `turn_detection: null`；等待 `session.updated` 后，通过 `input_audio_buffer.append` 发送 Base64 PCM。松开按键后发送 `input_audio_buffer.commit`。预览事件以 `text + stash` 替换当前句子，只有已提交消息项的最终输入转写才结束听写。不发送 `response.create`，不使用模型回答或生成音频。收尾最多等待 15 秒，取消时即使写入阻塞也会关闭连接。“测试连接”只发送半秒静音。真实服务行为及设备听写仍待验证。

BluetoothHelper 只使用[权威 BLE 协议](../docs/ai-passport-ble-protocol.zh_CN.md)中的 Go Voice UUID。helper 在发送 host_ready 前确认 MTU 和订阅，有响应写重试、断线退避重连及父进程退出清理。两侧进程边界均使用有界 NDJSON 与 canonical Base64。音频缓存只存在内存中。

启动模型准备会检查/下载并保留 zh-CN 资产，再预热分析器。权限在启动时检查、使用前申请，绝不在会话中途弹出：“权限”页将蓝牙、语音识别和辅助功能列为必需，自动化（System Events）和麦克风（用于热键）为可选；逐项显示状态，仍可弹出系统请求时提供“授权”，否则打开对应的“隐私与安全性”面板，并在仍有缺失时每两秒刷新。全部状态上报后若缺少必需权限，应用会自动打开该页，设备页也提供入口。未授予语音识别时开始的会话会立即以该原因结束，而不是报启动超时。

会话期间，听写胶囊条悬浮在指针所在屏幕的底部中央、程序坞上方，在所有桌面空间可见。胶囊左侧为取消按钮（仅在会话期间全局占用 Esc），随后是小声波和一行定宽文字，然后是 AI 润色期间可点击的确认按钮，用于跳过润色直接插入 ASR 原文。说话和识别收尾时，这行文字实时显示识别出的内容：最新的字贴在右侧并平滑滑入，较早的字向左滚出并在左缘渐隐；尚未识别出文字时显示“请说话…”，其他阶段显示会话状态（准备、识别、润色）。AI 润色期间，胶囊上方的预览气泡显示将要插入的 ASR 原文（最多三行）。它是不获取键盘焦点显示的 MyGo 面板，并在任何粘贴前隐藏，确保文字进入你听写的目标应用；会话被取消或失败时显示原因 1.5 秒。

Mac 麦克风热键无需工牌即可录音。在 macOS 任意位置单独按住“通用”页选择的组合键（默认 Shift+Option；也可选 Control+Option、Control+Shift、Command+Option 或关闭）250 毫秒，说话，松开后识别并插入。按住期间按下其他键、鼠标按键或其他修饰键视为快捷键：不足 250 毫秒时不开始录音，之后则取消本次会话。SpeechBridge 用一个原样放行事件的 Core Graphics event tap 监听组合键，已有的辅助功能授权即可覆盖；只在按住期间用 AVAudioEngine 打开默认输入设备，并转换为同样的 16 kHz、16-bit 单声道 PCM（每帧 20 毫秒），因此识别引擎、润色、插字和悬浮胶囊与工牌会话一致（在线引擎同样会收到这段音频）。松开即停止输入；最后一帧到达或松开一秒后结束识别。单次最长 5 分钟，输入连续 3 秒没有音频则结束会话。热键会话期间不会开始工牌会话，工牌断开也不会结束热键会话。麦克风权限为可选项，列在“权限”页；权限未决定时第一次按住只弹出系统请求而不录音，被拒绝时胶囊显示原因。音频不保存。

go-voice 是菜单栏应用：`LSUIElement` 使其不出现在程序坞和应用切换器中，菜单栏模板图标提供“设置…”（Cmd+,，显示主窗口）、“检查更新…”（正式版打开 MyGo 签名更新窗口；调试版说明其不会自动更新）和“退出”。关闭主窗口只会隐藏，应用继续驻留菜单栏。侧边栏底部连接状态卡片右侧的电源按钮也可退出应用，走同样的退出流程；固件安装期间该按钮不可用。“通用”页提供“登录时打开”开关：通过 MyGo（`SMAppService`，系统设置 › 通用 › 登录项）把 go-voice 加入当前用户的登录项，开关始终显示系统报告的状态，在系统设置中关闭也会同步。macOS 要求用户允许新的登录项时，页面会说明并提供“打开登录项”，直到允许为止。登录时启动的 go-voice 只驻留菜单栏、不打开窗口；缺少必需权限时才显示权限页。该设置不写入 `settings.json`。图标 `gui/resources/tray.png` 由透明白色标志经 `"$WAKU_BUN_EXECUTABLE" gui/scripts/generate-tray-icon.mjs`（需要 FFmpeg）生成：按图案裁切，36×36 像素，黑色透明底模板图。包含 Bluetooth、Speech、Apple Events 和麦克风权限文案，应用与 SpeechBridge 为热键录音带有 audio-input entitlement。插字还需要用户授予辅助功能权限。Secure Input、缺少辅助功能授权或目标应用不可用时，最终文本仅留在剪贴板。粘贴优先 System Events，失败使用 CGEvent；0.5 秒后仅在剪贴板归属未变时恢复原内容。

默认使用 Apple FoundationModels 本地润色，提供关闭、专名、轻度（默认）和结构化模式。专名只按词库纠正专有名词；轻度先按词库纠正，再去掉没有意义的语气词（叹词和犹豫声、只用来停顿的“那个”“就是”、说错重来的重复词、因犹豫声被识别出的单个字母，如“Y”），并修正标点，不改写、不调整语序、不添加内容；结构化在此基础上以校对者身份修改文字：结合上下文和常识纠正识别错误（同音字、发音相近的名称、错字、识别走样的英文词），词库和候选之外的词也会纠正；本身是正常词、但放在句中说不通的也会改，例如把“增加豆瓣语音识别的介入”中的“介入”改为“接入”（关闭思考的快速模型仍可能保留被听成另一个知名产品的名称，如“豆瓣”，除非正确的名称作为词库词进入提示）；说话人改口时只保留改正后的说法；以最小改动理顺语法和断句，再按内容分段或列点，不概括、不删减、不添加事实。忠实度检查不把去掉的语气词算作丢失内容；结构化模式允许更多改动（至少保留 55% 原文字符，新增不超过五分之二），但仍拒绝摘要、答复和编造内容。设置 `GOVOICE_REFINE_LIVE=1`、`GOVOICE_REFINE_KEY`、`GOVOICE_REFINE_ENDPOINT`、`GOVOICE_REFINE_MODEL`（以及可选的 `GOVOICE_REFINE_API=responses`）后运行 `go test ./internal/refinement -run TestLiveStructuredCorrections`，会用真实模型检查这类纠正，以及同样的词在说得通时是否保留；每次运行都会计费。无论本地还是远端，所有启用的模式都会优先按本次词库纠正专有名词（同音字、音译、大小写和空格），但不会加入没说过的词。调用模型前，应用先比对拼音（合并 z/zh、c/ch、s/sh、n/l 和 -n/-ng），提供候选片段和 gojieba 本地分词。模型比较原文和所有竞争词，普通词适合上下文或意图不明确时保留原文，不因词条存在于词库就优先替换。完整词候选排在切断词语的候选之前；后者仍作为提示保留，但需要更强的上下文支持，已经正确写出的词条受保护。关闭 Jev 时也生效。本次词表中的中文词会加入本地分词词典；词集合变化时重新建立词典，删除或改名的条目不会残留到后续调用。远端请求会要求关闭思考模式（`reasoning_effort`/`reasoning.effort: none`、`thinking: disabled`、`enable_thinking: false` 和对话模板参数）；服务商以 HTTP 400/422 拒绝这些参数时自动减少参数重试，并按端点和模型记住可接受的级别；回复中的 `<think>` 内容会被去掉；忠实度检查把这类替换视为纠正，并设有上限，词库词不能替代原本的听写内容。失败、超时、空输出或字符忠实度过低都保留最终 ASR 原文。本地失败不会调用远端。明确选择并保存 `remote` 后，才允许把最终转写和当前词库发往配置的 HTTPS OpenAI-compatible 端点；音频不发送。设置页将其标注为 OpenAI 兼容接口，并提供 Chat Completions（默认）和 Responses 两种接口格式。可选的“流式输出”开关以流式（SSE）请求并收齐全文，结果仍在校验后整体插入，并不会更快，只用于只接受流式请求的服务或模型；流式中途出错或在最后一个事件前中断时，整条回复按失败丢弃。可选的“供应商路由”（仅允许、优先顺序、排除三个列表，以及不允许列表之外供应商的开关）会作为顶层 provider 对象发送，用于同一模型有多个供应商的网关，如蚂蚁数科 DTMaaS；全部留空则不发送。填写例如 `https://provider.example/v1` 的基础端点和模型名，应用按所选格式追加 `/chat/completions` 或 `/responses`。Responses 请求设置 `store: false`，且只接受恰好一个已完成的 `message` 输出。“测试连接”在保存前按页面当前填写的内容检查接口，优先使用输入框中的 API key，否则使用已保存的 key：即使当前选择本地引擎，也只发送一段固定示例句，从不发送听写内容或词库；成功时显示润色后的示例，失败时给出可能原因（缺少端点/模型/key、HTTP 401/403/404/429/5xx、超时，或返回了另一种接口格式）。“保存设置”会把输入的 API key 一并按端点存入钥匙串；“测试连接”“保存设置”“保存到钥匙串”“删除凭据”的结果都显示在按钮下方，表单再次修改后自动隐藏。API key 仅通过 Security.framework 存入 macOS Keychain，不写入设置、日志、测试夹具或 `mygo.json`。“AI 润色”“语音识别”和“Jev 判断”页的 API key 输入框为空时，会提示钥匙串里是否已有当前端点或服务商的 key（只查是否存在，不读取内容）；留空即沿用已保存的 key。识别引擎下拉框每个服务商只列一项；服务商的模型（支付宝 MaaS 的 Fun-ASR、Paraformer 和 Qwen3-ASR-Flash，豆包的小时版和并发版）在其卡片中选择，共用同一个 API key 输入框，切换模型不用重新填写 key。已输入的 key 不会带到其他服务商，切回时恢复各服务商已输入的 key、所选模型和百炼地址草稿。

设置和词库以受限权限保存在 `~/Library/Application Support/MyGo-GoVoice/`。词库保留来源、权重、锁定、创建和使用时间，按标准化术语去重，每次会话最多注入 100 个 contextual strings：先选择排序后的显式词条，再用去重后的替换目标补齐剩余名额。Top 100 只用于 Apple 本地识别提示。纠错在会话开始时固定完整词库与去重后的替换目标，在后台进行本地检索。命中的中文候选、字面命中和 Top 100 中的非纯中文提示词（用于音译、大小写及空格纠正）供 Jev 和润色使用，不发送完整词库。检索结果超过 256 个词、4,096 处候选或转写超过 8,000 字时，跳过两种模型处理，保留本地显式纠正结果，不截断竞争候选；润色提示超过 128 KiB 时也跳过。不注入错误写法，也不会把派生目标另存为词条。词库页分为“词条”“替换词”“Jev 判断”和“公共词库”四个标签页。“替换词”页的每条规则由两个输入框填写：原词（识别出的错误写法，例如“施玉宝”）和替换词（正确写法，例如“私域宝”）。实时和最终识别结果会在本地直接替换，不经过 AI，并先于胶囊预览、插入和润色生效，较长的原词优先；某条规则替换出的结果还会继续被其他规则修正，AI 润色结果也会再替换一次；英文原词不区分大小写，只匹配完整单词。原词不能是词库中的词条，同一原词只能有一条规则。最终纠正还会按完整词边界匹配被水平空白拆开的中文原词，保留标点和换行、不切断已有词语，只应用已配置的映射，不自行推测新的同音替换。这一步使用词库快照在后台运行，关闭 Jev 和 AI 润色时也生效。字面规则优先，实时预览仍按字面规则替换；超过 8,000 字或词典不可用时跳过额外分词，保留字面纠正。早前版本登记在词条里的替换词会在读取时自动迁移成规则。“导出”把词条（含锁定和权重）和替换词写入一个 JSON 文件（`"format": "go-voice-vocabulary"`），不含使用记录；“导入”合并这样的文件或应用自己的 `vocabulary.json`：新增没有的条目，更新已有条目的锁定、权重和替换目标，不删除任何内容，跳过无效或冲突的条目并报告数量。其他格式的文件不会改动词库。“Jev 判断”页可开启 TypeSafe 的 Jev 模型判断同音词替换（默认关闭）：每次语音结束后，先用 gojieba 本地分词提供词边界，再由 Jev 结合完整转写，对每处重叠的候选词与保留原文进行一道 Choice 选择题。词边界作为线索而非硬性过滤，保留被切碎或与邻字合并的陌生专名。只匹配两个字以上中文词条的同音或近音，已经正确写出的词库词受保护。候选词的选择概率达到 70% 才直接替换；保留原文的概率达到 70% 则要求 AI 润色不要改，拿不准的交给润色。关闭 AI 润色时也生效。接口地址和模型可以修改：默认 TypeSafe 官方 `https://api.typesafe.ai/v1/systemone` 和 `jev-latest`，另有“支付宝 MaaS”预设（`https://maas-api.antdigital.com/v1/systemone`、`jev-1.13`），也可以填写其他兼容 System One 接口的服务；只填域名时自动补上 `/v1/systemone`，地址必须是 https 且不含账号、查询参数，否则不保存。请求包含完整转写、本地分词结果和命中的候选词，不发送音频和无关词库；没有命中时不发送请求。每次最多判断 24 处，每处最多 63 个候选加保留原文；候选过多的整组留给润色，不截掉部分选项。文字超过 8,000 个 Unicode 字符或请求超过 128 KiB 时跳过，不截断上下文。超过 4 秒或失败时跳过这一步并在状态栏说明原因。“测试连接”按页面上填写的地址和模型只发送固定示例句，并显示每处的概率和处理结果。API key 仅存入 macOS 钥匙串，按接口地址分别保存（TypeSafe 官方为账户 `vocabulary:typesafe-jev`，其他为 `vocabulary:jev:<接口地址>`）。转写仅显示本次会话，不持久保存音频或转写历史。

“公共词库”内置 96,336 词的 Jieba 通用库和 12,854 词的 THUOCL IT 包，默认全部开启，读取旧设置时也默认开启。开关保存到设置，下次会话生效。它们是只读的基础数据，独立于个人词条、导入导出、个人 2,000 词上限和 Top 100 识别提示。公共候选仅在完整词边界进行无声调的同音匹配，保留来源与来源内频率排序，不把普通词作为个人专名保护。后台索引缓存后，每处最多召回八个公共候选，每次转写最多 96 个；个人候选在润色提示和 Jev 问题名额中保持优先。命中词条不会触发本地自动替换，需要开启 AI 润色或 Jev 中至少一项。内置数据加载不联网；只有命中候选会进入明确启用的远端模型。固定来源、可复现编译与当前限制见[词库评测](docs/vocabulary-evaluation.zh_CN.md)。

## 发布与更新

正式版使用团队 Developer ID 和 hardened runtime 签名，经过公证并 staple，并从已签名的 `go-voice-v*` GitHub release 自动更新，每天检查一次。`./scripts/release.sh` 负责测试、构建（`./scripts/build.sh --release`）、公证、验证并上传草稿 release；公开草稿后更新才会推送。密钥、准备工作、发布流程与更新验收见[更新、签名与公证](docs/updates.zh_CN.md)。

## 文档

- [词库评测](docs/vocabulary-evaluation.zh_CN.md)：完整词库检索、手写回归样例、按需启用的模型测试和性能测量。
- [实现与验收记录](docs/acceptance.zh_CN.md)
- [更新、签名与公证](docs/updates.zh_CN.md)与[发布说明](docs/CHANGELOG.zh_CN.md)
- [第三方声明](docs/THIRD_PARTY_NOTICES.zh_CN.md)
- [根架构文档](../docs/passport-audio-input-architecture.zh_CN.md)
- [根目录契约](../docs/project-structure.zh_CN.md)

构建与主机单测成功不代表真实设备或 TCC 验收通过。剩余检查详见验收记录。
