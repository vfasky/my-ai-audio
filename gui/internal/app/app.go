package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updaterui "github.com/egoist/mygo/plugins/updater/native"
	nativeui "github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/device"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/hotkey"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/keychain"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type settings struct {
	Peripheral string            `json:"peripheral"`
	Refinement refinement.Config `json:"refinement"`
	ASR        asr.Config        `json:"asr"`
	Jev        jev.Config        `json:"jev"`
	Packs      wordlists.Config  `json:"vocabulary_packs"`
	// Hotkey is the modifier chord held to record with the Mac microphone,
	// or hotkey.Off.
	Hotkey string `json:"hotkey"`
}
type application struct {
	ctx               context.Context
	stop              context.CancelFunc
	window            *mygo.Window
	view              appui.View
	coordinator       device.Coordinator
	installer         *firmware.Process
	installState      firmware.State
	installPaths      firmware.Paths
	bluetooth, speech *bridge.Process
	config            settings
	store             vocabulary.Store
	directory         string
	subscribed        bool
	hello             *protocol.Control
	quitting          bool
	refineCancel      context.CancelFunc
	localReply        chan string
	terms             []string // recognition hints, at most 100
	correctionTerms   []string // complete vocabulary frozen when recording starts
	sessionPacks      wordlists.Config
	permissions       permission.Set
	// online recognizes the session onlineID when an online engine is
	// chosen; SpeechBridge still inserts its text.
	online   *asr.Session
	onlineID uint32
	// onboarded is set once the first complete permission report decided
	// whether to open the permissions page.
	onboarded bool
	// The dictation capsule: a panel shown without the keyboard during a
	// session, lingering briefly after one ends without inserting.
	hud       *mygo.Window
	hudShown  bool
	hudUntil  time.Time
	escapeSet bool
	tray      *mygo.Tray
	// atLogin: the system started go-voice at login, so it stays in the
	// menu bar unless the onboarding needs the window.
	atLogin bool
	// live is set on the UI thread when a session is active, so the tick
	// goroutine can skip posting while idle.
	live   atomic.Bool
	inPost bool
	paint  paintKind
}

type paintKind uint8

const (
	paintView paintKind = 1 << iota
	paintSession
	paintCatalog
)

// Run starts the app; icon is the packaged PNG app icon shown in the sidebar
// and tray the menu-bar template icon.
func Run(icon, tray []byte) {
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &application{ctx: ctx, stop: cancel, config: settings{Refinement: refinement.Default()}, permissions: permission.Set{}}
	// The interface is designed dark only; keep the window chrome in step.
	mygo.Theme.SetSource(mygo.ThemeDark)
	// A menu-bar app: no Dock icon (LSUIElement in mygo.json from launch),
	// and closing the window keeps it running in the menu bar.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	// Signed updates in MyGo's native update window: a daily background
	// check, release notes, download and relaunch. Only release builds with
	// the update feed and public key of mygo.json update themselves
	// (docs/updates.md); debug builds never do.
	mygo.Use(updaterui.New(updater.Options{Language: "zh-Hans", Icon: icon}))
	mygo.App.WhenReady(func() {
		if bitmap, err := nativeui.DecodeBitmap(icon); err == nil {
			a.view.AppIcon = bitmap
		}
		a.atLogin = mygo.App.WasOpenedAtLogin()
		a.initialize()
		a.window = mygo.NewWindow(mygo.WindowOptions{
			Title: "go-voice", Width: 1120, Height: 840, MinWidth: 920, MinHeight: 660, Hidden: a.atLogin,
			TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: appui.TitleBarHeight,
			BackgroundColor: "#0A0A0A",
			Content:         nativeui.View(a.view.Draw),
		})
		// Hide rather than close, except while quitting: a prevented close
		// would cancel the quit.
		a.window.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.window.Hide()
		})
		if !a.atLogin {
			mygo.App.Focus()
		}
		a.startTray(tray)
		// The capsule is drawn without a shadow, and the panel has none either:
		// macOS would trace one around the transparent window's alpha.
		a.hud = mygo.NewWindow(mygo.WindowOptions{
			Title: "go-voice dictation", Panel: true, Hidden: true, Frameless: true, Transparent: true,
			DisableResize: true, DisableShadow: true, Width: appui.HUDWidth, Height: appui.HUDHeight,
			Content: nativeui.View(a.view.DrawHUD),
		})
		a.hud.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			a.effects(a.coordinator.Cancel("已取消"))
			a.hideHUD()
		})
		a.startNative()
	})
	mygo.App.OnBeforeQuit(func(_ *mygo.QuitEvent) {
		a.quitting = true
		a.effects(a.coordinator.Cancel("Application exiting"))
		if a.installer != nil {
			a.installer.Close()
			select {
			case <-a.installer.Done():
			case <-time.After(600 * time.Millisecond):
			}
		}
		a.bindEscape(false)
		cancel()
		if a.bluetooth != nil {
			a.bluetooth.Close()
		}
		if a.speech != nil {
			a.speech.Close()
		}
	})
	_ = mygo.App.Run()
}

// startTray puts go-voice in the menu bar: settings, the update check, and
// quit.
func (a *application) startTray(icon []byte) {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           icon,
		IconIsTemplate: true,
		ToolTip:        "go-voice",
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.showMain() }},
			{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { a.checkForUpdates() }},
			mygo.Separator(),
			{Label: "退出 go-voice", Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
		}),
	})
	if err != nil {
		a.view.Snapshot.Status = "菜单栏图标不可用"
		return
	}
	a.tray = tray
}
func (a *application) showMain() {
	if a.window == nil {
		return
	}
	a.readLoginItem()
	a.window.Show()
	mygo.App.Focus()
}

// The system's login item of go-voice; tests replace them.
var (
	setOpenAtLogin = func(open bool) error { return mygo.App.SetOpenAtLogin(open) }
	openAtLogin    = func() bool { return mygo.App.OpenAtLogin() }
)

const loginItemsURL = "x-apple.systempreferences:com.apple.LoginItems-Settings.extension"

// setLoginItem adds go-voice to the login items, or removes it, then shows
// what the system now has: a new login item may wait for approval.
func (a *application) setLoginItem(open bool) {
	err := setOpenAtLogin(open)
	s := &a.view.Snapshot
	s.OpenAtLogin = openAtLogin()
	s.LoginApproval = open && !s.OpenAtLogin && loginNeedsApproval(err)
	s.LoginFeedback, s.LoginFeedbackOK = loginMessage(open, err)
	a.refresh()
}

// readLoginItem catches up with changes made in System Settings, such as
// allowing a login item that waited for approval.
func (a *application) readLoginItem() {
	s := &a.view.Snapshot
	open := openAtLogin()
	changed := s.OpenAtLogin != open
	s.OpenAtLogin = open
	if s.LoginApproval && s.OpenAtLogin {
		s.LoginApproval = false
		s.LoginFeedback, s.LoginFeedbackOK = loginMessage(true, nil)
		changed = true
	}
	if changed {
		a.mark(paintView)
	}
}

// loginNeedsApproval: MyGo registered the login item, but the user has to
// allow it in System Settings > General > Login Items.
func loginNeedsApproval(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Login Items")
}

