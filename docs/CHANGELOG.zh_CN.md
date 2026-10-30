<p align="right">
  <strong>简体中文</strong> · <a href="CHANGELOG.md">English</a>
</p>

# Changelog

## Unreleased

- go-voice 内置默认词库：16 个词条和 15 条替换词（公司与产品名、工作用词及其常见误识别写法）。首次运行且没有 `vocabulary.json` 时以它们为起点；已有词库文件不受影响。仅通过主机测试验证。

- 发布 go-voice 0.7.0，包含已实机验证的 USB 应用安装器、可选 Mac 麦克风热键及升级后的更新器；同步网站下载链接与发布说明，并说明可选麦克风流程及其授权。

- 修复受支持五分区设备的 USB 固件安装被 `RECOVERY_MARKER_MISSING` 拦截的问题。桌面安装器现对两种受支持布局使用相同的有界 ROM 应用写入流程，不再要求 bootloader 日志标记；保留芯片、安全状态、Flash 容量、镜像摘要与分区检查、设备端校验及复位，并保留设备现有的 bootloader、身份、资源和 Recovery 存储。小程序合并固件的 Recovery 契约继续强制执行。

- 为 go-voice 增加按住说话热键：在 macOS 任意位置单独按住 Shift+Option 即用 Mac 麦克风录音，松开后按所选引擎识别、润色并插入，同样显示悬浮胶囊。“通用”页可改用其他组合键或关闭；按住期间按下其他键会取消，相同修饰键的快捷键不受影响。麦克风为“权限”页中的可选权限，首次使用时申请。仅通过主机与 Swift 测试验证；Mac 上的实际录音和 event tap 仍待验证。

- 将 go-voice 的 MyGo 从 0.5.5 升级至 0.7.3，最低 Go 版本提高至 1.27.3；发布打包适配新的 DMG 文件名。更新器在每次启动约 10 秒后检查，之后每天检查一次，暂存已校验的更新并在应用退出后替换，通过 LaunchServices 重新启动。

- 发布 go-voice 0.6.2：USB 安装同时支持原有五分区模板与实测九分区资源布局；同步网站下载按钮、备用版本标签与发布数据。

- 将网站下载按钮、备用版本标签与发布数据同步到 go-voice 0.6.1，提供 macOS 26 兼容性修复版本。

- 在 go-voice 0.6.1 中修复 macOS 26 构建兼容性：把 C/C++ 编译及链接固定到声明的最低系统版本，避免沿用构建机较新 SDK 的默认值。检查主程序与三个 helper 的实际 arm64 Mach-O 最低版本，包括更新归档中的副本。macOS 26 真机上的安装和听写仍需验收。

- 准备 `go-voice` 0.6.0：豆包流式识别、校对式结构化润色、已保存 key 提示、润色的供应商路由与流式输出，以及 Omni 地域设置；更新网站识别文案与 0.6.0 下载链接。

- go-voice 的 AI 润色页为 OpenAI 兼容端点新增两个选项。供应商路由把仅允许、优先顺序、排除三个列表（可选地加上 allow_fallbacks 为 false）作为顶层 provider 对象发送，供蚂蚁数科 DTMaaS 等网关在同一模型的多个供应商之间选择，列表全空时不发送。流式输出开关对 Chat Completions 和 Responses 都以 SSE 请求并收齐全文，用于只接受流式请求的服务或模型；结果仍在校验后整体插入，所以不会更快。流式已用 DeepSeek 两种接口实测；供应商路由只有主机测试，没有对真实 DTMaaS 验证。
- go-voice 会提示 API key 已保存：“AI 润色”“语音识别”和“Jev 判断”页的 key 输入框为空时，显示钥匙串中已有 key、留空即沿用。识别引擎下拉框改为每个服务商一项，模型在服务商卡片中选择并共用一个 API key 输入框，在支付宝 MaaS 或豆包的模型之间切换不用重新填写 key；切换服务商后再切回，会恢复各自已输入的 key、所选模型和百炼地址草稿。右上角状态和设备页改为显示当前在线识别服务商，不再一律写支付宝 MaaS。

- `go-voice` 的阿里云百炼 Omni Realtime 默认模型改为 `qwen3.8-omni-flash-realtime`，并把自由填写的端点改为“地域（北京或新加坡）加工作空间 ID”，同时显示拼出的连接地址；“自定义”仍可填写完整 WSS 地址。全球 DashScope 地址不再作为默认值，因此已使用 Omni 的用户需要填写一次工作空间 ID，已保存的模型名保持不变。真实服务行为仍待验证。

- `go-voice` 新增火山引擎豆包流式语音识别（双向流式二进制 WebSocket 协议，`X-Api-Key` 鉴权），提供两个资源选项：流式识别 2.0 的小时版和并发版。说话时显示中间文字，以带结束标志的回复文本作为最终转写；API key 使用独立的钥匙串账户，测试连接只发送静音，失败时结束本次语音且不切换引擎。选择后每次语音的音频会发送到火山引擎，不含词库。目前仅用本地协议对端验证，真实服务尚未验证。

