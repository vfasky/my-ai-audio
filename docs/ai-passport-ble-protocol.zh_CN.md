<p align="right">
  <strong>简体中文</strong> · <a href="ai-passport-ble-protocol.md">English</a>
</p>

# AI Passport BLE 语音协议

本页定义 Go Voice 的 UUID。协议行为保留 Vokie V1；下文 Vokie 主机行为是参考实现说明，未修改的 Vokie 内置客户端不支持这些新 UUID。Go Voice 需要兼容主机，不要求安装 Vokie。见[一期范围](go-voice-firmware.zh_CN.md)。

协议 V1 使用一个 Bluetooth Low Energy GATT 服务。AI Passport 作为外围设备，Vokie 或其他兼容应用作为中心设备。

## 服务

| 项目 | UUID | 方向 | 属性 |
| --- | --- | --- | --- |
| 语音服务 | `676f0001-766f-6963-652d-617564696f00` | 无 | 无 |
| Control | `676f0002-766f-6963-652d-617564696f00` | 双向 | 有响应写、通知 |
| Audio | `676f0003-766f-6963-652d-617564696f00` | 设备到主机 | 通知 |
| Device info | `676f0004-766f-6963-652d-617564696f00` | 设备到主机 | 读取 |

设备以 `Go Voice` 名称广播，只允许一个连接。主机必须协商 `ATT_MTU >= 185`，订阅 Control 和 Audio，接收并校验 `hello`，然后发送 `host_ready`。握手完成前，固件会忽略 PTT 输入。

Control 只接受有响应写。无响应写和读取 Control 不属于 V1 契约；读取 `hello` 应使用 Device info。

## 控制消息

Control 值是带 `v: 1` 的完整 UTF-8 JSON 对象，最长 182 字节。设备到主机的消息如下：

```json
{"v":1,"type":"hello","device":"ai-passport","fw":"0.3.0","codec":"ima-adpcm","sampleRate":16000,"channels":1,"frameMs":20}
{"v":1,"type":"ptt_down","sessionId":123,"seq":0}
{"v":1,"type":"ptt_up","sessionId":123,"seq":1,"finalSequence":7}
{"v":1,"type":"button_event","button":"down","event":"click","durationMs":0,"seq":2}
{"v":1,"type":"button_event","button":"ok","event":"long","durationMs":650,"seq":3}
{"v":1,"type":"device_error","sessionId":123,"message":"transport"}
```

- `sessionId` 和 `seq` 是无符号 32 位整数。
- 每次 PTT 会话的音频序号从零开始，与控制事件 `seq` 相互独立。
- 没有音频帧时使用 `finalSequence: 4294967295`。
- `device_error.message` 仅用于诊断，不能包含音频或转写内容。
- 当前设备错误包括 `audio_read`、`mtu` 和 `transport`。

主机到设备的消息如下：

```json
{"v":1,"type":"host_ready"}
{"v":1,"type":"host_state","state":"processing"}
```

固件接受 `ready`、`recording`、`processing`、`success` 和 `error`。Vokie 插件通常发送 `ready`、`processing`、`success` 或 `error`。可选的 message 字段仅用于诊断，不能包含转写文本。

## 按键语义

`button_event` 表示非 PTT 动作。当前 Vokie 主机映射如下：

| 设备动作 | 主机行为 |
| --- | --- |
| 空闲时单击 `UP` | 发送 `ptt_down` 并开始采集 |
| 采集中单击 `UP` | 完成当前帧并发送 `ptt_up` |
| 单击 `DOWN` | `send_enter` |
| 请求进行时单击 `OK` | `session_cancel` |
| 空闲时单击 `OK` | `delete_char` |
| 请求进行时长按 `OK` | `session_cancel` |
| 空闲时长按 `OK` | `clear_input` |

主机从采集开始直到收到终态 `session_state` 前，都把请求视为活动状态；该窗口覆盖 ASR、后处理、回传和粘贴。固件在 650 ms 时识别 `OK` 长按。