// loginMessage says in the interface's language how a change of the login
// item ended.
func loginMessage(open bool, err error) (string, bool) {
	switch {
	case err == nil && open:
		return "已开启：下次登录 Mac 时 go-voice 会在菜单栏后台启动。", true
	case err == nil:
		return "已关闭登录时打开。", true
	case loginNeedsApproval(err):
		return "已添加到登录项，等待在系统设置中允许。", false
	case strings.Contains(err.Error(), "app bundle"):
		return "无法更改：请启动打包后的 go-voice.app，而不是直接运行可执行文件。", false
	}
	verb := "无法关闭"
	if open {
		verb = "无法开启"
	}
	return verb + "登录时打开：" + strings.TrimPrefix(err.Error(), "mygo: "), false
}

// checkForUpdates opens the update window, or says why this build cannot
// update itself.
func (a *application) checkForUpdates() {
	mygo.App.Focus()
	// Enabled probes the install location, and Message is modal: keep both
	// off the main thread.
	go func() {
		if mygo.Updater.Enabled() {
			updater.CheckForUpdates()
			return
		}
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageInfo,
			Title:   "检查更新",
			Message: "此版本无法自动更新",
			Detail:  updatesUnavailableDetail(mygo.App.Version(), mygo.IsDev()),
		})
	}()
}

// updatesUnavailableDetail explains why a build does not update itself:
// debug builds never do, and release builds need to write where they are
// installed.
func updatesUnavailableDetail(version string, development bool) string {
	if development || version == "" {
		return "开发版不会自动更新。请从 GitHub Releases 安装正式版。"
	}
	return "当前版本 " + version + "。go-voice 无法写入当前安装位置，请将它移到“应用程序”或 ~/Applications 后再检查更新。"
}
func (a *application) post(fn func()) {
	mygo.RunOnMain(func() {
		if a.quitting {
			return
		}
		a.inPost = true
		fn()
		a.inPost = false
		a.flush()
	})
}
func (a *application) mark(p paintKind) { a.paint |= p }
func (a *application) flush() {
	if a.paint == 0 {
		return
	}
	if a.paint&paintCatalog != 0 {
		a.syncCatalog()
		a.paint |= paintSession | paintView
	}
	if a.paint&paintSession != 0 {
		a.syncSession()
		a.paint |= paintView
	}
	if a.paint&paintView != 0 {
		a.present()
	}
	a.paint = 0
}
func (a *application) initialize() {
	a.installState = firmware.Initial()
	base, e := os.UserConfigDir()
	if e != nil {
		a.view.Snapshot.Status = "Application support unavailable"
		base = ""
	}
	a.directory = filepath.Join(base, "MyGo-GoVoice")
	vocabularyPath := filepath.Join(a.directory, "vocabulary.json")
	a.store, e = vocabulary.Load(vocabularyPath)
	if e != nil {
		a.view.Snapshot.Status = "Vocabulary load failed"
	} else if _, statErr := os.Stat(vocabularyPath); os.IsNotExist(statErr) {
		// First run: start from the vocabulary that ships with the app.
		a.store = vocabulary.Defaults()
	}
	if b, e := os.ReadFile(filepath.Join(a.directory, "settings.json")); e == nil && len(b) < 16384 {
		var s settings
		if json.Unmarshal(b, &s) == nil {
			a.config = s
		}
	}
	a.config.ASR = asr.Normalize(a.config.ASR)
	a.config.Jev = jev.Normalize(a.config.Jev)
	a.config.Hotkey = hotkey.Normalize(a.config.Hotkey)
	a.applyASR()
	a.readLoginItem()
	a.view.Actions = appui.Actions{
		FirmwareScan: func() {
			if a.installState.Scan() {
				a.sendInstaller(firmware.Request{Type: "scan"})
				a.refresh()
			}
		},
		FirmwareSelect:  func(id string) { a.installState.Select(id); a.refresh() },
		FirmwareInstall: a.installFirmware,
		Scan:            func() { a.sendBluetooth(map[string]any{"type": "scan"}) },
		Select: func(id string) {
			if a.installState.Busy() {
				return
			}
			a.effects(a.coordinator.Disconnect())
			a.subscribed = false
			a.hello = nil
			a.config.Peripheral = id
			a.saveSettings()
			a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": id})
			a.mark(paintSession)
			a.flush()
		},
		Forget: func() {
			a.effects(a.coordinator.Disconnect())
			a.config.Peripheral = ""
			a.subscribed = false
			a.hello = nil
			a.saveSettings()
			a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": nil})
			a.mark(paintSession)
			a.flush()
		},
		Cancel: func() { a.effects(a.coordinator.Cancel("已取消")) },
		Commit: func() {
			// The panel took the keyboard when clicked: hide it first so the
			// paste reaches the app the user dictated into.
			a.hideHUD()
			if a.refineCancel != nil {
				a.refineCancel()
				a.refineCancel = nil
			}
			a.localReply = nil
			a.effects(a.coordinator.Commit())
		},
		Warmup: func() {
			if !a.coordinator.Active {
				a.sendSpeech(map[string]any{"type": "warmup"})
			}
		},
		RequestPermission: a.requestPermission,
		OpenSettings: func(n permission.Name) {
			if e := mygo.Shell.OpenExternal(permission.SettingsURL(n)); e != nil {
				a.view.Snapshot.Status = "无法打开系统设置"
			}
		},
		Put: func(previous string, t vocabulary.Term) {
			if e := a.store.Update(previous, t); e != nil {
				a.view.Snapshot.Status = vocabularyError(e)
			} else {
				a.saveVocabulary()
			}
			a.refresh()
		},
		Delete: func(term string) { _ = a.store.Delete(term, false); a.saveVocabulary(); a.refresh() },
		PutReplacement: func(previous string, r vocabulary.Replacement) {
			if e := a.store.PutReplacement(previous, r); e != nil {
				a.view.Snapshot.Status = vocabularyError(e)
			} else {
				a.saveVocabulary()
			}
			a.refresh()
		},
		DeleteReplacement: func(from string) { a.store.DeleteReplacement(from); a.saveVocabulary(); a.refresh() },
		ImportVocabulary:  a.importVocabulary,
		Quit:              func() { mygo.App.Quit() },
		SetOpenAtLogin:    a.setLoginItem,
		OpenLoginItems: func() {
			if e := mygo.Shell.OpenExternal(loginItemsURL); e != nil {
				a.view.Snapshot.Status = "无法打开系统设置"
			}
		},
		ConfigurePacks:   a.configurePacks,
		SetHotkey:        a.setHotkey,
		ExportVocabulary: a.exportVocabulary,
		ConfigureASR:     a.configureASR,
		TestASR:          a.testASR,
		DeleteASRKey:     a.deleteASRKey,
		CheckKey:         a.checkKey,
		ConfigureJev:     a.configureJev,
		TestJev:          a.testJev,
		DeleteJevKey:     a.deleteJevKey,
		Configure: func(c refinement.Config, key string) {
			a.config.Refinement = c
			saved := a.writeSettings()
			if key == "" || saved != nil {
				msg, ok := settingsMessage(c, saved, false, nil)
				a.setFeedback(c, msg, ok)
				a.refresh()
				return
			}
			go func() {
				e := (keychain.Store{}).Set(ctxOrBackground(a.ctx), c.Endpoint, key)
				a.post(func() {
					if e == nil {
						a.noteKey(c.Endpoint, true)
					}
					msg, ok := settingsMessage(c, nil, true, e)
					a.setFeedback(c, msg, ok)
				})
			}()
		},
		Credential: func(endpoint, secret string) {
			c := a.view.Config
			if msg, ok := credentialPrecheck(endpoint, secret); !ok {
				a.setFeedback(c, msg, false)
				a.refresh()
				return
			}
			go func() {
				e := (keychain.Store{}).Set(ctxOrBackground(a.ctx), endpoint, secret)
				a.post(func() {
					if e != nil {
						a.setFeedback(c, "API key 存入钥匙串失败，请重试或检查钥匙串访问权限。", false)
					} else {
						a.noteKey(endpoint, true)
						a.setFeedback(c, "API key 已存入钥匙串。", true)
					}
				})
			}()
		},
		TestRemote: func(c refinement.Config, key string) {
			if a.view.Snapshot.Testing {
				return
			}
			a.view.Snapshot.Testing, a.view.Snapshot.Feedback = true, ""
			a.refresh()
			go func() {
				out, err := refinement.Test(ctxOrBackground(a.ctx), c, key, keychain.Store{})
				a.post(func() {
					a.view.Snapshot.Testing = false
					msg, ok := remoteTestMessage(out, err)
					a.setFeedback(c, msg, ok)
				})
			}()
		},
		DeleteCredential: func(endpoint string) {
			c := a.view.Config
			if strings.TrimSpace(endpoint) == "" {
				a.setFeedback(c, "请先填写端点：API key 按端点保存在钥匙串中。", false)
				a.refresh()
				return
			}
			go func() {
				e := (keychain.Store{}).Delete(ctxOrBackground(a.ctx), endpoint)
				a.post(func() {
					if e != nil {
						a.setFeedback(c, "没有删除：钥匙串中没有此端点的 API key，或无法访问钥匙串。", false)
					} else {
						a.noteKey(endpoint, false)
						a.setFeedback(c, "已从钥匙串删除此端点的 API key。", true)
					}
				})
			}()
		},
	}
	a.refresh()
}

