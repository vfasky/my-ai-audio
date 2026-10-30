// Package ui renders snapshots and emits user intentions; it owns no native sessions.
package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

type Device struct{ ID, Name string }
type Snapshot struct {
	Install                                                                                           firmware.State
	SpeechActive                                                                                      bool
	Devices                                                                                           []Device
	Selected, Connection, Firmware, BundledFirmware, Phase, Partial, Original, Refined, Status, Model string
	Ready                                                                                             bool
	Meter                                                                                             protocol.Meter
	Terms                                                                                             []vocabulary.Term
	Replacements                                                                                      []vocabulary.Replacement
	// VocabFeedback is how the last import or export of the vocabulary
	// ended.
	VocabFeedback    string
	VocabFeedbackOK  bool
	Packs            wordlists.Config
	PacksFeedback    string
	PacksFeedbackOK  bool
	PacksFeedbackFor wordlists.Config
	Config           refinement.Config
	Permissions      permission.Set
	// CanCommit: the dictation capsule may insert the ASR original now.
	CanCommit bool
	// Testing: the connection test of the OpenAI-compatible interface runs.
	Testing bool
	// Feedback is how the last action of the refinement page ended (test,
	// save, keychain), for FeedbackFor: it hides once the draft differs.
	Feedback    string
	FeedbackOK  bool
	FeedbackFor refinement.Config
	// ASR is the saved recognition engine. ASRTesting and ASRFeedback are
	// the connection test and the last action of the recognition page, for
	// ASRFeedbackFor.
	ASR            asr.Config
	ASRTesting     bool
	ASRFeedback    string
	ASRFeedbackOK  bool
	ASRFeedbackFor asr.Config
	// SavedKeys tells, per keychain account, whether an API key is saved
	// there; an account it lacks has not been looked up yet.
	SavedKeys map[string]bool
	// OpenAtLogin is whether the system starts go-voice at login.
	// LoginApproval: the login item waits for the user to allow it in
	// System Settings. LoginFeedback is how the last change ended.
	// Jev is the saved vocabulary check; JevTesting and JevFeedback the
	// connection test and the last action of its tab, for JevFeedbackFor.
	Jev             jev.Config
	JevTesting      bool
	JevFeedback     string
	JevFeedbackOK   bool
	JevFeedbackFor  jev.Config
	OpenAtLogin     bool
	LoginApproval   bool
	LoginFeedback   string
	LoginFeedbackOK bool
	// Hotkey is the saved chord held to record with the Mac microphone.
	Hotkey string
}
type Actions struct {
	FirmwareScan, FirmwareInstall func()
	FirmwareSelect                func(string)
	Scan, Forget, Cancel, Warmup  func()
	Select                        func(string)
	Put                           func(string, vocabulary.Term)
	Delete                        func(string)
	// PutReplacement saves a replacement in place of the one from previous.
	PutReplacement    func(previous string, r vocabulary.Replacement)
	DeleteReplacement func(from string)
	// ImportVocabulary and ExportVocabulary pick a JSON file and merge it
	// into, or write, the terms and replacements.
	ImportVocabulary, ExportVocabulary func()
	ConfigurePacks                     func(wordlists.Config)
	// Configure saves the settings, and a typed API key to the keychain.
	Configure  func(refinement.Config, string)
	Credential func(string, string)
	// TestRemote checks the interface as the page shows it, with the typed
	// API key, else the saved one.
	TestRemote        func(refinement.Config, string)
	DeleteCredential  func(string)
	RequestPermission func(permission.Name)
	OpenSettings      func(permission.Name)
	// Commit skips refinement and inserts the ASR original.
	Commit func()
	// Quit quits the app, as the menu bar's Quit does.
	Quit func()
	// ConfigureASR saves the recognition engine, and a typed API key to the
	// keychain; TestASR checks the engine with the typed key, else the
	// saved one; DeleteASRKey removes the saved key.
	ConfigureASR func(asr.Config, string)
	TestASR      func(asr.Config, string)
	DeleteASRKey func()
	// CheckKey looks up, without reading it, whether the keychain holds a
	// key for an account, and records the answer in Snapshot.SavedKeys.
	CheckKey func(account string)
	// SetOpenAtLogin adds go-voice to the login items, or removes it;
	// OpenLoginItems opens them in System Settings.
	SetOpenAtLogin func(bool)
	// ConfigureJev saves the vocabulary check, and a typed API key to the
	// keychain under its endpoint; TestJev judges a fixed sample at the
	// drafted endpoint and model with the typed key, else the saved one;
	// DeleteJevKey removes the key saved for the drafted endpoint.
	ConfigureJev   func(jev.Config, string)
	TestJev        func(jev.Config, string)
	DeleteJevKey   func()
	OpenLoginItems func()
	// SetHotkey saves the chord held to record with the Mac microphone, or
	// hotkey.Off.
	SetHotkey func(string)
}
type View struct {
	Page                  int
	Snapshot              Snapshot
	Actions               Actions
	term, editing, secret string
	// The replacement editor: the source and target typed, and the source
	// of the replacement being edited.
	from, to, editingFrom string
	// vocabTab is the tab of the vocabulary page: terms or replacements.
	vocabTab int
	locked   bool
	Config   refinement.Config
	// ASR is the recognition engine drafted on its page; asrSecret the
	// typed API key.
	ASR       asr.Config
	asrSecret string
	// asrSecrets and asrDrafts keep the key typed and the engine drafted
	// for each provider left in the drop-down, model and Omni address
	// included, so that trying another provider and coming back loses
	// neither.
	asrSecrets map[string]string
	asrDrafts  map[string]asr.Config
	// keyChecks are the keychain accounts already looked up.
	keyChecks map[string]bool
	// omniWorkspace and omniRegion draft the Omni address: the region is an
	// index of asr.OmniRegions, or len(asr.OmniRegions) for a custom address.
	omniWorkspace string
	omniRegion    int
	// Jev is the vocabulary check drafted on its tab; jevSecret the typed
	// API key.
	Jev       jev.Config
	Packs     wordlists.Config
	jevSecret string
	Loaded    bool
	// AppIcon is the packaged app icon shown at the top of the sidebar.
	AppIcon *ui.Bitmap
	// slide scrolls the words of the dictation capsule.
	slide slide
}

// TitleBarHeight is the inset title bar the window reserves for the traffic
// lights; the toolbar and the sidebar header align to it.
const TitleBarHeight = 52

const sidebarWidth = 236

var pages = []struct {
	id, title, subtitle string
	icon                *ui.SVG
}{
	{"voice", "设备与语音", "从 FoloToy AI Passport 接收音频并在本机转写", iconWave},
	{"vocabulary", "词库", "个人专名、通用词库与 IT 行业词库", iconBook},
	{"recognition", "语音识别", "Apple 本地识别，或在线识别服务", iconCaption},
	{"refinement", "AI 润色", "本地优先，失败时保留 ASR 原文", iconSparkle},
	{"firmware", "固件安装", "通过 USB 安装随应用提供的固件", iconChip},
	{"permissions", "权限", "开始使用前授予系统权限，避免使用中途失败", iconShield},
	{"general", "通用", "登录时打开、麦克风热键等应用行为", iconSliders},
}