- go-voice 结构化润色获得校对者级别的修改权限：结合上下文和常识纠正词库及候选之外的识别错误，包括本身是正常词但在句中说不通的情况（例如在谈语音识别接入的句子里把“介入”改为“接入”；被听成另一个知名产品的名称仍可能保留），说话人改口时只保留最终说法，理顺病句后再分段或列点。忠实度检查相应放宽，但仍拒绝摘要、答复和编造内容；专名和轻度模式不变。真实模型效果仍待验证。

- 准备发布 `go-voice` 0.5.0：默认启用通用与 IT 公共词库，支持完整个人词库检索。官网新增独立词库介绍，说明词数、来源、开关和隐私行为，并更新下载链接及发布说明。

- 内置只读通用词库（96,336 个 Jieba 词条）与 IT 行业包（12,854 个 THUOCL 词条），默认全部开启。在“词库 > 公共词库”管理，下次语音生效。预编译读音在后台加载并缓存索引。公共候选保留来源与频率依据，不成为受保护的个人专名，不占用识别提示名额，也不覆盖个人词库。附带固定来源记录及 MIT 署名。真实模型纠错效果仍待验证。

- 分离 go-voice 的识别提示与纠错词库：Apple 识别仍使用 Top 100，本地拼音检索使用全部词条和替换目标。保留完整竞争候选、正确专名和当前会话的词库快照；上下文不足以支持纠正时保留原文。新增 40 条手写回归样例、分别报告纠正与误改的可选在线评测及检索性能基准。真实纠错效果尚未验证。

- 准备发布 `go-voice` 0.4.0，包含固件版本提醒、本地词边界纠正、改进的 Jev 判断和更低待机功耗；同步更新网站介绍与 0.4.0 下载链接。

- 固件安装页显示设备与随附固件版本；当已连接设备低于应用随附版本时给出更新提醒，并在侧栏「固件安装」上显示「更新」。设备固件现为 0.2.0，以便与 0.1.0 区分。

- 将 gojieba 扩展到 `go-voice` 的词库与替换流程：词条按锁定和权重排序后，去重后的替换目标补齐 Top 100；其中的中文词作为 Jev 和普通润色共用的本地分词词典。关闭 Jev 时，润色也使用词边界线索，并保护已正确写出的较长词条。最终纠正还会按完整词边界匹配被水平空白拆开的中文原词，不跨标点和换行；关闭 AI 润色和 Jev 时也能生效，实时预览继续按原有字面规则替换。词典更新和最终纠正在后台使用词库快照处理。

- 改进 `go-voice` 的 Jev 词库判断：先用 gojieba 本地分词，再结合完整识别文字，将每一处重叠的同音候选和保留原文一起比较；只有该选项概率达到 70% 才直接替换或要求润色保留原文。词边界作为线索，保留被切碎的陌生专名候选。请求现在会向所选服务发送完整转写、分词结果和命中的候选词，不发送音频和无关词库；文字或请求过长时跳过，不截断上下文。安装包附带固定版本词典，并启用 CGO；真实模型效果仍待验证。

- 降低固件待机功耗：熄屏后 ADC 三键扫描从 20 ms 放慢到 50 ms；麦克风空闲满 5 秒后彻底关闭 ES8311 模拟通路；背光关闭时 ST7789 进入休眠（DISPOFF/SLPIN）。熄屏后的第一次按键最多多约 50 ms，codec 休眠后的第一次说话可能带上模拟启动瞬态，亮屏前需等待 120 ms 的 SLPOUT。实际电流收益仍须真机测量。

- 网站新增 AI Passport 硬件介绍与购买区，展示微信小店购买图片并支持查看完整大图，链接至官方产品介绍；应用下载区增加购买入口，价格以店铺实时信息为准。

- 移除不再使用的独立 Vokie 插件及其包完整性检查。Go Voice 继续使用自有原生 helper 和协议测试。README 新增上游致谢，并保留所有适用的许可证声明。

- 准备发布 `go-voice` 0.3.0，包含 Omni Realtime WebSocket 听写、独立调试身份和 Bun watch 入口；同步更新网站的识别功能介绍与 0.3.0 下载链接，网站介绍及更新日志只展示中文。

- 根目录新增 `package.json`，可用 `bun run dev` 启动既有的签名调试 watch 会话：Go 源码与配置变化后自动重构建并重启应用，Ctrl+C 停止会话，无需 JavaScript 依赖。

- 为 `go-voice` 增加阿里云百炼 Omni Realtime WebSocket 听写，支持配置 WSS 端点与模型、按服务商和端点隔离钥匙串凭据、输入转写预览及松开按键后手动提交。只使用输入转写，不生成对话回答；测试连接只发送静音，失败时不切换引擎。
- 本地 macOS 构建改为 `go-voice-debug.app`，主应用标识为 `yz.sh.mygo-go-voice.debug`，三个原生 helper 的标识也增加 `.debug`。调试版使用独立的临时配置且不启用更新，保留正式版身份并分离 macOS 权限记录。跨次构建保留调试版权限仍需稳定的签名证书。