// setFeedback shows the result of an action of the refinement page for the
// draft c it acted on.
func (a *application) setFeedback(c refinement.Config, msg string, ok bool) {
	s := &a.view.Snapshot
	s.Feedback, s.FeedbackOK, s.FeedbackFor = msg, ok, c
	a.mark(paintView)
}

// settingsMessage tells how saving the refinement settings c ended: the
// settings file, the typed API key when there was one, and what the remote
// engine still lacks.
func settingsMessage(c refinement.Config, saved error, typedKey bool, key error) (string, bool) {
	if saved != nil {
		return "保存设置失败：无法写入 ~/Library/Application Support/MyGo-GoVoice/。", false
	}
	msg := "设置已保存。"
	if typedKey {
		if key != nil {
			return "设置已保存，但 API key 存入钥匙串失败，请重试。", false
		}
		msg = "设置已保存，API key 已存入钥匙串。"
	}
	if c.Engine == "remote" && refinement.CheckRemote(c) != nil {
		return msg + "远端润色还需要以 https:// 开头的端点和模型名称。", false
	}
	return msg, true
}

// credentialPrecheck refuses to save a key without one or without the
// endpoint that keys it in the keychain.
func credentialPrecheck(endpoint, secret string) (string, bool) {
	switch {
	case strings.TrimSpace(endpoint) == "":
		return "请先填写端点：API key 按端点保存在钥匙串中。", false
	case secret == "":
		return "请先输入 API key。", false
	}
	return "", true
}

// remoteTestMessage tells the user how the connection test of the
// OpenAI-compatible interface ended, and what to check when it failed.
func remoteTestMessage(res refinement.TestResult, err error) (string, bool) {
	var status *refinement.StatusError
	switch {
	case err == nil:
		out := res.Text
		if r := []rune(out); len(r) > 60 {
			out = string(r[:60]) + "…"
		}
		thinking := "已请求关闭思考模式"
		if !res.ThinkingOff {
			thinking = "该服务不接受关闭思考的参数，按模型默认方式调用"
		}
		return fmt.Sprintf("连接成功（%.1f 秒，%s），润色示例：%s", res.Elapsed.Seconds(), thinking, out), true
	case errors.Is(err, refinement.ErrConfig):
		return "请填写以 https:// 开头的端点（不含查询参数）和模型名称。", false
	case errors.Is(err, refinement.ErrKey):
		return "没有可用的 API key：请在上方输入，或先保存到钥匙串。", false
	case errors.Is(err, context.DeadlineExceeded):
		return "连接超时（30 秒）：请检查端点地址和网络。", false
	case errors.Is(err, refinement.ErrResponse):
		return "已连接，但返回内容不符合所选接口格式：请确认接口类型（Chat Completions 或 Responses）。", false
	case errors.As(err, &status):
		switch code := status.Code; {
		case code == 401 || code == 403:
			return fmt.Sprintf("API key 无效或没有权限（HTTP %d）。", code), false
		case code == 404:
			return "找不到接口或模型（HTTP 404）：请检查端点是否包含 /v1 等版本路径，以及模型名称。", false
		case code == 429:
			return "请求过于频繁或额度不足（HTTP 429）。", false
		case code >= 500:
			return fmt.Sprintf("服务端出错（HTTP %d），请稍后再试。", code), false
		default:
			return fmt.Sprintf("服务拒绝了请求（HTTP %d）：请检查模型名称和接口类型。", code), false
		}
	}
	return "无法连接到端点：请检查地址、网络或代理设置。", false
}

func ctxOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
func (a *application) saveSettings() {
	if a.writeSettings() != nil {
		a.view.Snapshot.Status = "保存设置失败"
	}
}

// writeSettings replaces settings.json atomically.
func (a *application) writeSettings() error {
	if e := os.MkdirAll(a.directory, 0700); e != nil {
		return e
	}
	b, _ := json.Marshal(a.config)
	tmp := filepath.Join(a.directory, "settings.json.tmp")
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(a.directory, "settings.json"))
}
func (a *application) saveVocabulary() {
	if e := a.store.Save(filepath.Join(a.directory, "vocabulary.json")); e != nil {
		a.view.Snapshot.Status = "Vocabulary save failed"
	}
}
func (a *application) refresh() {
	a.paint = paintCatalog | paintSession | paintView
	a.flush()
}
func (a *application) syncCatalog() {
	s := &a.view.Snapshot
	s.Terms = append([]vocabulary.Term(nil), a.store.Terms...)
	s.Packs = a.config.Packs
	s.Replacements = append([]vocabulary.Replacement(nil), a.store.Replacements...)
	s.Permissions = make(permission.Set, len(a.permissions))
	for n, st := range a.permissions {
		s.Permissions[n] = st
	}
}
func (a *application) syncSession() {
	s := &a.view.Snapshot
	s.Install = a.installState
	s.SpeechActive = a.coordinator.Active
	c := &a.coordinator
	s.Selected = a.config.Peripheral
	s.Ready = c.Ready
	s.Phase = c.Phase
	s.Meter = c.Meter
	s.Partial = c.Partial
	s.Original = c.Original
	s.Refined = c.Refined
	s.Config = a.config.Refinement
	s.ASR = a.config.ASR
	s.Jev = a.config.Jev
	s.Hotkey = a.config.Hotkey
	s.CanCommit = c.CanCommit()
	if c.Status != "" {
		s.Status = c.Status
	}
	a.live.Store(c.Active)
}
func (a *application) present() {
	if a.window != nil && a.window.IsVisible() {
		// A live dictation capsule does not need to rebuild hidden settings pages.
		sessionOnly := a.paint&paintCatalog == 0 && a.hudShown && a.view.Page != 0
		if !sessionOnly {
			a.window.Invalidate()
		}
	}
	a.syncHUD()
}

const hudLinger = 1500 * time.Millisecond