var (
	engines      = []string{"local", "remote"}
	engineLabels = []string{"Apple 本地", "OpenAI 兼容接口"}
	apis         = []string{refinement.APIChat, refinement.APIResponses}
	apiLabels    = []string{"Chat Completions", "Responses"}
	apiPaths     = []string{"/chat/completions", "/responses"}
	modes        = []string{"off", "names", "light", "structured"}
	modeLabels   = []string{"关闭", "专名", "轻度", "结构化"}
)

func (v *View) Draw(c *ui.Context) {
	if !v.Loaded {
		v.Config = v.Snapshot.Config
		v.ASR = v.Snapshot.ASR
		v.syncOmni()
		v.Packs = v.Snapshot.Packs
		v.Jev = v.Snapshot.Jev
		v.Loaded = true
	}
	c.SetTheme(theme)
	c.Root().Background(colCanvas)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		v.sidebar(c)
		v.pane(c)
	})
}

// sidebar is the floating Liquid Glass source list of macOS 26, inset from
// the window edges with the traffic lights inside it.
func (v *View) sidebar(c *ui.Context) {
	ui.Column(c).Width(sidebarWidth).Shrink(0).Padding(8, 0, 8, 8).Children(func() {
		ui.Column(c).Grow(1).Radius(20).Material(glass.Glass{Tint: ui.RGBA(0, 0, 0, 0.45)}).Border(1, colHairline).Clip().Children(func() {
			ui.Box(c).Height(TitleBarHeight - 8).Shrink(0).DragWindow()
			ui.Row(c).Padding(2, 16, 14).Gap(10).Children(func() {
				if v.AppIcon != nil {
					ui.Image(c, v.AppIcon).Size(36, 36).Fit(ui.Contain).Label("go-voice")
				} else {
					ui.Box(c).Size(30, 30).Radius(9).Background(colWell).Border(1, colHairline).Center().Children(func() {
						ui.Icon(c, iconWave).Size(16, 16).TextColor(colOrange)
					})
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "go-voice").FontSize(14).FontWeight(600).SingleLine()
					ui.Text(c, "FoloToy AI Passport").FontSize(11).TextColor(colMuted).SingleLine()
				})
			})
			nav := pages[min(max(v.Page, 0), len(pages)-1)].id
			list := ui.Sidebar(c, &nav, func() {
				ui.SidebarSection(c, "工作区", nil, func() {
					for _, p := range pages[:GeneralPage] {
						item := ui.SidebarItem(c, p.id, p.icon, p.title)
						if p.id == "vocabulary" && len(v.Snapshot.Terms) > 0 {
							item.Children(func() {
								ui.Badge(c, fmt.Sprint(len(v.Snapshot.Terms))).Background(colPill).TextColor(colMuted)
							})
						}
						if p.id == "firmware" && firmware.NeedsUpdate(v.Snapshot.Firmware, v.Snapshot.BundledFirmware) {
							item.Children(func() {
								ui.Badge(c, "更新").Background(colOrange).TextColor(ui.Hex("#FFFFFF"))
							})
						}
						if missing := len(v.Snapshot.Permissions.Missing()); p.id == "permissions" && missing > 0 {
							item.Children(func() {
								ui.Badge(c, fmt.Sprint(missing)).Background(colOrange).TextColor(ui.Hex("#FFFFFF"))
							})
						}
					}
				})
				ui.SidebarSection(c, "应用", nil, func() {
					for _, p := range pages[GeneralPage:] {
						ui.SidebarItem(c, p.id, p.icon, p.title)
					}
				})
			}).Grow(1).Background(ui.Transparent).Label("导航")
			if list.Changed() {
				for i, p := range pages {
					if p.id == nav {
						v.Page = i
					}
				}
			}
			v.connectionCard(c)
		})
	})
}

func (v *View) connectionCard(c *ui.Context) {
	s := v.Snapshot
	dot, title := colFaint, "未连接"
	switch {
	case s.Install.Busy():
		dot, title = colOrange, "固件安装中"
	case s.Ready:
		dot, title = colGreen, "已连接"
	case s.Selected != "":
		dot, title = colOrange, "等待设备"
	}
	ui.Row(c).Margin(0, 10, 10).Padding(10, 8, 10, 12).Gap(8).Radius(14).Background(ui.RGBA(0, 0, 0, 0.28)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				statusDot(c, dot)
				ui.Text(c, title).FontSize(12).FontWeight(600).SingleLine()
			})
			detail := "固件 " + fallback(s.Firmware, "未知")
			if firmware.NeedsUpdate(s.Firmware, s.BundledFirmware) {
				detail += " · 可更新"
			}
			if s.Selected == "" {
				detail = "尚未选择设备"
			}
			ui.Text(c, detail).FontSize(11).TextColor(colMuted).SingleLine()
		})
		// Quitting would interrupt a firmware write.
		busy := s.Install.Busy()
		tip := "退出 go-voice"
		if busy {
			tip = "固件安装中，完成后才能退出"
		}
		quit := ui.ButtonBase(c.Key("quit")).Size(30, 30).Radius(15).Center().Shrink(0).Label("退出 go-voice").Tooltip(tip).Disabled(busy)
		quit.Background(pick(quit.Hovered() && !busy, colPillHover, ui.Transparent)).TextColor(pick(quit.Hovered() && !busy, colText, colMuted))
		if busy {
			quit.Opacity(0.38)
		}
		quit.Children(func() { ui.Icon(c, iconPower).Size(15, 15) })
		if quit.Clicked() && !busy {
			v.Actions.Quit()
		}
	})
}

// pane is the content column: a scroll view under a toolbar that floats over
// the macOS 26 soft scroll edge.
func (v *View) pane(c *ui.Context) {
	bar := c.TitleBar()
	page := pages[min(max(v.Page, 0), len(pages)-1)]
	ui.Box(c).Grow(1).MinWidth(0).Children(func() {
		ui.Scroll(c).Fill().ScrollbarInsets(TitleBarHeight, 0, 0).Children(func() {
			ui.Column(c).Padding(TitleBarHeight+18, 36, 72).AlignItems(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(820).Gap(28).Children(func() {
					switch v.Page {
					case 0:
						v.device(c)
					case 1:
						v.vocabulary(c)
					case 2:
						v.recognition(c)
					case 3:
						v.refinement(c)
					case 4:
						v.firmware(c)
					case PermissionsPage:
						v.permissions(c)
					case GeneralPage:
						v.general(c)
					}
				})
			})
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(TitleBarHeight + 10).PassThrough().Material(glass.ScrollEdge{})
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(TitleBarHeight).Padding(0, bar.Right+16, 0, 28).Gap(12).DragWindow().Children(func() {
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, page.title).FontSize(15).FontWeight(650).SingleLine()
				ui.Text(c, page.subtitle).FontSize(11).TextColor(colMuted).SingleLine()
			})
			ui.Row(c).Height(30).PaddingX(12).Gap(8).Radius(15).Material(glass.Glass{}).Children(func() {
				label, state := modelStatus(v.Snapshot.Model)
				if v.Snapshot.ASR.Online() {
					label, state = "在线识别 · "+v.Snapshot.ASR.Provider(), modelReady
				}
				statusDot(c, pick(state == modelReady, colGreen, pick(state == modelFailed, colRed, colFaint)))
				ui.Text(c, label).FontSize(11.5).TextColor(colSubtle).SingleLine().MaxWidth(220)
			})
		})
		if v.Snapshot.Status != "" {
			ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).Height(64).PassThrough().Material(glass.ScrollEdge{Bottom: true})
			ui.Row(c).Attach(ui.AnchorBottom, ui.AnchorBottom).Bottom(16).Height(32).PaddingX(14).Gap(8).Radius(16).
				Material(glass.Glass{}).PassThrough().Children(func() {
				ui.Text(c, v.Snapshot.Status).FontSize(12).TextColor(colSubtle).SingleLine().MaxWidth(520)
			})
		}
	})
}