- 新增 `go-voice` 产品网站 `go-voice.yzsaas.net`（`website/`）：以黑色为主的中英双语页面，介绍应用与 FoloToy AI Passport 硬件，并根据已发布的 release 提供最新 Apple 芯片 DMG 下载与更新日志。使用 `cf` CLI 部署到 Cloudflare Workers。

- `go-voice` 词库新增可选的 Jev 同音词判断（新“Jev 判断”标签页，默认关闭）：对读音像中文词条的每处片段，由 TypeSafe 的 Jev 给出“说的就是这个词”的概率；可能性高的由代码直接替换，可能性低的保留原文并要求 AI 润色不要改，只有拿不准的才交给 AI 润色。支持 TypeSafe 官方及任何兼容其 System One 接口的服务（如支付宝 MaaS 的 `jev-1.13`），可填写接口地址和模型。会把命中的片段及前后文、命中的词条发送到所选服务，不发送音频；超过 4 秒或失败时跳过。API key 按接口地址分别存入钥匙串，“测试连接”用固定示例句展示 Jev 的判断。

- `go-voice` 听写胶囊改为在一行定宽文字中实时显示正在识别的内容，不再只显示“请说话…”：最新的字从右侧滑入，较早的字向左滚出。Apple 本地识别改为说话过程中约每秒刷新一次文字，不再等松开按键后一次性出现。胶囊上方的预览气泡只在 AI 润色期间出现，显示将要插入的 ASR 原文。胶囊和气泡不再有阴影。

- `go-voice` 新增“通用”页（侧边栏“应用”分区），提供“登录时打开”开关：把应用加入 macOS 登录项，与系统设置中的更改保持一致；macOS 要求允许时引导前往“登录项”。登录时启动后只驻留菜单栏、不打开窗口，缺少必需权限时才显示权限页。

- `go-voice` 新增在线语音识别：新的“语音识别”页可从下拉框选择 Apple 本地 SpeechAnalyzer（默认）或支付宝 MaaS 模型：Fun-ASR、Paraformer v2 边说边传音频、边识别边出字，非流式的 Qwen3-ASR-Flash 在松开按键后整段识别；API key 存入钥匙串，“测试连接”只发送半秒静音，失败时以原因结束本次语音，不会自动改用本地识别。选择在线模型后，每次语音的音频会发送到支付宝 MaaS。

- 在 `go-voice` 侧边栏底部连接状态卡片右侧增加退出按钮，除菜单栏外也能从窗口退出应用。固件安装期间按钮不可用，避免中断写入。

- `go-voice` 词库支持一键导入、导出 JSON，按钮位于“词条”“替换词”标签旁。导出把词条（含锁定和权重）和全部替换词写入一个文件；导入时合并：新增没有的条目，更新已有条目，不删除任何内容，跳过无效或冲突的条目，并显示处理结果。其他格式的文件不会改动词库。

- `go-voice` 因音频中断而结束会话时会说明具体原因：连续 3 秒没有音频、超过 5 分钟上限、帧序号超出上限、乱序等待的帧过多，或丢失超过 100 帧，并改用中文提示，不再所有情况都显示同一句英文。各项限制本身不变。

- `go-voice` 默认的轻度润色只做听写真正需要的事：先按词库纠正专有名词，再去掉没有意义的语气词（叹词、停顿用词、说错重来的重复、因犹豫声识别出的单个字母）并修正标点，不改写、不调整语序、不添加内容；有实际意义的同类词会保留。忠实度检查不再因去掉语气词而退回原文，但仍拒绝删掉实际内容的输出。“模式”一行会说明当前所选模式的作用。

- 为 `go-voice` 词库增加替换词规则，位于“词条”旁的“替换词”标签页：用两个输入框填写原词（识别出的错误写法）和替换词（正确写法），实时和最终识别结果会在本地直接替换，不经过 AI，并先于胶囊预览、插入和润色生效。英文原词不区分大小写、只匹配完整单词；原词不能是词库中的词条，同一原词只能有一条规则。列表中的规则可以编辑和删除。一条规则替换出的结果还会继续被其他规则修正，AI 润色后的文字也会再替换一次，模型无法把错误写法改回去。关闭 AI 润色时，直接插入替换后的文字。

- `go-voice` 远端润色关闭思考模式，并让词库纠正优先。请求会带上常见的关闭思考参数；服务商以 HTTP 400/422 拒绝时减少参数重试，并按端点和模型记住可接受的级别；回复中的 `<think>` 内容会被去掉。调用模型前先比对拼音（合并 z/zh、c/ch、s/sh、n/l 和 -n/-ng），把读音像中文词库词的片段列为候选，提示词要求模型优先使用词库词。“测试连接”会显示用时、关闭思考参数是否被接受，以及包含拼写纠正和同音字纠正的示例。新增依赖 go-pinyin（MIT）。真实服务商上的效果仍待用户验证。