// syncHUD shows the capsule while a session records and processes, hides it
// before insertion, and keeps a cancelled or failed session's reason briefly.
func (a *application) syncHUD() {
	if a.hud == nil {
		return
	}
	c := &a.coordinator
	switch {
	case c.Active && c.Phase != "inserting":
		a.hudUntil = time.Time{}
		a.showHUD()
		a.bindEscape(true)
	case a.hudShown && c.Active:
		a.hideHUD()
	case a.hudShown:
		a.bindEscape(false)
		if a.hudUntil.IsZero() {
			a.hudUntil = time.Now().Add(hudLinger)
			deadline := a.hudUntil
			time.AfterFunc(hudLinger+50*time.Millisecond, func() {
				a.post(func() {
					if !a.hudShown || a.hudUntil.IsZero() || a.hudUntil != deadline || time.Now().Before(a.hudUntil) {
						return
					}
					a.hideHUD()
				})
			})
		} else if time.Now().After(a.hudUntil) {
			a.hideHUD()
		}
	}
	if a.hudShown {
		a.hud.Invalidate()
	}
}

// showHUD shows the capsule on the display with the pointer.
func (a *application) showHUD() {
	if a.hud == nil || a.hudShown {
		return
	}
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	a.hud.SetPosition(d.WorkArea.X+(d.WorkArea.Width-appui.HUDWidth)/2, d.WorkArea.Y+d.WorkArea.Height-appui.HUDHeight-8)
	a.hud.ShowInactive()
	a.hudShown = true
}

// flashHUD shows why the hotkey did not start, for as long as a failed
// session's reason.
func (a *application) flashHUD(reason string) {
	a.coordinator.Status = reason
	a.view.Snapshot.Status = reason
	a.hudUntil = time.Time{}
	a.showHUD()
	a.mark(paintSession)
}

func (a *application) hideHUD() {
	a.bindEscape(false)
	a.hudUntil = time.Time{}
	if a.hud != nil && a.hudShown {
		a.hud.Hide()
	}
	a.hudShown = false
}

// bindEscape claims Escape system-wide only while a session runs, so the
// capsule cancels it without taking the keyboard from the target app.
func (a *application) bindEscape(on bool) {
	switch {
	case on && !a.escapeSet:
		a.escapeSet = mygo.GlobalShortcut.Register("Escape", func() {
			a.post(func() { a.effects(a.coordinator.Cancel("已取消")) })
		}) == nil
	case !on && a.escapeSet:
		mygo.GlobalShortcut.Unregister("Escape")
		a.escapeSet = false
	}
}
func nativePath(name string) string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "..", "Resources", "native", name+".app", "Contents", "MacOS", name)
}
func (a *application) startNative() {
	a.startInstaller()
	var e error
	a.speech, e = bridge.Start(a.ctx, nativePath("SpeechBridge"), func(event bridge.Event) { a.post(func() { a.onSpeech(event) }) })
	if e != nil {
		a.view.Snapshot.Status = "SpeechBridge failed to launch"
		return
	}
	a.bluetooth, e = bridge.Start(a.ctx, nativePath("BluetoothHelper"), func(event bridge.Event) { a.post(func() { a.onBluetooth(event) }) })
	if e != nil {
		a.view.Snapshot.Status = "BluetoothHelper failed to launch"
		return
	}
	if a.config.Peripheral != "" {
		a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": a.config.Peripheral})
	}
	a.sendSpeech(map[string]any{"type": "warmup"})
	a.sendSpeech(map[string]any{"type": "hotkey", "text": a.config.Hotkey})
	a.queryPermissions()
	go func() {
		// Grants change in System Settings without telling the app: re-read
		// them while any is missing, so the onboarding updates on its own.
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.post(func() {
					a.readLoginItem()
					micMissing := hotkey.Enabled(a.config.Hotkey) && a.permissions[permission.Microphone] != permission.Granted
					if len(a.permissions.Missing()) > 0 || a.permissions[permission.Automation] != permission.Granted || micMissing {
						a.queryPermissions()
					}
				})
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case now := <-ticker.C:
				if !a.live.Load() {
					continue
				}
				a.post(func() {
					if !a.coordinator.Active {
						a.live.Store(false)
						return
					}
					if e := a.coordinator.Tick(now); len(e) > 0 {
						a.effects(e)
					}
				})
			}
		}
	}()
}
func (a *application) startInstaller() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	a.installPaths = firmware.BundlePaths(exe)
	a.view.Snapshot.BundledFirmware = firmware.ReadVersion(a.installPaths.Manifest)
	p, err := firmware.Start(a.ctx, a.installPaths.Helper)
	if err != nil {
		return
	}
	a.installer = p
	go a.watchInstaller(p)
}

func (a *application) watchInstaller(p *firmware.Process) {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	timeout := timer.C
	var mu sync.Mutex
	var queued []firmware.Event
	scheduled := false
	apply := func() {
		mu.Lock()
		batch := queued
		queued = nil
		scheduled = false
		mu.Unlock()
		if a.installer != p {
			return
		}
		for _, event := range batch {
			if a.installState.Apply(event) {
				a.view.Snapshot.Devices = nil
				a.sendBluetooth(map[string]any{"type": "scan"})
			}
		}
		a.mark(paintSession)
	}
	enqueue := func(e firmware.Event) {
		mu.Lock()
		queued = append(queued, e)
		if scheduled {
			mu.Unlock()
			return
		}
		scheduled = true
		mu.Unlock()
		a.post(apply)
	}
	stopTimeout := func() {
		if timeout == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timeout = nil
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-p.Done():
			for {
				select {
				case e := <-p.Events():
					enqueue(e)
				default:
					a.post(func() {
						if a.installer != p {
							return
						}
						a.installState.Unavailable()
						a.installer = nil
						a.mark(paintSession)
					})
					return
				}
			}
		case e := <-p.Events():
			stopTimeout()
			enqueue(e)
		case <-timeout:
			timeout = nil
			a.post(func() {
				if a.installer != p {
					return
				}
				if !a.installState.HelperReady && a.installState.Phase == "unavailable" {
					p.Close()
					a.installer = nil
					a.mark(paintSession)
				}
			})
		}
	}
}
func (a *application) sendInstaller(r firmware.Request) {
	if a.installer == nil {
		a.installState.Unavailable()
		a.mark(paintSession)
		return
	}
	if err := a.installer.Send(r); err != nil {
		a.installer.Close()
		a.installer = nil
		a.installState.Unavailable()
		a.mark(paintSession)
	}
}
func (a *application) installFirmware() {
	if !a.installState.Install(a.coordinator.Active) {
		return
	}
	// Forget the BLE preference before installing so restart cannot silently bind it.
	a.effects(a.coordinator.Disconnect())
	a.subscribed = false
	a.hello = nil
	a.config.Peripheral = ""
	a.saveSettings()
	a.view.Snapshot.Connection = "已断开，正在安装固件"
	a.sendBluetooth(map[string]any{"type": "set-preferred-device", "deviceId": nil})
	a.sendInstaller(firmware.Request{Type: "install", Port: a.installState.Selected, FirmwarePath: a.installPaths.Firmware, ManifestPath: a.installPaths.Manifest})
	a.refresh()
}
func (a *application) sendBluetooth(v any) {
	if a.bluetooth != nil {
		if e := a.bluetooth.Send(v); e != nil {
			a.bluetooth.Close()
			a.bluetooth = nil
			a.effects(a.coordinator.Disconnect())
			a.view.Snapshot.Status = "Bluetooth pipe unavailable"
			a.mark(paintView)
			if !a.inPost {
				a.flush()
			}
		}
	}
}
func (a *application) sendSpeech(v any) {
	if a.speech != nil {
		if e := a.speech.Send(v); e != nil {
			a.speech.Close()
			a.speech = nil
			a.effects(a.coordinator.Cancel("Speech pipe unavailable"))
			a.view.Snapshot.Status = "Speech pipe unavailable"
			a.mark(paintView)
			if !a.inPost {
				a.flush()
			}
		}
	}
}
func (a *application) queryPermissions() {
	a.sendBluetooth(map[string]any{"type": "permissions"})
	a.sendSpeech(map[string]any{"type": "permissions"})
}