type modelState int

const (
	modelPreparing modelState = iota
	modelReady
	modelFailed
)

// modelStatus says in the interface's language how far SpeechBridge got in
// preparing the zh-CN speech model, from the messages of its warmup.
func modelStatus(message string) (string, modelState) {
	switch m := strings.ToLower(message); {
	case strings.Contains(m, "unavailable"):
		return "中文语音模型不可用", modelFailed
	case strings.Contains(m, "warmed"):
		return "中文语音模型已就绪", modelReady
	case strings.Contains(m, "downloading"):
		return "正在下载中文语音模型", modelPreparing
	case strings.Contains(m, "checking"):
		return "正在检查中文语音模型", modelPreparing
	}
	return "正在准备中文语音模型", modelPreparing
}

func phaseLabel(p string) string {
	switch p {
	case "ready":
		return "就绪 · 按下 FoloToy AI Passport 按键开始说话"
	case "starting":
		return "正在开始"
	case "recording":
		return "正在聆听"
	case "tail":
		return "正在收尾"
	case "finalizing":
		return "正在识别"
	case "refining":
		return "AI 润色中"
	case "inserting":
		return "正在插入文字"
	case "disconnected":
		return "已断开"
	}
	return "等待连接"
}

// isConnected reports whether the selected device is connected.
func isConnected(p string) bool { return p != "" && p != "disconnected" }

func isLive(p string) bool { return p == "starting" || p == "recording" || p == "tail" }
func isWorking(p string) bool {
	return p == "finalizing" || p == "refining" || p == "inserting"
}

// cardScans reports whether the device card shows the scan button: while no
// session runs and no device is connected.
func cardScans(s Snapshot) bool { return !s.SpeechActive && !isConnected(s.Phase) }

// deviceActions are the buttons of the device card, only those that apply
// now: scan while no device is connected, cancel while a session runs, and
// retry the speech model when its preparation failed.
func (v *View) deviceActions(c *ui.Context) {
	s := v.Snapshot
	_, model := modelStatus(s.Model)
	retry := model == modelFailed && !s.SpeechActive
	if !cardScans(s) && !s.SpeechActive && !retry {
		return
	}
	ui.Row(c).Justify(ui.Center).Gap(12).Wrap().GapY(12).Children(func() {
		switch {
		case s.SpeechActive:
			if pill(c, "cancel", "取消本次语音", pillSecondary, iconClose, nil, false) {
				v.Actions.Cancel()
			}
		case s.Selected == "":
			if pill(c, "scan", "扫描附近设备", pillPrimary, nil, nil, s.Install.Busy()) {
				v.Actions.Scan()
			}
		case cardScans(s):
			if pill(c, "scan", "重新扫描", pillSecondary, nil, nil, s.Install.Busy()) {
				v.Actions.Scan()
			}
		}
		if retry && pill(c, "warmup", "重新准备语音模型", pillSecondary, nil, nil, false) {
			v.Actions.Warmup()
		}
	})
}

func (v *View) deviceName() string {
	s := v.Snapshot
	if s.Selected == "" {
		return "未选择 FoloToy AI Passport"
	}
	for _, d := range s.Devices {
		if d.ID == s.Selected && d.Name != "" {
			return d.Name
		}
	}
	return "FoloToy AI Passport"
}