短按在去抖后的松手事件立即触发，不等待驱动的 180 ms 双击窗口。快速连续短按分别产生动作，Vokie 忽略驱动延后的单击/双击回调。OK 使用独立的 650 ms 长按计时，短按后马上再长按也有效；长按后松手不再额外删除一个字符。在主机握手前或另一连接期间开始的按压不产生动作。

## 音频分片封装

每个 Audio 通知包含一个完整分片。多分片帧以 `(sessionId, sequence)` 为键：

| 偏移 | 大小 | 值 |
| ---: | ---: | --- |
| 0 | 2 | 魔数 `0x5041`，小端（`41 50`） |
| 2 | 1 | 封装版本 `1` |
| 3 | 1 | 保留，必须为零 |
| 4 | 4 | 会话 ID，小端 |
| 8 | 4 | 音频序号，小端 |
| 12 | 1 | 分片索引 |
| 13 | 1 | 分片数量 |
| 14 | 2 | 载荷长度，小端 |
| 16 | 可变 | ADPCM 载荷 |

载荷长度必须与通知长度一致。发送方按照协商后的 `ATT_MTU - 3 - 16` 载荷空间分片。主机必须支持最多 64 个分片、忽略重复分片，并允许乱序到达。MTU 为 185 时，一个 166 字节 ADPCM 帧正好放入一条通知。

Control 通知不进行第二层分片，每条通知必须包含一个完整 JSON 值。

## IMA ADPCM 帧

每个独立的 20 ms 帧表示 320 个 16 kHz、16-bit、单声道 PCM 采样点。编码载荷固定为 166 字节：

| 偏移 | 大小 | 值 |
| ---: | ---: | --- |
| 0 | 2 | 初始预测值，有符号 int16 小端，即采样点 0 |
| 2 | 1 | IMA 步长索引，`0..88` |
| 3 | 2 | 采样点数，无符号小端，当前为 `320` |
| 5 | 1 | 保留，必须为零 |
| 6 | 160 | 319 个低半字节优先的 IMA ADPCM 半字节及一个零填充半字节 |

解码器把预测值限制在 int16 范围，并把步长索引限制在 `0..88`。最后一个高半字节是填充，应根据 `sampleCount` 忽略。

## 生命周期与丢包处理

1. 主机连接设备、校验 MTU，并订阅 Control 和 Audio。
2. 设备发送 `hello`；主机校验后发送幂等的 `host_ready`。
3. 用户单击 `UP`；设备发送 `ptt_down` 并开始采集。
4. 设备按递增序号发送音频帧。
5. 用户再次单击 `UP`；设备完成当前帧，发送带最终序号或空会话标记的 `ptt_up`。
6. 主机等待并解码 `finalSequence` 之前的完整帧，再发送主机录音会话停止命令。
7. 兼容主机可以用 320 个静音采样点替代缺失帧，但恢复数量必须有上限。Vokie 插件最多恢复 100 个缺失帧。
8. 断线或设备终止错误会取消匹配的主机请求并清空缓冲区。
9. 重新订阅后收到重复但有效的 `hello` 时，主机重新发送 `host_ready`，不改变活动会话身份。

Vokie 插件把单次会话限制为 15,000 个音频帧（五分钟），等待会话接受的时间为 1.2 秒，连续三秒没有合法音频视为停滞，并在 `ptt_up` 后最多等待 500 ms 接收已声明的尾帧。这些是主机实现限制，不是额外的 BLE 字段。

## 安全与设备选择

协议 V1 不提供 BLE 配对、对端身份认证或应用层加密。CoreBluetooth 外围设备 UUID 只是路由偏好，不能证明设备身份。需要防止附近设备冒充或窃听的部署环境，应在未来协议中增加配对或应用层密码学方案。

Vokie 第一方主机目前使用 Apple Silicon Mac 上的 CoreBluetooth Helper，并可持久化首选外围设备 UUID。其他主机可以采用不同的设备选择策略，但必须保持上述线路协议不变。