// requestPermission shows the system request while one can still appear,
// otherwise System Settings. Bluetooth asks on its own when the helper starts.
func (a *application) requestPermission(n permission.Name) {
	if n == permission.Bluetooth || !permission.Prompts(a.permissions[n]) {
		a.view.Actions.OpenSettings(n)
		return
	}
	a.sendSpeech(map[string]any{"type": "request-permission", "text": string(n)})
}

// onPermission records a helper report. The first time every grant has been
// reported, a missing required one opens the permissions page; a session that
// started without Speech ends with a clear reason rather than a timeout.
func (a *application) onPermission(e bridge.Event) {
	n, st, ok := permission.Parse(e.Permission, e.State)
	if !ok {
		return
	}
	a.permissions[n] = st
	a.mark(paintCatalog)
	// Online recognition does not need the Speech grant.
	if n == permission.Speech && st != permission.Granted && a.coordinator.Active && a.online == nil {
		a.effects(a.coordinator.Cancel("语音识别未授权：请到“权限”页授权后重试"))
	}
	if n == permission.Microphone && (st == permission.Denied || st == permission.Restricted) && a.coordinator.Active && a.coordinator.Mic {
		a.effects(a.coordinator.Cancel(micDenied))
	}
	if a.onboarded {
		return
	}
	for _, name := range []permission.Name{permission.Bluetooth, permission.Speech, permission.Accessibility} {
		if _, reported := a.permissions[name]; !reported {
			return
		}
	}
	a.onboarded = true
	if len(a.permissions.Missing()) > 0 && !a.installState.Busy() {
		a.view.Page = appui.PermissionsPage
		// Started at login without its window: the user must see what is missing.
		if a.atLogin {
			a.showMain()
		}
	}
}

func (a *application) onBluetooth(e bridge.Event) {
	if e.Type == "permission" {
		a.onPermission(e)
		return
	}
	if a.installState.Busy() {
		return
	}
	switch e.Type {
	case "discovered":
		for _, d := range a.view.Snapshot.Devices {
			if d.ID == e.DeviceID {
				return
			}
		}
		if len(a.view.Snapshot.Devices) < 64 {
			a.view.Snapshot.Devices = append(a.view.Snapshot.Devices, appui.Device{ID: e.DeviceID, Name: e.DeviceName})
			a.mark(paintView)
		}
	case "connected":
		a.view.Snapshot.Connection = "Connected; ATT payload ≥182 B"
		a.mark(paintView)
	case "subscribed":
		a.subscribed = true
		if a.hello != nil {
			a.effects(a.coordinator.Control(*a.hello, time.Now()))
		}
	case "control":
		v, err := protocol.ParseControl(e.Payload)
		if err != nil {
			a.effects(a.coordinator.Cancel("Invalid BLE control"))
			return
		}
		if v.Type == "hello" {
			a.hello = &v
			a.view.Snapshot.Firmware = v.Firmware
			a.mark(paintView)
			if !a.subscribed {
				return
			}
		}
		a.effects(a.coordinator.Control(v, time.Now()))
	case "audio":
		a.effects(a.coordinator.Audio(protocol.Frame{SessionID: e.SessionID, Sequence: e.Sequence, Payload: e.Payload}, time.Now()))
	case "error", "disconnected":
		a.subscribed = false
		a.hello = nil
		a.view.Snapshot.Connection = "Disconnected"
		a.mark(paintView)
		a.effects(a.coordinator.Disconnect())
	}
}
func (a *application) onSpeech(e bridge.Event) {
	if e.Type == "permission" {
		a.onPermission(e)
		return
	}
	if e.Type == "model" {
		a.view.Snapshot.Model = e.Message
		a.mark(paintView)
		return
	}
	if e.Type == "disconnected" {
		a.effects(a.coordinator.Cancel("Speech bridge exited"))
		return
	}
	switch e.Type {
	case "hotkey":
		a.onHotkey(e.Text)
		return
	case "mic":
		a.effects(a.coordinator.MicAudio(e.SessionID, e.Payload, time.Now()))
		return
	case "mic-end":
		a.effects(a.coordinator.MicEnd(e.SessionID, time.Now()))
		return
	case "mic-error":
		if a.coordinator.Active && a.coordinator.Mic && a.coordinator.Generation == e.SessionID {
			a.effects(a.coordinator.Cancel("无法使用麦克风：请检查输入设备，或到“权限”页授权麦克风"))
		}
		return
	}
	if e.Type == "refined" && e.SessionID == a.coordinator.Generation && a.localReply != nil {
		select {
		case a.localReply <- e.Text:
		default:
		}
		return
	}
	if e.Type == "partial" || e.Type == "final" {
		// Known misrecognitions become their terms before anything shows,
		// inserts or refines the text.
		e.Text = a.store.Correct(e.Text)
	}
	a.effects(a.coordinator.Speech(e.Type, e.SessionID, e.Text, time.Now()))
}

// jsonFiles is the filter of the vocabulary file dialogs.
var jsonFiles = []mygo.FileFilter{{Name: "JSON", Extensions: []string{"json"}}}

// importVocabulary merges a JSON file the user picks into the vocabulary.
func (a *application) importVocabulary() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.window, Title: "导入词库", ButtonLabel: "导入", Filters: jsonFiles})
		if err != nil || len(paths) == 0 {
			return
		}
		b, err := readLimited(paths[0], 2*1024*1024)
		a.post(func() {
			if err != nil {
				a.setVocabFeedback("无法读取文件："+filepath.Base(paths[0]), false)
				return
			}
			res, err := a.store.Import(b)
			if err != nil {
				a.setVocabFeedback("导入失败：这不是 go-voice 导出的词库 JSON 文件。", false)
				return
			}
			a.saveVocabulary()
			a.setVocabFeedback(importMessage(res), true)
		})
	}()
}

// exportVocabulary writes the terms and replacements to a JSON file the
// user picks.
func (a *application) exportVocabulary() {
	b, err := a.store.Export()
	if err != nil {
		a.setVocabFeedback("导出失败。", false)
		return
	}
	terms, replacements := len(a.store.Terms), len(a.store.Replacements)
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.window, Title: "导出词库", ButtonLabel: "导出", DefaultPath: "go-voice-vocabulary.json", Filters: jsonFiles})
		if err != nil || path == "" {
			return
		}
		err = os.WriteFile(path, b, 0o644)
		a.post(func() {
			if err != nil {
				a.setVocabFeedback("导出失败：无法写入 "+filepath.Base(path), false)
				return
			}
			a.setVocabFeedback(fmt.Sprintf("已导出 %d 个词条、%d 条替换词到 %s。", terms, replacements, filepath.Base(path)), true)
		})
	}()
}

func (a *application) setVocabFeedback(msg string, ok bool) {
	a.view.Snapshot.VocabFeedback, a.view.Snapshot.VocabFeedbackOK = msg, ok
	a.mark(paintView)
}