func (v *View) device(c *ui.Context) {
	s := v.Snapshot
	if missing := s.Permissions.Missing(); len(missing) > 0 {
		v.permissionBanner(c, missing)
	}
	where := "在本机实时转写。"
	if s.ASR.Online() {
		where = "经" + s.ASR.Provider() + "在线转写。"
	}
	ui.RichText(c,
		ui.Span{Text: "仅使用 "},
		ui.Span{Text: "FoloToy AI Passport 音频", Weight: 650},
		ui.Span{Text: where + "也可以按住热键用 Mac 麦克风说话。不保存、不回放音频；最终文本插入到你选择的应用。"},
	).FontSize(16).LineHeight(1.65).TextColor(colSubtle)

	live, working := isLive(s.Phase), isWorking(s.Phase)
	ui.Column(c).Background(colCard).Radius(26).Padding(28, 28, 26).Gap(24).Children(func() {
		ui.Column(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, v.deviceName()).FontSize(19).FontWeight(500).SingleLine()
			sub := phaseLabel(s.Phase)
			if s.Firmware != "" {
				sub += " · 固件 " + s.Firmware
			}
			ui.Text(c, sub).FontSize(14).TextColor(colSubtle).SingleLine()
		})
		v.deviceActions(c)
		// One well follows the session: idle dots, live bars, then a pulse while
		// recognition, refinement and insertion run.
		meter := s.Meter
		switch {
		case working:
			well(c, phaseLabel(s.Phase), true, func(p *ui.Painter, r ui.Rect) { drawPulse(p, r, true) })
		case live:
			well(c, "实时音频", true, func(p *ui.Painter, r ui.Rect) { drawWave(p, r, meter.Bins, true) })
		default:
			well(c, "等待语音", false, func(p *ui.Painter, r ui.Rect) { drawWave(p, r, meter.Bins, false) })
		}
		ui.Row(c).Justify(ui.Center).Gap(36).Children(func() {
			stat(c, "时长", fmt.Sprintf("%.1f s", s.Meter.Duration().Seconds()))
			stat(c, "RMS", fmt.Sprintf("%.3f", s.Meter.RMS))
			stat(c, "PEAK", fmt.Sprintf("%.3f", s.Meter.Peak))
		})
	})

	group(c, "transcript", "本次语音", nil, func() {
		for _, pair := range [][2]string{{"实时转写", s.Partial}, {"最终 ASR 原文", s.Original}, {"润色结果", s.Refined}} {
			row(c, pair[0], func() {
				ui.Text(c, pair[0]).FontSize(13).TextColor(colMuted).Width(96).Shrink(0).AlignSelf(ui.Start)
				t := ui.Text(c, fallback(pair[1], "—")).FontSize(14).LineHeight(1.5).MaxLines(10).Grow(1).Selectable()
				if pair[1] == "" {
					t.TextColor(colFaint)
				}
			})
		}
	})

	group(c, "devices", "附近的 FoloToy AI Passport", func() {
		// The card offers scanning while no device is connected.
		if !cardScans(s) && smallPill(c, "rescan", "扫描", pillGhost, nil, s.Install.Busy()) {
			v.Actions.Scan()
		}
		if s.Selected != "" {
			if smallPill(c, "forget", "忘记设备", pillGhost, nil, false) {
				v.Actions.Forget()
			}
		}
	}, func() {
		if len(s.Devices) == 0 {
			row(c, "empty", func() {
				ui.Text(c, "尚未发现设备。打开 FoloToy AI Passport 后点击“扫描”。").FontSize(13).TextColor(colMuted)
			})
		}
		for _, d := range s.Devices {
			row(c, "ble-"+d.ID, func() {
				ui.Box(c).Size(30, 30).Radius(15).Background(colPill).Center().Children(func() {
					ui.Icon(c, iconBolt).Size(14, 14).TextColor(pick(d.ID == s.Selected, colOrange, colMuted))
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, fallback(d.Name, "FoloToy AI Passport")).FontSize(14).FontWeight(500).SingleLine()
					ui.Text(c, d.ID).FontSize(11).TextColor(colFaint).SingleLine().FontFeatures("tnum")
				})
				if d.ID == s.Selected {
					ui.Badge(c, "已选择").Background(colOrangeSoft).TextColor(colOrange)
				} else if smallPill(c, "select", "选择并连接", pillSecondary, nil, s.Install.Busy()) {
					v.Actions.Select(d.ID)
				}
			})
		}
	})

	group(c, "diagnostics", "诊断", nil, func() {
		ui.Row(c).Padding(14, 16).Gap(10).Children(func() {
			for _, m := range []struct {
				label string
				value uint32
			}{{"收帧", s.Meter.Received}, {"缺帧", s.Meter.Missing}, {"补帧", s.Meter.Recovered}, {"重复", s.Meter.Duplicates}} {
				ui.Column(c).Grow(1).Basis(0).Padding(10, 12).Gap(4).Radius(12).Background(colWell).Children(func() {
					ui.Text(c, m.label).FontSize(11).TextColor(colMuted)
					ui.Text(c, fmt.Sprint(m.value)).FontSize(18).FontWeight(500).FontFeatures("tnum")
				})
			}
		})
		kv(c, "连接", fallback(s.Connection, "—"))
		kv(c, "握手", pick2(s.Ready, "已完成", "未完成"))
		kv(c, "语音模型", fallback(s.Model, "—"))
	})

	ui.Text(c, "插字需要辅助功能与自动化授权。Secure Input 或未授权时，最终文本仅保留在剪贴板。").FontSize(12).TextColor(colMuted).LineHeight(1.5)
}

func stat(c *ui.Context, label, value string) {
	ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
		ui.Text(c, value).FontSize(15).FontWeight(500).FontFeatures("tnum").SingleLine()
		spaced(c, label, colMuted).FontSize(10.5).LetterSpacing(1.6)
	})
}

func kv(c *ui.Context, key, value string) {
	row(c, key, func() {
		ui.Text(c, key).FontSize(13).TextColor(colMuted).Grow(1)
		ui.Text(c, value).FontSize(13).TextColor(colSubtle).SingleLine().MaxWidth(420)
	})
}

func (v *View) firmware(c *ui.Context) {
	s := v.Snapshot.Install
	tone := colMuted
	switch {
	case s.Phase == "error":
		tone = colRed
	case s.Phase == "complete":
		tone = colGreen
	case s.Busy():
		tone = colOrange
	}
	ui.Column(c).Background(colCard).Radius(26).Padding(26, 28).Gap(18).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			ui.Box(c).Size(52, 52).Radius(16).Background(colWell).Border(1, colHairline).Center().Children(func() {
				ui.Icon(c, iconChip).Size(24, 24).TextColor(tone)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, firmware.StageLabel(s.Phase)).FontSize(19).FontWeight(500)
				ui.Text(c, fallback(s.Message, "仅安装随应用提供的 Go Voice 固件。")).FontSize(13).LineHeight(1.5).TextColor(colSubtle).MaxLines(6)
				if s.Phase == "error" && s.ErrorCode != "" {
					// Helper codes are fixed identifiers, never device or path data.
					ui.Text(c, fmt.Sprintf("错误码 %s · 失败阶段：%s", s.ErrorCode, firmware.StageLabel(s.FailedStage))).
						FontSize(12).TextColor(colRed).FontFeatures("tnum").Selectable()
				}
			})
		})
		if s.Busy() || s.Phase == "complete" {
			ui.Column(c).Gap(8).Children(func() {
				ui.Progress(c, float64(s.Percent)/100).Label("当前阶段进度")
				ui.Text(c, fmt.Sprintf("当前阶段进度 %d%%", s.Percent)).FontSize(12).TextColor(colMuted).FontFeatures("tnum")
			})
		}
	})

	group(c, "versions", "固件版本", nil, func() {
		kv(c, "设备", fallback(v.Snapshot.Firmware, "未读取"))
		kv(c, "随附", fallback(v.Snapshot.BundledFirmware, "未知"))
	})
	status, warn := firmwareStatus(v.Snapshot.Firmware, v.Snapshot.BundledFirmware)
	notice(c, "fw-status", status, warn)

	notice(c, "lock", "安装前会断开当前蓝牙设备；写入开始后无法取消。写入、验证和重启期间请勿拔线或退出应用。", true)
	notice(c, "cable", "请使用可靠的 USB 数据线，并确认选中的设备。不显示或保存 USB 序列号；USB 选择不证明设备身份，也不会自动绑定蓝牙设备。", false)

	group(c, "usb", "USB 设备", func() {
		if smallPill(c, "usb-scan", "扫描 USB 设备", pillSecondary, nil, !s.CanScan()) {
			v.Actions.FirmwareScan()
		}
	}, func() {
		if len(s.Devices) == 0 {
			row(c, "usb-empty", func() {
				ui.Text(c, "未发现兼容设备。连接 FoloToy AI Passport 后点击“扫描 USB 设备”。").FontSize(13).TextColor(colMuted)
			})
		}
		for i, d := range s.Devices {
			row(c, fmt.Sprintf("usb-%d", i), func() {
				// Helper names and port paths can contain serial numbers: never render them.
				ui.Icon(c, iconChip).Size(16, 16).TextColor(pick(d.ID == s.Selected, colOrange, colMuted))
				ui.Text(c, fmt.Sprintf("兼容 USB 设备 %d", i+1)).FontSize(14).Grow(1)
				if d.ID == s.Selected {
					ui.Badge(c, "已选择").Background(colOrangeSoft).TextColor(colOrange)
				} else if s.Phase == "ready" {
					if smallPill(c, "usb-select", "选择此设备", pillSecondary, nil, false) {
						v.Actions.FirmwareSelect(d.ID)
					}
				}
			})
		}
	})
	if len(s.Devices) > 1 {
		notice(c, "many", "存在多个兼容设备。为避免选错，请仅连接要安装固件的设备后重新扫描。", true)
	}
	if v.Snapshot.SpeechActive {
		notice(c, "speech", "语音会话进行中，请先完成或取消本次语音。", true)
	}
	ui.Row(c).Gap(16).Children(func() {
		ui.Box(c).Grow(1).Height(1).Background(colRule)
		if s.Phase == "complete" {
			if pill(c, "goto-device", "前往设备页选择刚重启的 FoloToy AI Passport", pillSecondary, nil, iconArrow, false) {
				v.Page = 0
			}
		}
		if pill(c, "install", "确认安装随附固件", pillPrimary, iconDownload, nil, !s.CanInstall(v.Snapshot.SpeechActive)) {
			v.Actions.FirmwareInstall()
		}
	})
}