- 为 `go-voice` 听写胶囊增加文字预览：胶囊上方的气泡在说话时以清晰字号显示最多三行最新识别的文字，AI 润色期间显示将要插入的 ASR 原文，方便判断是否直接插入。胶囊内改为显示会话状态。在真实应用上方的显示效果仍待用户确认。

- `go-voice` 的所有润色模式都按词库纠正专有名词，不再只有“专名”模式：被识别成同音字、英文词的中文音译、大小写或空格错误的词，会改为词库中的写法，但不会加入没说过的词。忠实度检查不再把这类纠正（例如“够我四”改为“Go Voice”）退回 ASR 原文，同时仍拒绝用词库词替代原本听写内容的输出。“测试连接”的示例也会展示这种纠正。真实模型上的效果仍待用户验证。

- `go-voice` 的 AI 润色页每个操作都在按钮下方给出结果：“保存设置”“保存到钥匙串”“删除凭据”现在用中文显示成功或失败原因，不再静默保存或弹出英文提示；表单再次修改后提示自动隐藏。“保存设置”会把输入的 API key 一并存入钥匙串，避免保存前输入的 key 丢失；远端引擎缺少 https 端点或模型时也会提醒。

- 在 `go-voice` 的 AI 润色页新增“测试连接”。它在保存前按当前填写的内容检查 OpenAI 兼容接口，使用输入的或已保存的 API key，只发送一段固定示例句。成功时显示润色后的示例；失败时给出可能原因，例如 key 无效（HTTP 401/403）、端点路径或模型错误（HTTP 404）、限流、服务端错误、超时，或返回了另一种接口格式。已用 HTTPS 模拟服务器测试覆盖；真实服务商仍待用户验证。

- 去掉 `go-voice` 设备页常驻的操作栏，改为在设备卡片中按状态显示按钮：没有设备时“扫描附近设备”，断开时“重新扫描”，会话中“取消本次语音”，只有模型准备失败时才出现“重新准备语音模型”。已连接时从设备列表的“扫描”重新搜索。右上角的模型状态改为中文，模型就绪时圆点为绿色，不可用时为红色。

- 移除 `go-voice` 设备页上单独的“V1 未认证连接”提示。连接仍未认证，也不会被描述为已认证或已加密。

- 为 `go-voice` 增加签名更新与公证发布。正式版使用 Developer ID 和 hardened runtime 签名（包括嵌套 helper），经过公证并 staple，并通过 MyGo 原生更新窗口从 `go-voice-v*` GitHub release 中 Ed25519 签名的归档与差分包自动更新，每天检查一次；菜单栏“检查更新…”会打开该窗口，调试版则说明其不会更新。`gui/scripts/release.sh` 负责验证、以 `build.sh --release` 构建、核对签名、staple 与 Gatekeeper，并上传草稿 release（manifest 最后上传）。固件工作流现在忽略 `go-voice-v*` tag。发布说明位于 `gui/docs/CHANGELOG.zh_CN.md`；实际公证发布与已安装副本的更新仍待用户检查。

- 将 `go-voice` 改为菜单栏应用：不显示程序坞图标（`LSUIElement`），菜单栏使用由透明白色标志生成的模板图标，菜单包含“设置…”（显示主窗口）、占位的“检查更新…”对话框和“退出”。关闭主窗口仅隐藏而不退出，隐藏的窗口也不会阻止退出。系统权限弹窗文案改为 FoloToy AI Passport，不再称作工牌。

- 为 `go-voice` 新增悬浮听写胶囊条：基于不获取键盘焦点显示的 MyGo 面板，位于当前屏幕底部中央，包含取消（仅会话期间全局占用 Esc）、小声波、实时转写的最新文字，以及跳过 AI 润色、直接插入 ASR 原文的确认按钮。每次粘贴前胶囊条都会隐藏，确保插入目标为听写的应用；会话取消或失败时短暂显示原因。在真实目标应用上的表现仍待用户确认。

- 将 `go-voice` 远端润色引擎标注为 OpenAI 兼容接口，并可选择 Chat Completions（默认，`/chat/completions`）或 Responses（`/responses`，携带 `store: false`）。选择器旁显示所追加的路径；未保存该选项的旧设置继续使用 Chat Completions。请求与响应格式已有单元测试；对真实服务商的调用仍需用户验证。

- 为 `go-voice` 新增权限引导。helper 通过经过校验的 `permission` 事件上报蓝牙、语音识别、辅助功能和自动化（System Events）授权状态；“权限”页逐项显示状态，系统仍可弹出请求时直接申请，否则打开对应的“隐私与安全性”面板，并在有缺失时持续刷新。启动时缺少必需权限会自动打开该页，设备页也提供入口。开始会话时不再申请语音识别权限，缺少授权会以明确原因结束会话，而不是报 `Speech start timeout`。真实 TCC 弹窗与授权仍待用户确认；ad hoc 签名重新构建后可能需要重新授予辅助功能。