// importMessage tells what an import added, updated and skipped.
func importMessage(r vocabulary.ImportResult) string {
	msg := fmt.Sprintf("已导入：新增 %d 个词条、%d 条替换词", r.TermsAdded, r.ReplacementsAdded)
	if r.TermsUpdated+r.ReplacementsUpdated > 0 {
		msg += fmt.Sprintf("，更新 %d 个词条、%d 条替换词", r.TermsUpdated, r.ReplacementsUpdated)
	}
	if r.Skipped > 0 {
		msg += fmt.Sprintf("，跳过 %d 条无效或冲突的内容", r.Skipped)
	}
	return msg + "。"
}

// readLimited reads a file of at most limit bytes.
func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = errors.New("file too large")
	}
	return b, err
}

// vocabularyError says why a term was not saved.
func vocabularyError(e error) string {
	switch {
	case errors.Is(e, vocabulary.ErrReplacementTaken):
		return "这个原词已经有替换词了，请编辑原来那一条"
	case errors.Is(e, vocabulary.ErrReplacementIsTerm):
		return "原词是词库中的词条，不能被替换"
	case errors.Is(e, vocabulary.ErrInvalidReplacement):
		return "替换词无效：原词和替换词都要填写且不能相同，原词最多 64 字"
	}
	return "词条无效，未保存"
}
func (a *application) effects(effects []device.Effect) {
	for _, e := range effects {
		switch e.Type {
		case "host_ready":
			a.sendBluetooth(map[string]any{"type": "host_ready"})
		case "host_state":
			a.sendBluetooth(map[string]any{"type": "host_state", "state": e.State})
		case "start":
			a.terms = a.store.Contextual(100)
			a.correctionTerms = a.store.CorrectionTerms()
			a.sessionPacks = a.config.Packs
			if a.config.ASR.Online() {
				a.startOnline(e.ID)
			} else {
				a.sendSpeech(map[string]any{"type": "start", "sessionId": e.ID, "terms": a.terms})
			}
		case "pcm":
			if a.online != nil && a.onlineID == e.ID {
				a.online.PCM(e.PCM)
			} else {
				a.sendLocalPCM(e.ID, e.Sequence, e.PCM)
			}
		case "finish":
			if a.online != nil && a.onlineID == e.ID {
				a.online.Finish()
			} else {
				a.sendSpeech(map[string]any{"type": "finish", "sessionId": e.ID, "text": e.Text})
			}
		case "cancel":
			a.closeOnline()
			if a.refineCancel != nil {
				a.refineCancel()
				a.refineCancel = nil
			}
			a.localReply = nil
			a.sendSpeech(map[string]any{"type": "cancel", "sessionId": e.ID})
		case "refine":
			a.refine(e)
		case "mic_start":
			a.sendSpeech(map[string]any{"type": "mic-start", "sessionId": e.ID})
		case "mic_stop":
			a.sendSpeech(map[string]any{"type": "mic-stop", "sessionId": e.ID})
		case "insert":
			// Never paste while the capsule could hold the keyboard.
			a.hideHUD()
			a.sendSpeech(map[string]any{"type": "insert", "sessionId": e.ID, "text": e.Text})
		default:
			a.sendSpeech(map[string]any{"type": e.Type, "sessionId": e.ID, "text": e.Text})
		}
	}
	a.live.Store(a.coordinator.Active)
	a.mark(paintSession)
	if !a.inPost {
		a.flush()
	}
}

func encodePCM(id, seq uint32, pcm []byte) []byte {
	b := make([]byte, 0, 64+base64.StdEncoding.EncodedLen(len(pcm)))
	b = append(b, `{"type":"pcm","sessionId":`...)
	b = strconv.AppendUint(b, uint64(id), 10)
	b = append(b, `,"sequence":`...)
	b = strconv.AppendUint(b, uint64(seq), 10)
	b = append(b, `,"payloadBase64":"`...)
	b = base64.StdEncoding.AppendEncode(b, pcm)
	return append(b, '"', '}', '\n')
}

func (a *application) sendLocalPCM(id, seq uint32, pcm []byte) {
	if a.speech == nil {
		return
	}
	if e := a.speech.SendLine(encodePCM(id, seq, pcm)); e != nil {
		a.speech.Close()
		a.speech = nil
		a.effects(a.coordinator.Cancel("Speech pipe unavailable"))
		a.view.Snapshot.Status = "Speech pipe unavailable"
	}
}
func (a *application) refine(e device.Effect) {
	if a.config.Refinement.Mode == "off" && !a.config.Jev.Enabled && !a.store.NeedsSegmentation(e.Text) {
		// No refinement: insert the recognized text, with the vocabulary's
		// replacements, at once.
		a.view.Snapshot.Status = "disabled"
		a.effects(a.coordinator.Speech("refinement_done", e.ID, a.store.Correct(e.Text), time.Now()))
		return
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.refineCancel = cancel
	reply := make(chan string, 1)
	a.localReply = reply
	cfg := a.config.Refinement
	hints := append([]string(nil), a.terms...)
	allTerms := append([]string(nil), a.correctionTerms...)
	packs := a.sessionPacks
	speech := a.speech
	judge := a.config.Jev
	corrections := a.store.Snapshot()
	go func() {
		prepared := e.Text
		if corrections.NeedsSegmentation(prepared) {
			prepared = corrections.CorrectSegmented(prepared)
		}
		var evidence refinement.Evidence
		publicNote := ""
		if cfg.Mode != "off" || judge.Enabled {
			var err error
			evidence, err = refinement.PrepareEvidence(ctx, prepared, allTerms, hints, packs)
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, refinement.ErrVocabularyLimit) {
				a.post(func() {
					if ctx.Err() != nil {
						return
					}
					a.view.Snapshot.Status = "词库候选过多或文字过长，已保留本地纠正结果。"
					a.effects(a.coordinator.Speech("refinement_done", e.ID, prepared, time.Now()))
				})
				return
			}
			if err != nil {
				publicNote = "公共词库暂不可用，已继续使用个人词库。 "
			}
		}
		if ctx.Err() != nil {
			return
		}
		// Jev judges the spans that sound like a term first; refinement
		// then leaves the ones it kept and decides the uncertain.
		text, keep, note := judgeVocabulary(ctx, judge, prepared, evidence)
		note = publicNote + note
		if cfg.Mode == "off" {
			if text != prepared {
				text = corrections.CorrectSegmented(text)
			}
			a.post(func() {
				if ctx.Err() != nil {
					return
				}
				a.view.Snapshot.Status = note + "disabled"
				a.effects(a.coordinator.Speech("refinement_done", e.ID, text, time.Now()))
			})
			return
		}
		out, status := refinement.RunEvidence(ctx, cfg, text, evidence, keychain.Store{}, func(ctx context.Context, prompt string) (string, error) {
			if speech == nil {
				return "", errors.New("speech unavailable")
			}
			if err := speech.Send(map[string]any{"type": "refine", "sessionId": e.ID, "text": prompt}); err != nil {
				return "", err
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case result := <-reply:
				return result, nil
			}
		}, keep...)
		// Native segmentation and correction stay on this worker.
		out = corrections.CorrectSegmented(out)
		a.post(func() {
			if ctx.Err() != nil {
				return
			}
			a.view.Snapshot.Status = note + status
			a.effects(a.coordinator.Speech("refinement_done", e.ID, out, time.Now()))
		})
	}()
}