func (v *View) vocabulary(c *ui.Context) {
	tabs := []string{fmt.Sprintf("词条 · %d", len(v.Snapshot.Terms)), fmt.Sprintf("替换词 · %d", len(v.Snapshot.Replacements)), "Jev 判断", "公共词库"}
	ui.Row(c).Gap(10).Children(func() {
		segmented(c, "vocab-tabs", &v.vocabTab, tabs)
		ui.Box(c).Grow(1)
		if smallPill(c, "import-vocab", "导入", pillSecondary, iconDownload, false) {
			v.Actions.ImportVocabulary()
		}
		if smallPill(c, "export-vocab", "导出", pillSecondary, nil, false) {
			v.Actions.ExportVocabulary()
		}
	})
	if v.Snapshot.VocabFeedback != "" {
		feedback(c, v.Snapshot.VocabFeedback, v.Snapshot.VocabFeedbackOK)
	}
	switch v.vocabTab {
	case 1:
		v.replacements(c)
		return
	case 2:
		v.judge(c)
		return
	case 3:
		v.wordlists(c)
		return
	}
	notice(c, "public-summary", publicSummary(v.Snapshot.Packs), false)
	notice(c, "vocab", "词条帮助 Apple 本地识别、分词和 AI 润色认出专有名词：下一次语音优先选取锁定和高权重词条，再用替换目标补齐 Top 100；锁定词不能被 AI 自动删除。", false)
	title := "添加词条"
	if v.editing != "" {
		title = "编辑词条"
	}
	group(c, "editor", title, nil, func() {
		row(c, "input", func() {
			ui.TextInput(c, &v.term).Placeholder("术语 / 标准写法").Grow(1).Label("术语")
			ui.Row(c).Gap(8).Children(func() {
				ui.Icon(c, iconLock).Size(14, 14).TextColor(colMuted)
				ui.Text(c, "锁定").FontSize(13).TextColor(colSubtle)
				ui.Switch(c, &v.locked).Label("锁定")
			})
			if v.editing != "" {
				if smallPill(c, "cancel-edit", "取消", pillGhost, nil, false) {
					v.term, v.editing, v.locked = "", "", false
				}
			}
			if smallPill(c, "save-term", pick2(v.editing != "", "更新", "保存"), pillPrimary, iconPlus, strings.TrimSpace(v.term) == "") {
				v.Actions.Put(v.editing, vocabulary.Term{Term: v.term, Source: "manual", Weight: 50, Locked: v.locked})
				v.term, v.editing = "", ""
			}
		})
	})
	group(c, "terms", fmt.Sprintf("词条 · %d", len(v.Snapshot.Terms)), nil, func() {
		if len(v.Snapshot.Terms) == 0 {
			row(c, "none", func() {
				ui.Text(c, "还没有个人词条。添加专有名词、产品名或人名，让纠错更贴合你的用语。").FontSize(13).TextColor(colMuted)
			})
		}
		for _, t := range v.Snapshot.Terms {
			row(c, t.NormalizedTerm, func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, t.Term).FontSize(14).FontWeight(500).SingleLine()
						if t.Locked {
							ui.Icon(c, iconLock).Size(12, 12).TextColor(colOrange).Label("已锁定")
						}
					})
					ui.Text(c, fmt.Sprintf("%s · 权重 %d", sourceLabel(t.Source), t.Weight)).FontSize(11.5).TextColor(colMuted).FontFeatures("tnum")
				})
				if smallPill(c, "edit", "编辑", pillGhost, nil, false) {
					v.term = t.Term
					v.editing = t.Term
					v.locked = t.Locked
				}
				if smallPill(c, "delete", "删除", pillDanger, nil, false) {
					v.Actions.Delete(t.Term)
				}
			})
		}
	})
}

// replacements is the replacement tab of the vocabulary page: what
// recognition writes, and what it becomes before anything else sees it.
func (v *View) replacements(c *ui.Context) {
	notice(c, "replace", "出现原词时仍在本地直接替换，不经过 AI；英文只匹配完整单词。最终文字还会按词边界匹配被空格拆开的中文原词（如“施 玉 宝”），不跨标点和换行；关闭 AI 润色和 Jev 时也生效。替换目标也会补充进下次语音的词表。", false)
	title := "添加替换词"
	if v.editingFrom != "" {
		title = "编辑替换词"
	}
	group(c, "replace-editor", title, nil, func() {
		row(c, "replace-input", func() {
			ui.TextInput(c, &v.from).Placeholder("原词：识别出的错误写法，如 施玉宝").Grow(1).Basis(0).Label("原词")
			ui.Text(c, "→").FontSize(15).TextColor(colMuted).Shrink(0)
			ui.TextInput(c, &v.to).Placeholder("替换词：正确写法，如 私域宝").Grow(1).Basis(0).Label("替换词")
			if v.editingFrom != "" {
				if smallPill(c, "cancel-replace", "取消", pillGhost, nil, false) {
					v.from, v.to, v.editingFrom = "", "", ""
				}
			}
			empty := strings.TrimSpace(v.from) == "" || strings.TrimSpace(v.to) == ""
			if smallPill(c, "save-replace", pick2(v.editingFrom != "", "更新", "保存"), pillPrimary, iconPlus, empty) {
				v.Actions.PutReplacement(v.editingFrom, vocabulary.Replacement{From: v.from, To: v.to})
				v.from, v.to, v.editingFrom = "", "", ""
			}
		})
	})
	group(c, "replacements", fmt.Sprintf("替换词 · %d", len(v.Snapshot.Replacements)), nil, func() {
		if len(v.Snapshot.Replacements) == 0 {
			row(c, "none", func() {
				ui.Text(c, "还没有替换词。某个词总被识别错时，把错误写法和正确写法添加到这里。").FontSize(13).TextColor(colMuted)
			})
		}
		for _, r := range v.Snapshot.Replacements {
			row(c, "replace-"+vocabulary.Normalize(r.From), func() {
				ui.Row(c).Grow(1).MinWidth(0).Gap(10).Children(func() {
					ui.Text(c, r.From).FontSize(14).TextColor(colSubtle).SingleLine().Shrink(1)
					ui.Text(c, "→").FontSize(14).TextColor(colMuted).Shrink(0)
					ui.Text(c, r.To).FontSize(14).FontWeight(500).SingleLine().Shrink(1)
				})
				if smallPill(c, "edit-replace", "编辑", pillGhost, nil, false) {
					v.from, v.to, v.editingFrom = r.From, r.To, r.From
				}
				if smallPill(c, "delete-replace", "删除", pillDanger, nil, false) {
					v.Actions.DeleteReplacement(r.From)
				}
			})
		}
	})
}