- 修复 `go-voice` USB 固件预检总在写入前中止：espflash 返回的 ROM READ_FLASH_SLOW 数据带有 ESP32-C3 状态尾，读取分区表和 bootloader 时被误判为不完整。FirmwareHelper 现仅接受 64 字节数据加 0/2/4 字节状态尾并取前 64 字节。固件页同时显示 helper 错误码与失败阶段。已有单元测试覆盖，真机预检与写入仍待验证。

- 基于 MyGo v0.5.5 将 `go-voice` 界面重新设计为固定暗色的 macOS 26 应用：内嵌红绿灯的悬浮 Liquid Glass 侧边栏、位于柔和滚动边缘上的工具栏、分组内嵌区块、白色主胶囊与石墨次胶囊按钮，以及胶囊分段控件。主卡片只保留一个声波面板并随会话切换：空闲时为暗色圆点，录音时以最新峰值为中心镜像显示橙色 FoloToy AI Passport 声波，识别、润色和插字时显示橙色脉冲，下方显示时长、RMS 与峰值。录音状态门禁不变；应用会拒绝的操作现显示为禁用。新增视图测试、页面快照，以及升级所需的 golang.org/x/text BSD 声明。真实 macOS 26 显示效果仍待用户确认。

- 为 `go-voice` 新增原生 USB 固件安装：固定使用 Rust `FirmwareHelper` 与 `espflash` 4.6.0，增加严格有界的 NDJSON Go 桥接和第四个 MyGo 页面，打包签名的 app-only 固件与 manifest，并以 fail-closed 方式检查 ESP32-C3、8 MB Flash、安全状态、分区表和永久 Recovery。唯一 Flash 修改是向 `0x10000` 写入并校验 factory 应用；不存在 erase-all，也不写 bootloader、分区表、NVS、PHY、`cardid` 或永久 Recovery。新增跨语言 manifest 测试、真实 USB 枚举证据、Rust/Go/Swift/打包/签名门禁及 serialport MPL-2.0 声明。实际设备预检/写入/复位和写入中断后的 Recovery 仍需真机验收。

- 将 Go Voice 标志和 `go-voice` 应用图标简化为软萌 Gopher，保留略微歪头、灵动眼神、小门牙和收起的小爪子，右下角配麦克风标识语音输入用途。图案保持白色、原黑色区域改为透明，保留抗锯齿边缘，提升深色设备屏幕上的对比度。保留生成的原始角色图稿，由同一源图生成桌面 PNG 与 64×64 RGB565 固件资源。桌面图标使用不透明黑色圆角底，固件仍保持透明白色图稿；独立的 APP 图标生成流程确保两种背景分别保留。重新构建并校验合并固件以包含当前标志。

- 实现基于 MyGo 的 Go Voice `go-voice` macOS 26+ Apple Silicon 桌面宿主：自有 CoreBluetooth helper、严格 BLE V1/IMA-ADPCM 会话层、实时波形/电平/时长/转写/帧诊断、动态词库、SpeechAnalyzer/SpeechTranscriber、Apple FoundationModels 与显式 OpenAI-compatible 润色、Keychain 凭据和安全回退的跨应用插字。新增 `./tools/validate.sh --gui`，构建 ad-hoc 签名的 `go-voice.app` 并运行 Go/Swift host tests；运行验收修复两个 helper 在持久 stdin 上等待满缓冲或 EOF 的问题，将它们打成供 TCC 识别的签名嵌套 app，保持 Speech 所需的物理主 RunLoop，并修正 PTT 去重，使固件连续会话复用控制序号 0/1 时不会被丢弃。真实 FoloToy AI Passport 耐久及端到端插字仍待验收。

- 基于 Vokie 提交 `ce26ea043251102155e54bc9503eb8133d5cafa8` 引入 Go Voice 一期固件：自有 BLE UUID、Go Voice 广播及状态文案、用户提供的 64×64 RGB565 图形及现有 Montserrat 20 标题字体。保留 BLE V1 帧格式、按键、功耗、参考开机音及抢占、受保护分区。见[范围与来源](go-voice-firmware.zh_CN.md)；发布前须确认独立音效授权。

- 停止音频时钟前明确静音并关闭扬声器 DAC，保留麦克风偏置与滤波器状态。音量为零时关闭输出，仅在音频流运行且明确请求播放时重新开启；读回寄存器校验失败则保持流运行并允许重试。此项处理排查刷写后偶发杂音时发现的待机输出隐患，实际声音仍需实机确认。

- 短按改为去抖后的松手即执行，不再等待双击判定；连续短按分别响应。OK 保留 650 ms 长按清空/取消，快按后紧接长按也有效，长按松手不再额外删除。