const (
	micDenied    = "麦克风未授权：请到“权限”页打开麦克风后重试"
	micRequested = "请在系统弹窗中允许 go-voice 使用麦克风，然后再按住热键"
)

// onHotkey records with the Mac microphone while the chord is held:
// SpeechBridge reports it held long enough (down), let go (up), or another
// key pressed meanwhile (abort), which means a shortcut rather than speech.
func (a *application) onHotkey(state string) {
	c := &a.coordinator
	now := time.Now()
	switch state {
	case "down":
		if c.Active || !hotkey.Enabled(a.config.Hotkey) {
			return
		}
		switch st := a.permissions[permission.Microphone]; st {
		case permission.Denied, permission.Restricted:
			a.flashHUD(micDenied)
			return
		case permission.Undetermined:
			// The user asked for the microphone just now: this is the
			// moment for the system's request, not in a running session.
			a.sendSpeech(map[string]any{"type": "request-permission", "text": string(permission.Microphone)})
			a.flashHUD(micRequested)
			return
		}
		a.effects(c.MicStart(now))
	case "up":
		a.effects(c.MicRelease(now))
	case "abort":
		if c.MicHeld() {
			a.effects(c.Cancel("已取消：按住热键时按下了其他键"))
		}
	}
}

// setHotkey saves the chord and hands it to SpeechBridge; turning it off
// ends a microphone session.
func (a *application) setHotkey(id string) {
	a.config.Hotkey = hotkey.Normalize(id)
	a.saveSettings()
	a.sendSpeech(map[string]any{"type": "hotkey", "text": a.config.Hotkey})
	if !hotkey.Enabled(a.config.Hotkey) && a.coordinator.Mic {
		a.effects(a.coordinator.Cancel("已关闭麦克风热键"))
	}
	if hotkey.Enabled(a.config.Hotkey) {
		a.queryPermissions()
	}
	a.refresh()
}

// applyASR gives an online engine time to connect, a little longer than its
// own start timeout so that its reason ends the session.
func (a *application) applyASR() {
	a.coordinator.StartTimeout = 0
	if a.config.ASR.Online() {
		a.coordinator.StartTimeout = asr.StartTimeout + 500*time.Millisecond
	}
}

// startOnline recognizes session id online. SpeechBridge only claims the
// session and the target app, so refinement and insertion work as before.
func (a *application) startOnline(id uint32) {
	a.closeOnline()
	a.sendSpeech(map[string]any{"type": "begin", "sessionId": id})
	a.onlineID = id
	c := a.config.ASR
	a.online = asr.Start(ctxOrBackground(a.ctx), asr.Options{
		Config: c,
		Key: func(ctx context.Context) (string, error) {
			return (keychain.Store{}).Get(ctx, asr.KeyAccount(c))
		},
		OnEvent: func(e asr.Event) { a.post(func() { a.onOnline(id, e) }) },
	})
}

func (a *application) closeOnline() {
	if a.online != nil {
		a.online.Close()
		a.online = nil
	}
}

// onOnline hands an online event of session id to the coordinator as
// SpeechBridge's would be; a failure ends the session with its reason.
func (a *application) onOnline(id uint32, e asr.Event) {
	if a.online == nil || a.onlineID != id {
		return
	}
	switch e.Type {
	case "error":
		a.closeOnline()
		if a.coordinator.Active && a.coordinator.Generation == id {
			a.effects(a.coordinator.Cancel(asr.Message(e.Err)))
		}
		return
	case "final":
		a.closeOnline()
		e.Text = a.store.Correct(e.Text)
	case "partial":
		e.Text = a.store.Correct(e.Text)
	}
	a.effects(a.coordinator.Speech(e.Type, id, e.Text, time.Now()))
}

func (a *application) setASRFeedback(c asr.Config, msg string, ok bool) {
	s := &a.view.Snapshot
	s.ASRFeedback, s.ASRFeedbackOK, s.ASRFeedbackFor = msg, ok, c
	a.mark(paintView)
}

// configureASR saves the recognition engine, and a typed key to the
// keychain; for an online engine without one, it checks a key is saved.
func (a *application) configureASR(c asr.Config, key string) {
	c = asr.Normalize(c)
	if c.Online() {
		if err := c.Validate(); err != nil {
			a.setASRFeedback(c, asr.Message(err), false)
			a.refresh()
			return
		}
	}
	a.config.ASR = c
	a.applyASR()
	if a.writeSettings() != nil {
		a.setASRFeedback(c, "保存设置失败：无法写入 ~/Library/Application Support/MyGo-GoVoice/。", false)
		a.refresh()
		return
	}
	if !c.Online() {
		msg, ok := asrSettingsMessage(c, false, nil)
		a.setASRFeedback(c, msg, ok)
		a.refresh()
		return
	}
	go func() {
		ctx := ctxOrBackground(a.ctx)
		var err error
		if key != "" {
			err = (keychain.Store{}).Set(ctx, asr.KeyAccount(c), key)
		} else {
			_, err = (keychain.Store{}).Get(ctx, asr.KeyAccount(c))
		}
		a.post(func() {
			// A failed write tells nothing about a key saved before.
			if key == "" || err == nil {
				a.noteKey(asr.KeyAccount(c), err == nil)
			}
			msg, ok := asrSettingsMessage(c, key != "", err)
			a.setASRFeedback(c, msg, ok)
		})
	}()
}

// checkKey looks up whether the keychain holds a key for account, without
// reading it, so that a page can say a key is saved instead of showing an
// empty field as if it had to be typed again.
func (a *application) checkKey(account string) {
	go func() {
		saved, err := (keychain.Store{}).Has(ctxOrBackground(a.ctx), account)
		a.post(func() {
			if err == nil {
				a.noteKey(account, saved)
			}
		})
	}()
}

// noteKey records whether account holds a key, for the pages to show.
func (a *application) noteKey(account string, saved bool) {
	s := &a.view.Snapshot
	if s.SavedKeys == nil {
		s.SavedKeys = map[string]bool{}
	}
	s.SavedKeys[account] = saved
	a.mark(paintView)
}

// asrSettingsMessage tells how saving the engine c ended: with a typed key
// stored or not, or for an online engine whether a key is saved (keyErr).
func asrSettingsMessage(c asr.Config, typedKey bool, keyErr error) (string, bool) {
	next := "设置已保存，识别引擎：" + asr.ChoiceOf(c).Label + "。"
	switch {
	case typedKey && keyErr != nil:
		return "设置已保存，但 API key 存入钥匙串失败，请重试。", false
	case typedKey:
		return next + "API key 已存入钥匙串。", true
	case c.Online() && keyErr != nil:
		return "设置已保存，但还没有" + c.Provider() + " 的 API key：请填写后再次保存。", false
	}
	return next, true
}

// testASR checks the engine c with the typed key, else the saved one.
func (a *application) testASR(c asr.Config, key string) {
	if a.view.Snapshot.ASRTesting {
		return
	}
	c = asr.Normalize(c)
	if err := c.Validate(); err != nil {
		a.setASRFeedback(c, asr.Message(err), false)
		a.refresh()
		return
	}
	a.view.Snapshot.ASRTesting, a.view.Snapshot.ASRFeedback = true, ""
	a.refresh()
	go func() {
		ctx := ctxOrBackground(a.ctx)
		var err error
		if key == "" {
			if key, err = (keychain.Store{}).Get(ctx, asr.KeyAccount(c)); err != nil {
				err = asr.ErrKey
			}
		}
		var res asr.TestResult
		if err == nil {
			res, err = asr.Test(ctx, c, key)
		}
		a.post(func() {
			a.view.Snapshot.ASRTesting = false
			msg, ok := asrTestMessage(c, res, err)
			a.setASRFeedback(c, msg, ok)
		})
	}()
}