func sourceLabel(s string) string {
	switch s {
	case "manual":
		return "手动"
	case "correction":
		return "纠错"
	case "ai":
		return "AI"
	case "imported":
		return "导入"
	}
	return s
}

// recognition is the page of the speech recognition engine: Apple's local
// SpeechAnalyzer, or an online recognition provider.
func (v *View) recognition(c *ui.Context) {
	group(c, "asr-engine", "识别引擎", nil, func() {
		row(c, "asr-engine", func() {
			label(c, "引擎", asrHelp(v.ASR))
			v.asrSelect(c)
		})
	})
	s := v.Snapshot
	if !v.ASR.Online() {
		notice(c, "asr-local", "本地识别不联网，需要“语音识别”权限和已下载的中文语音模型；词库 Top 100 作为识别提示。", false)
	} else {
		when := "实时"
		if !v.ASR.Streaming() {
			when = "在松开按键后整段"
		}
		to := v.ASR.Provider()
		if host := v.ASR.Host(); host != "" {
			to += "（" + host + "）"
		}
		notice(c, "asr-online", "在线识别：每次语音的音频会"+when+"发送到"+to+"转写，不发送词库，也不在本机保存音频。连接失败时本次语音结束，不会自动改用本地识别。", true)
		group(c, "asr-key", v.ASR.Provider(), nil, func() {
			if models := asrModels(v.ASR.Engine); models != nil {
				v.asrModelRow(c, models)
			}
			if v.ASR.Engine == asr.EngineOmni {
				v.omniFields(c)
			}
			field(c, "API key", func() {
				ui.TextInput(c, &v.asrSecret).Password().Placeholder(v.keyPlaceholder(asrAccount(v.ASR), "仅保存到 macOS 钥匙串")).Grow(1).Label(v.ASR.Provider() + " API key")
			})
		})
	}
	ui.Row(c).Gap(12).Wrap().GapY(12).Children(func() {
		if v.ASR.Online() && pill(c, "asr-delete-key", "删除凭据", pillDanger, nil, nil, false) {
			v.Actions.DeleteASRKey()
		}
		ui.Box(c).Grow(1).MinWidth(24).Height(1).Background(colRule)
		if v.ASR.Online() && pill(c, "asr-test", pickText(s.ASRTesting, "正在测试…", "测试连接"), pillSecondary, nil, nil, s.ASRTesting) {
			v.Actions.TestASR(v.ASR, v.asrSecret)
		}
		if pill(c, "asr-save", "保存设置", pillPrimary, nil, iconArrow, false) {
			v.Actions.ConfigureASR(v.ASR, v.asrSecret)
			v.asrSecret = ""
		}
	})
	if s.ASRFeedback != "" && !s.ASRTesting && s.ASRFeedbackFor == asr.Normalize(v.ASR) {
		feedback(c, s.ASRFeedback, s.ASRFeedbackOK)
	}
}

// asrSelect is the drop-down of the recognition engines, a capsule like the
// segmented controls.
func (v *View) asrSelect(c *ui.Context) {
	engines := asrEngines()
	current := engines[0]
	for _, e := range engines {
		if e.ID == asr.Normalize(v.ASR).Engine {
			current = e
		}
	}
	id := current.ID
	sel := ui.SelectBase(c.Key("asr-select"), &id)
	sel.Trigger.Height(32).PaddingX(14).Gap(10).Radius(999).Shrink(0).Border(1, colHairline).Label("选择识别引擎").
		Background(pick(sel.Trigger.Hovered(), colPill, colWell)).Children(func() {
		ui.Text(c, current.Label).FontSize(12.5).FontWeight(500).SingleLine()
		ui.Icon(c, iconChevron).Size(12, 12).TextColor(colMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Padding(4).Gap(2).Radius(14).Background(colCard).Border(1, colHairline).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.45))
		for _, ch := range engines {
			item := sel.Item(ch.ID).Height(30).PaddingX(10).Gap(8).Radius(10)
			if item.Highlighted() {
				item.Background(colPillHover)
			}
			item.Children(func() {
				ui.Icon(c, iconCheck).Size(12, 12).TextColor(pick(ch.ID == current.ID, colText, ui.Transparent))
				ui.Text(c, ch.Label).FontSize(12.5).SingleLine()
			})
		}
	})
	if sel.Trigger.Changed() {
		for _, ch := range engines {
			if ch.ID == id {
				v.chooseASR(ch.Config)
			}
		}
	}
}

// asrEngines are the entries of the engine drop-down: one per provider, in
// the order of asr.Choices. The models of a provider share its API key, so
// they are chosen in its card rather than listed as separate engines.
func asrEngines() []asr.Choice {
	var out []asr.Choice
	for _, ch := range asr.Choices {
		if i := slices.IndexFunc(out, func(o asr.Choice) bool { return o.ID == ch.Config.Engine }); i >= 0 {
			out[i].Label = ch.Config.Provider()
			continue
		}
		ch.ID = ch.Config.Engine
		out = append(out, ch)
	}
	return out
}