- 原生 USB Serial/JTAG 总线活跃时持有禁止轻睡眠锁及 80 MHz APB 频率下限锁，每 2 秒轮询，并以 5 秒释放宽限容忍短暂 SOF 间隙。总线长时间静默或拔线后重新允许电池待机。主机挂起及重插行为仍需硬件验收，此前孤立断连的原因尚未确认。

- 待机期间保留 ES8311 偏置及滤波器状态，暂停 I2S/DMA，避免完全重启 codec 导致录音开头反复削波。恢复新的 DMA 后先读走一个不完整的 I2S 槽采样（16 kHz 下为 62.5 微秒），再返回完整 PCM 帧，避免首采样尖峰进入 ADPCM 初始值。暂停/恢复失败时保留必要电源锁并支持重试，不中止固件；保留的模拟静态电流尚未测量。

- 降低熄屏待机活动：闲置时暂停 I2S，录音任务等待事件，熄屏暂停 LVGL tick/刷新/动画，并启用动态调频、自动轻睡眠及 BLE Modem-sleep。ADC 按键改为 20 ms 扫描，相同主机状态不再重置亮屏超时。实际节电幅度、唤醒和音频表现仍须真机测量。

- 新增两秒 Vokie 开机音，以适中音量在音频任务中每次开机播放一次；开始语音输入时让位给录音，随后静音并丢弃开机播放期间的麦克风采样。
- 在 Vokie 完成 BLE 握手前，状态界面保持 OFFLINE；未连接时按 DOWN 或 OK 也不会再误显示 READY。
- 将 VOICE/SEND/UNDO 放在靠近右边缘的位置，用低调的细线指向对应按键。每个右侧按键仅显示自身文案与指引线。单项提示统一使用原来 SEND 所在的中间位置及单行紧凑背景，指引线从这里连向对应实体按键；换键时替换上一项提示。开机保留原来的三项位置和指引线路径，展示全部三项，三秒后淡出。独立硬件电源键不提供应用层按下事件。
- 将语音状态及说明放在标志上方，READY 顶部对齐上方按键指引线；Vokie Power 移至标志下方，文字底部对齐下方按键指引线。标志的视觉中心对齐 SEND 行，主要内容保持水平居中。
- 电池图标和电量百分比以低调灰色（`#646C78`）常驻屏幕右上角，亮屏时持续可见，每 30 秒刷新。实体按键选中的提示在录音或处理中保持显示，空闲按键提示超时 3 秒后淡出；开机全部提示固定三秒后淡出，即使主机进入活动状态也不延长。状态变化和电量刷新不会唤出提示，电量刷新保留原有屏幕变暗和熄屏策略。标题采用 22 px Barlow Condensed Bold 窄体粗字，移除重复的状态圆点。20% 及以下标红；SOC 无效时显示实测电压，读数均不可用时显示 `--%`。
- 使用 FoloToy PR #38 的原装 520 mAh 电池参数初始化 CW2017，恢复未配置电量计的有效 SOC，并提供读回校验、有界就绪等待和避免重复写入。

- 在固件旁新增独立的 AI Passport Vokie Plugin，支持导入兼容的 Vokie 版本并开发自定义桌面端集成；同时明确当前 Vokie 的内置连接为默认使用路径。
- 用 Vokie AI Passport 语音固件替换硬件 Demo 菜单：实现并公开 BLE V1 外设协议、16 kHz 单声道 IMA ADPCM 传输、实体 PTT/编辑按键、主机驱动的状态界面和三级背光省电策略。
- 增加 fork 专用项目 README、BLE 协议说明、第三方声明和独立的 Vokie 品牌素材条款，为公开分发做好准备。
- 移除继承自上游的自动同步工作流；本独立衍生项目的下游 fork 通过显式评审合并公共仓库更新。

- 按功能域整理文档并采用双入口：根目录 `AGENTS.md` 变为薄路由（只保留硬约束与任务路由），详细的 AI 开发工作流下沉到 `docs/development/ai-guide.md`，`agent-guide.md` 并入其中。为 `docs/development/` 增加二级分区（`engineering/`、`ci/`、`release/`），把 `plays/` 应用档案与 `experiences/` 移入带专属 README 的 `docs/reference/` 参考区；删除 `docs/software-design/`（空脚手架）；把 `assets/{fonts,images,music}/README` 三个叶子 README 并入 `assets/` README；把 `project-completion` 的六个子文档压平为单文件；并把每个目录统一为单一 README，消除所有 `INDEX` 文件与一处重复经验索引。所有交叉引用与文献链接已更新；未丢弃任何内容。

- 将小程序 BLE 安装兼容提升为二创模板强制契约：固定保护 `cardid`/Recovery 分区，
  保留上键持续 5 秒进入 Recovery 的 bootloader hook，并在 CI 强制校验合并镜像结构、
  分区表 MD5/范围、3 MB 应用上限和保护分区数据不入包。