func asrTestMessage(c asr.Config, res asr.TestResult, err error) (string, bool) {
	if err != nil {
		return asr.Message(err), false
	}
	return fmt.Sprintf("连接成功（%.1f 秒），可以使用%s。测试只发送了半秒静音。", res.Elapsed.Seconds(), asr.ChoiceOf(c).Label), true
}

func (a *application) deleteASRKey() {
	c := asr.Normalize(a.view.ASR)
	if err := c.Validate(); err != nil {
		a.setASRFeedback(c, asr.Message(err), false)
		a.refresh()
		return
	}
	go func() {
		err := (keychain.Store{}).Delete(ctxOrBackground(a.ctx), asr.KeyAccount(c))
		a.post(func() {
			if err != nil {
				a.setASRFeedback(c, "删除失败：无法访问钥匙串。", false)
			} else {
				a.noteKey(asr.KeyAccount(c), false)
				a.setASRFeedback(c, "已从钥匙串删除"+c.Provider()+" 的 API key。", true)
			}
		})
	}()
}

// judgeVocabulary runs the Jev check when on: the text with the likely
// terms in place, the heard words to keep, and a note when it was skipped.
func judgeVocabulary(ctx context.Context, c jev.Config, text string, evidence refinement.Evidence) (string, []string, string) {
	if !c.Enabled {
		return text, nil, ""
	}
	res, err := checkVocabulary(ctx, c, text, evidence)
	if err != nil {
		note := ""
		if ctx.Err() == nil {
			note = "Jev 词库判断已跳过：" + jev.Message(err) + " "
		}
		return text, nil, note
	}
	return res.Text, res.Keep, ""
}

// checkVocabulary has Jev at the endpoint of c judge the spans of text that
// sound like a vocabulary term, within jev.Timeout; on failure text goes on
// unchanged.
var checkVocabulary = func(ctx context.Context, c jev.Config, text string, evidence refinement.Evidence) (jev.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, jev.Timeout)
	defer cancel()
	if len(evidence.Spans(text)) == 0 {
		return jev.Result{Text: text}, nil
	}
	key, err := (keychain.Store{}).Get(ctx, jev.KeyAccount(c))
	if err != nil || key == "" {
		return jev.Result{Text: text}, jev.ErrKey
	}
	return jev.CheckEvidence(ctx, c, text, evidence, key)
}

func (a *application) setJevFeedback(c jev.Config, msg string, ok bool) {
	s := &a.view.Snapshot
	s.JevFeedback, s.JevFeedbackOK, s.JevFeedbackFor = msg, ok, c
	a.mark(paintView)
}

// configureJev saves the vocabulary check, and a typed key to the keychain
// under its endpoint; turned on without one, it checks a key is saved. An
// invalid endpoint is not saved.
func (a *application) configureJev(c jev.Config, key string) {
	c = jev.Normalize(c)
	if _, err := c.URL(); err != nil {
		a.setJevFeedback(c, "未保存："+jev.Message(err), false)
		a.refresh()
		return
	}
	a.config.Jev = c
	if a.writeSettings() != nil {
		a.setJevFeedback(c, "保存设置失败：无法写入 ~/Library/Application Support/MyGo-GoVoice/。", false)
		a.refresh()
		return
	}
	if key == "" && !c.Enabled {
		msg, ok := jevSettingsMessage(c, false, nil)
		a.setJevFeedback(c, msg, ok)
		a.refresh()
		return
	}
	go func() {
		ctx := ctxOrBackground(a.ctx)
		var err error
		if key != "" {
			err = (keychain.Store{}).Set(ctx, jev.KeyAccount(c), key)
		} else {
			_, err = (keychain.Store{}).Get(ctx, jev.KeyAccount(c))
		}
		a.post(func() {
			if key == "" || err == nil {
				a.noteKey(jev.KeyAccount(c), err == nil)
			}
			msg, ok := jevSettingsMessage(c, key != "", err)
			a.setJevFeedback(c, msg, ok)
		})
	}()
}

// jevSettingsMessage tells how saving c ended: with a typed key stored or
// not, or when turned on whether a key is saved (keyErr).
func jevSettingsMessage(c jev.Config, typedKey bool, keyErr error) (string, bool) {
	next := "设置已保存，Jev 词库判断已关闭。"
	if c.Enabled {
		next = "设置已保存，Jev 词库判断已开启（" + c.Host() + "，" + jev.Normalize(c).Model + "）。"
	}
	switch {
	case typedKey && keyErr != nil:
		return "设置已保存，但 API key 存入钥匙串失败，请重试。", false
	case typedKey:
		return next + "API key 已存入钥匙串。", true
	case c.Enabled && keyErr != nil:
		return "设置已保存，但还没有 " + c.Host() + " 的 API key：请填写后再次保存。", false
	}
	return next, true
}

// testJev judges the fixed sample at the endpoint and model of c, with the
// typed key, else the one saved for that endpoint.
func (a *application) testJev(c jev.Config, key string) {
	if a.view.Snapshot.JevTesting {
		return
	}
	draft := c
	c = jev.Normalize(c)
	a.view.Snapshot.JevTesting, a.view.Snapshot.JevFeedback = true, ""
	a.refresh()
	go func() {
		ctx := ctxOrBackground(a.ctx)
		var err error
		if _, err = c.URL(); err == nil && key == "" {
			if key, err = (keychain.Store{}).Get(ctx, jev.KeyAccount(c)); err != nil {
				err = jev.ErrKey
			}
		}
		var res jev.TestResult
		if err == nil {
			res, err = jev.Test(ctx, c, key)
		}
		a.post(func() {
			a.view.Snapshot.JevTesting = false
			msg, ok := jevTestMessage(res, err)
			a.setJevFeedback(draft, msg, ok)
		})
	}()
}

// jevTestMessage shows how Jev judged each span of the sample and what
// the thresholds make of it.
func jevTestMessage(res jev.TestResult, err error) (string, bool) {
	if err != nil {
		return jev.Message(err), false
	}
	text := []rune(jev.TestText)
	var parts []string
	for _, d := range res.Decisions {
		verdict := "交给 AI 润色判断"
		switch {
		case d.P >= jev.Replace:
			verdict = "替换为“" + d.Term + "”"
		case d.OriginalP >= jev.Replace:
			verdict = "保留原文"
		}
		from, to := max(0, d.Start-2), min(len(text), d.End+2)
		parts = append(parts, fmt.Sprintf("“%s”%.0f%%，%s", string(text[from:to]), d.P*100, verdict))
	}
	return fmt.Sprintf("连接成功（%.1f 秒）。示例句的判断：%s。测试只发送固定示例句。", res.Elapsed.Seconds(), strings.Join(parts, "；")), true
}

// deleteJevKey removes the key saved for the endpoint the tab shows.
func (a *application) deleteJevKey() {
	draft := a.view.Jev
	c := jev.Normalize(draft)
	go func() {
		err := (keychain.Store{}).Delete(ctxOrBackground(a.ctx), jev.KeyAccount(c))
		a.post(func() {
			if err != nil {
				a.setJevFeedback(draft, "删除失败：无法访问钥匙串。", false)
			} else {
				a.noteKey(jev.KeyAccount(c), false)
				a.setJevFeedback(draft, "已从钥匙串删除 "+c.Host()+" 的 API key。", true)
			}
		})
	}()
}