// asrModels are the choices of engine when it offers more than one.
func asrModels(engine string) []asr.Choice {
	var out []asr.Choice
	for _, ch := range asr.Choices {
		if ch.Config.Engine == engine {
			out = append(out, ch)
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// asrModelRow chooses among the models of the drafted provider; its key
// field below serves them all.
func (v *View) asrModelRow(c *ui.Context, models []asr.Choice) {
	labels := make([]string, len(models))
	i := 0
	for j, m := range models {
		_, labels[j], _ = strings.Cut(m.Label, " · ")
		if m.Config == v.ASR {
			i = j
		}
	}
	row(c, "asr-model", func() {
		label(c, "模型", "同一服务商的模型共用下方的 API key，切换模型不用重新填写。")
		if segmented(c, "asr-model-seg", &i, labels) {
			v.chooseASR(models[i].Config)
		}
	})
}

// chooseASR drafts the engine next. A model of the same provider keeps the
// typed key; another provider never gets it, but brings back what was
// drafted for it before: its typed key, model and Omni address.
func (v *View) chooseASR(next asr.Config) {
	if next.Engine == v.ASR.Engine {
		if next.Engine != asr.EngineOmni {
			v.ASR = next
		}
		return
	}
	if v.asrSecrets == nil {
		v.asrSecrets, v.asrDrafts = map[string]string{}, map[string]asr.Config{}
	}
	v.asrSecrets[v.ASR.Engine], v.asrDrafts[v.ASR.Engine] = v.asrSecret, v.ASR
	v.asrSecret = v.asrSecrets[next.Engine]
	if draft, ok := v.asrDrafts[next.Engine]; ok {
		next = draft
	} else if v.Snapshot.ASR.Engine == next.Engine {
		next = v.Snapshot.ASR
	}
	v.ASR = next
	v.syncOmni()
}

// keyPlaceholder is the hint of an empty API key field: that account
// already holds a key, which an empty field keeps, or else none. It asks
// the app once to look account up; "" is an account not complete yet.
func (v *View) keyPlaceholder(account, none string) string {
	if account == "" {
		return none
	}
	saved, known := v.Snapshot.SavedKeys[account]
	if !known && !v.keyChecks[account] && v.Actions.CheckKey != nil {
		if v.keyChecks == nil {
			v.keyChecks = map[string]bool{}
		}
		v.keyChecks[account] = true
		v.Actions.CheckKey(account)
	}
	if saved {
		return "已保存在钥匙串，留空即沿用；填写新 key 会替换"
	}
	return none
}

// asrAccount is the keychain account of the key of an online engine, or ""
// while the draft cannot be saved, such as Omni without a workspace.
func asrAccount(c asr.Config) string {
	c = asr.Normalize(c)
	if !c.Online() || c.Validate() != nil {
		return ""
	}
	return asr.KeyAccount(c)
}

// endpointAccount is the keychain account of the refinement key: the
// endpoint itself, once it is one.
func endpointAccount(endpoint string) string {
	if refinement.CheckRemote(refinement.Config{Endpoint: endpoint, Model: "-"}) != nil {
		return ""
	}
	return endpoint
}

// syncOmni drafts the region and workspace from the Omni address.
func (v *View) syncOmni() {
	v.omniWorkspace, v.omniRegion = "", 0
	if v.ASR.Engine != asr.EngineOmni || strings.TrimSpace(v.ASR.Endpoint) == "" {
		return
	}
	ws, region, ok := asr.ParseOmniEndpoint(v.ASR.Endpoint)
	if !ok {
		v.omniRegion = len(asr.OmniRegions)
		return
	}
	v.omniWorkspace = ws
	for i, r := range asr.OmniRegions {
		if r.ID == region {
			v.omniRegion = i
		}
	}
}

// omniFields is the form of Alibaba Cloud Omni Realtime: a region and the
// workspace ID make its address, or the address is typed in full.
func (v *View) omniFields(c *ui.Context) {
	custom := len(asr.OmniRegions)
	labels := []string{}
	for _, r := range asr.OmniRegions {
		labels = append(labels, r.Label)
	}
	labels = append(labels, "自定义")
	row(c, "asr-region", func() {
		label(c, "地域", "每个百炼工作空间在各地域有独立的接入地址；选“自定义”可直接填写完整地址。")
		if segmented(c, "asr-region-seg", &v.omniRegion, labels) && v.omniRegion != custom {
			v.ASR.Endpoint = asr.OmniEndpoint(strings.TrimSpace(v.omniWorkspace), asr.OmniRegions[v.omniRegion].ID)
		}
	})
	if v.omniRegion == custom {
		field(c, "WebSocket 地址", func() {
			ui.TextInput(c, &v.ASR.Endpoint).Placeholder("wss://{工作空间 ID}.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime").Grow(1).Label("WebSocket 地址")
		})
	} else {
		field(c, "工作空间", func() {
			if ui.TextInput(c, &v.omniWorkspace).Placeholder("百炼控制台中的工作空间 ID").Grow(1).Label("工作空间 ID").Changed() {
				v.ASR.Endpoint = asr.OmniEndpoint(strings.TrimSpace(v.omniWorkspace), asr.OmniRegions[v.omniRegion].ID)
			}
		})
		if v.ASR.Endpoint != "" {
			field(c, "连接地址", func() {
				ui.Text(c, v.ASR.Endpoint).FontSize(12).TextColor(colMuted).SingleLine().Grow(1).MinWidth(0)
			})
		}
	}
	field(c, "模型", func() {
		ui.TextInput(c, &v.ASR.Model).Placeholder(asr.Normalize(asr.Config{Engine: asr.EngineOmni}).Model).Grow(1).Label("识别模型")
	})
}

// asrHelp says what the chosen engine does with the audio.
func asrHelp(c asr.Config) string {
	switch {
	case c.Engine == asr.EngineOmni:
		return "WebSocket 实时上传音频，松开按键后提交并等待最终转写；只识别你说的话，不生成对话回答。"
	case c.Engine == asr.EngineDoubao:
		return "火山引擎豆包双向流式识别：边说边出字，松开按键后返回最终结果；小时版按识别时长计费，并发版按同时可用的并发路数计费，需与该 API key 在火山引擎控制台开通的方式一致。API key 在新版语音控制台获取。"
	case c.Streaming():
		return "在线流式识别：边说边出字，松开按键后返回最终结果。"
	case c.Online():
		return "整段识别：说话时不显示文字，松开按键后上传整段音频，再返回识别结果。"
	}
	return "默认。Apple SpeechAnalyzer 在本机识别，音频不离开 Mac。"
}

func (v *View) refinement(c *ui.Context) {
	group(c, "engine", "润色引擎", nil, func() {
		row(c, "engine", func() {
			label(c, "引擎", "默认 Apple 本地模型；失败不会自动切换到远端。")
			if i := indexOf(engines, v.Config.Engine); segmented(c, "engine-seg", &i, engineLabels) {
				v.Config.Engine = engines[i]
			}
		})
		row(c, "mode", func() {
			label(c, "模式", modeHelp(v.Config.Mode))
			if i := indexOf(modes, v.Config.Mode); segmented(c, "mode-seg", &i, modeLabels) {
				v.Config.Mode = modes[i]
			}
		})
	})
	remote := v.Config.Engine == "remote"
	if remote {
		notice(c, "remote", "显式远端模式：最终转写和词库将发送到所填 HTTPS 端点；音频不发送。", true)
		v.remoteSettings(c)
	}
	ui.Row(c).Gap(12).Wrap().GapY(12).Children(func() {
		if remote {
			if pill(c, "delete-cred", "删除凭据", pillDanger, nil, nil, false) {
				v.Actions.DeleteCredential(v.Config.Endpoint)
			}
			ui.Box(c).Grow(1).MinWidth(24).Height(1).Background(colRule)
			if pill(c, "test-remote", pickText(v.Snapshot.Testing, "正在测试…", "测试连接"), pillSecondary, nil, nil, v.Snapshot.Testing) {
				v.Actions.TestRemote(v.Config, v.secret)
			}
			if pill(c, "save-cred", "保存到钥匙串", pillSecondary, iconKey, nil, false) {
				v.Actions.Credential(v.Config.Endpoint, v.secret)
				v.clearSecret()
			}
		} else {
			ui.Box(c).Grow(1)
		}
		if pill(c, "save-config", "保存设置", pillPrimary, nil, iconArrow, false) {
			v.Actions.Configure(v.Config, v.secret)
			v.clearSecret()
		}
	})
	s := v.Snapshot
	if s.Feedback != "" && !s.Testing && s.FeedbackFor == v.Config {
		feedback(c, s.Feedback, s.FeedbackOK)
	}
}

// remoteSettings holds the fields that only an OpenAI-compatible endpoint
// uses; they are hidden while the local engine is chosen.
func (v *View) remoteSettings(c *ui.Context) {
	group(c, "endpoint", "OpenAI 兼容接口", nil, func() {
		field(c, "接口", func() {
			i := indexOf(apis, v.Config.API)
			if segmented(c, "api-seg", &i, apiLabels) {
				v.Config.API = apis[i]
			}
			ui.Text(c, "POST 端点 + "+apiPaths[i]).FontSize(12).TextColor(colMuted).SingleLine().Grow(1).MinWidth(0)
		})
		field(c, "端点", func() {
			ui.TextInput(c, &v.Config.Endpoint).Placeholder("https://provider.example/v1").Grow(1).Label("端点")
		})
		field(c, "模型", func() { ui.TextInput(c, &v.Config.Model).Placeholder("模型名称").Grow(1).Label("模型") })
		field(c, "API key", func() {
			ui.TextInput(c, &v.secret).Password().Placeholder(v.keyPlaceholder(endpointAccount(v.Config.Endpoint), "仅保存到 macOS 钥匙串")).Grow(1).Label("API key")
		})
		row(c, "stream", func() {
			label(c, "流式输出", "以流式（SSE）请求并收齐全文：润色结果仍在校验后整体插入，不会更快；只用于只接受流式请求的服务或模型。")
			ui.Switch(c, &v.Config.Stream).Label("以流式请求")
		})
	})
	group(c, "routing", "供应商路由（可选）", nil, func() {
		row(c, "routing-about", func() {
			label(c, "指定供应商", "同一模型有多个供应商的网关（如蚂蚁数科 DTMaaS）可在请求里指定；填模型详情页的供应商标识，如 antdigital，多个用逗号分隔。全部留空则不传。")
		})
		field(c, "仅允许", func() {
			ui.TextInput(c, &v.Config.ProviderOnly).Placeholder("only：只使用这些供应商").Grow(1).Label("仅允许的供应商")
		})
		field(c, "优先顺序", func() {
			ui.TextInput(c, &v.Config.ProviderOrder).Placeholder("order：按此顺序优先选择").Grow(1).Label("供应商优先顺序")
		})
		field(c, "排除", func() {
			ui.TextInput(c, &v.Config.ProviderIgnore).Placeholder("ignore：不使用这些供应商").Grow(1).Label("排除的供应商")
		})
		row(c, "routing-strict", func() {
			label(c, "不用列表之外的供应商", "开启后传 allow_fallbacks: false：优先顺序里的供应商都不可用时不改用其他供应商。")
			ui.Switch(c, &v.Config.ProviderStrict).Label("不允许其他供应商")
		})
	})
}

// clearSecret forgets the typed API key once it was handed over to be
// saved; without an endpoint, which keys it, it stays for another try.
func (v *View) clearSecret() {
	if strings.TrimSpace(v.Config.Endpoint) != "" {
		v.secret = ""
	}
}

// feedback shows how the last action of the page ended: green when it
// succeeded, red when it needs attention.
func feedback(c *ui.Context, text string, ok bool) {
	fg, bg, glyph := colRed, colRedSoft, iconWarn
	if ok {
		fg, bg, glyph = colGreen, ui.RGBA(48, 209, 88, 0.12), iconCheck
	}
	ui.Row(c.Key("feedback")).Padding(12, 16).Gap(10).Radius(14).Background(bg).Children(func() {
		ui.Icon(c, glyph).Size(15, 15).TextColor(fg).Shrink(0).AlignSelf(ui.Start)
		ui.Text(c, text).FontSize(13).LineHeight(1.5).TextColor(colText).Grow(1).MinWidth(0).Selectable()
	})
}

// modeHelp says what a refinement mode changes; every mode keeps the ASR
// original when it fails, times out or strays from it.
func modeHelp(mode string) string {
	switch mode {
	case "off":
		return "不润色，直接插入识别结果；词库替换词仍然生效。"
	case "names":
		return "只按词库纠正专有名词，其他文字和标点不动。"
	case "structured":
		return "在轻度的基础上按上下文纠正错字、理顺病句，再分段或列点，不删减、不添加内容。"
	}
	return "按词库纠正专有名词，去掉“喂、嗯、那个”等语气词并修正标点，不改写内容。"
}

func pickText(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func label(c *ui.Context, title, detail string) {
	ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(14).FontWeight(500)
		ui.Text(c, detail).FontSize(12).TextColor(colMuted)
	})
}

func field(c *ui.Context, name string, input func()) {
	row(c, name, func() {
		ui.Text(c, name).FontSize(13).TextColor(colSubtle).Width(72).Shrink(0)
		input()
	})
}

// segmented is a capsule segmented control: a black track whose chosen
// segment is the reference's white primary capsule.
func segmented(c *ui.Context, key string, selected *int, labels []string) bool {
	seg := ui.SegmentedBase(c.Key(key), selected, len(labels))
	seg.Track.Padding(3).Gap(2).Radius(999).Background(colWell).Border(1, colHairline).Shrink(0).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Height(28).PaddingX(14).Radius(999).Label(l)
			switch {
			case i == *selected:
				s.Background(ui.Hex("#FFFFFF")).TextColor(colCanvas)
			case s.Hovered():
				s.Background(colPill).TextColor(colText)
			default:
				s.TextColor(colMuted)
			}
			s.Children(func() { ui.Text(c, l).FontSize(12.5).FontWeight(500).SingleLine() })
		}
	})
	return seg.Track.Changed()
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return 0
}

func fallback(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func firmwareStatus(device, bundled string) (string, bool) {
	switch firmware.Relate(device, bundled) {
	case firmware.Older:
		return "设备固件 " + device + " 低于随附 " + bundled + "，建议安装。", true
	case firmware.Equal:
		return "设备已是随附固件 " + bundled + "。", false
	case firmware.Newer:
		return "设备固件 " + device + " 比随附 " + bundled + " 更新。除非要重装随附版本，否则不必安装。", false
	default:
		if bundled == "" {
			return "无法读取随附固件版本。请重新安装 go-voice。", true
		}
		if device == "" {
			return "尚未通过蓝牙读取设备固件。随附版本 " + bundled + "。连接 FoloToy AI Passport 后可核对是否需要更新。", false
		}
		return "无法比较设备固件 " + device + " 与随附 " + bundled + "。", true
	}
}

func pick2(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