- 规定多应用发布的 Release 标题约定：tag 按 `v<版本>-<应用名>`（如 `v0.1.0-voice-keychain`）命名，让 Release 标题同时带版本与应用名；发布成功后核对标题，保证一眼扫 Release 列表就能区分是哪个应用。
- 新增发布后收尾流程：`issue-suggestions` skill 用于把用户反馈作为 issue 提交到上游项目；`experience-pr` skill 用于把可复用的开发经验作为文档 PR 提交；新增 `docs/experiences/` 目录保存单条经验文件；并配套 `project-completion`、`file-issues` 与经验索引文档。
- 精简仓库根目录：将 GitHub 可识别的社区治理文档迁入 `.github/`，将变更记录迁入 `docs/`，同步全部引用，并在仓库检查中加入根目录文档白名单。
- 全仓库文档语言规范：所有维护中的 Markdown 默认 `.md` 文件使用英文，简体中文使用配对的 `.zh_CN.md`，双方提供语言切换；静态检查会阻止缺失配对、缺失切换链接或英文默认页混入中文正文。
- AI 开发流程一期：精简按任务加载的上下文入口，统一本地/CI 验证脚本，新增 PR 自动构建与模板，并提交依赖锁文件以提高构建可复现性。
- PR 审查修复：GitHub Actions 固定到完整 commit SHA，构建与发布 job 按最小权限拆分，同步 checkout 关闭凭证持久化；补充 Feature Request / Usage Question issue 表单；启用并修正私密安全报告兜底说明；清理 README 路径、CI 触发条件与历史分支描述漂移。
- 语言规范变更：commit 标题、PR 标题与 body 由"默认中文"改为**使用英文**（`docs/contribution/commit-and-pr.md` 更新）；中文写作规范（全角标点）适用范围剔除 PR/MR 描述（`doc-conventions.md` 更新）。
- CI 构建改造：`build-firmware.yml` 显式传入 `SDKCONFIG_DEFAULTS=sdkconfig.defaults` 再 `idf.py build`，由 defaults 启用自定义分区表（`CONFIG_PARTITION_TABLE_CUSTOM=y`，文件名为 `partitions.csv`）；`CONFIG_ESPTOOLPY_HEADER_FLASHSIZE_UPDATE` 改为 `n`，再用 `idf.py merge-bin -o build/FoloToy-AI-Passport-full.bin` 合并可直刷完整固件；产物精简为仅 full.bin；`actions/cache` 升级到 v5 以消除 GitHub Actions Node.js 20 弃用警告；CI 文档同步更新。
- 合并上游 PR #6（wireless-low-power-demos）以解决 PR #4 冲突：引入无线/低功耗 demo（`main/demo_wifi.c`、`demo_ble.c`、`demo_radio.c`、`demo_low_power.c`）、`partitions.csv`（NVS/PHY/3 MB factory-app 分区）、`main/CMakeLists.txt`/`main.c`/`demo.h`/`sdkconfig.defaults` 更新；同步硬件指南的 Wi-Fi/BLE/低功耗章节；README 能力契约表补充 Wi-Fi/Bluetooth LE/Low power 三项（中英双语）。
- 提交规范补充：`docs/contribution/commit-and-pr.md` 明确 PR 标题与 commit 标题使用相同的 Conventional Commit 格式和英文祈使句，不用名词短语当标题。
- CI 与文档清理：`sync-main.yml` 移除 `test_mode` 残留模板注释；`docs/development/coding-conventions.md` 将「Redis TTL」条目泛化为「缓存组件」条目（当前固件无 TTL 约束需求，消除从模板带入的无关约定）。
- 补充通用规范（借鉴 Shinku）：`docs/contribution/doc-conventions.md` 新增中文全角标点规范（正文 `，`；`（`）`，代码/命令/路径保留英文原样）、凭证不入仓规范（token/密钥/私钥绝不入仓，提交前 git diff 扫描敏感前缀）、文件删除安全规范（删除走系统回收站，不用 rm -rf/git clean -fd）。
- 代码注释规范强化：`docs/development/coding-conventions.md` 补充完善注释要求——函数说明（用途/参数/返回值/副作用/线程上下文/内存所有权/初始化顺序）、变量说明（语义/取值范围/生命周期/同步要求）、逻辑注释（状态机/时序/寄存器/魔数依据），覆盖范围宁多勿少，中文注释保留英文技术术语。
- 文档去 AI 化：`docs/README.md` / `docs/README.zh_CN.md` 移除 AI 专属章节（Entry point、Source-of-truth、提需求格式、BSP 边界、Runtime invariants、验收交付格式、构建命令），README 只保留给人看的项目介绍、硬件能力契约、demo 案例与项目结构；构建命令章节删除（与 `docs/development/build-and-test.md` 重复）。
- 新增 `docs/development/agent-guide.md`：集中承载"AI 如何在本仓库工作"（上下文建立顺序、事实来源优先级、提需求格式、BSP 边界、运行时规则、交付格式），并链接 build-and-test 与硬件指南，不重复构建命令与验收矩阵。
- 同步更新索引：`AGENTS.md` 规则索引新增 agent-guide 条目；`docs/INDEX.md` 与 `docs/development/README.md` 新增 agent-guide 索引行。
- 文档补充：`docs/fork-guide.md` 说明「为什么根目录不放置 README」——根目录 README 预留给 fork 开发者自行放置（上游留空），fork 后可将自己的内容写入根目录 `README.md` 介绍 fork 后的项目；GitHub 显示优先级（根 README > docs/README.md）契合该预留意图。
- 分支合并：创建 `main-update` 分支（基于与上游一致的 main），将 `feature/repo-structure`、`ci/build-firmware`、`ci/sync-main` 三个分支合并进来，统一 docs 结构（CI 文档归入 `docs/development/`，workflow 文件随 ci 分支引入 `.github/workflows/`）；解决 development/software-design README 的 add/add 冲突。
- 合并后审查修复：`docs/INDEX.md` 补充 CI 文档索引；`docs/fork-guide.md` 修正 workflow 引用为 `.github/workflows/sync-main.yml`；`docs/README` 双语项目结构块补充 `.github/workflows/` 与 CI 文档说明。
- ci 分支 CI 文档路径调整：`ci/build-firmware` 的 `docs/software-design/CI-build-and-release.md` 与 `ci/sync-main` 的 `docs/software-design/CI-sync-main.md` 均移入各分支的 `docs/development/`（CI 属工程规范）；`docs/software-design/README.md` 保留为软件设计索引；feature 分支的 software-design 索引同步更新引用。
- fork 补充文档目录迁移：`assets/docs/` 移至 `docs/assets/`（文档素材归入 docs/ 更合理），新增 `docs/assets/.gitkeep` 空目录占位；同步更新 AGENTS.md / INDEX / doc-conventions / fork-guide 的路径引用。
- 文档结构调整：根目录不再放 README——上游英文 README 移入 `docs/README.md`、中文移入 `docs/README.zh_CN.md`（GitHub 从 docs/ 识别主 README）；原 `docs/README.md` 根总索引更名为 `docs/INDEX.md`；同步更新 AGENTS.md / CONTRIBUTING / SUPPORT / fork-guide / doc-conventions 的路径引用。
- 初始化项目文档：新增 `AGENTS.md`、`CLAUDE.md` 和 `CHANGELOG.md`。
- 仓库结构规整：上游英文 `README.md` 更名为 `README.en_US.md`，保留 `README.zh_CN.md`。
- 新增目录骨架：`docs/`（software-design / hardware-design）、`assets/`（fonts / images / music，各含 `README.md`）、`skills/`。
- 将上游硬件开发指南归位到 `docs/hardware-design/AI_HARDWARE_DEVELOPMENT_GUIDE.md`。
- 文档规范：子目录 readme 统一为大写 `README.md`；补充 fork 用户约定（main 只动根 README）。
- 扩展 fork 用户约定：`main` 分支允许修改根目录 `README.md` 和 `assets/docs/`（README 不足以说明项目时存放补充文档与素材）。
- 新增 `assets/docs/` 目录约定：上游 main 只保留空目录 `.gitkeep`，内容文件仅存在于 fork；使用方法规范写入 AGENTS.md「给 fork 用户」约定。
- CI 文档迁移：`docs/software-design/CI.md` 从本分支移除，迁至 `ci/build-firmware` 分支并改名为 `docs/software-design/CI-build-and-release.md`。
- 补充 `main` 分支策略说明：解释 `main` 保持干净的两大原因（与上游同步无冲突 + 多小项目按分支整理）；例外——执意 main 开发需停用 CI 自动同步；提醒 fork 用户默认 action 关闭需手动启用（此条为整个 CI 的通用要求，统一写入 AGENTS.md）。
- 文档拆分：将 `AGENTS.md` 按主题拆为公共文档——新增 `docs/contribution/`（doc-conventions.md、commit-and-pr.md）与 `docs/development/`（build-and-test.md、coding-conventions.md），新增 `docs/fork-guide.md`；`AGENTS.md` 精简为简介 + 项目概述 + 必读文档索引。
- 同步更新索引：`docs/software-design/README.md`、`README.en_US.md` / `README.zh_CN.md` 的 `docs/` 目录说明。
- 参考 cindy 仓库文档组织完善索引：新增 `docs/README.md` 根总索引；AGENTS.md 规则索引按触发场景改写（附触发条件）；`docs/contribution/` 与 `docs/development/` 的 README 补充收录标准。
- 引入社区治理文档（参照 cindy 改写，放仓库根目录）：新增 `CONTRIBUTING.md` / `.zh_CN.md`（贡献指南，针对 ESP-IDF/AI agent/fork 场景改写）、`CODE_OF_CONDUCT.md` / `.zh_CN.md`（贡献者公约）、`SECURITY.md` / `.zh_CN.md`（安全报告流程）、`SUPPORT.md` / `.zh_CN.md`（支持渠道）；AGENTS.md 与 docs/README.md 同步引用。
