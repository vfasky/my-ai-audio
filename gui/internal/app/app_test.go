package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/firmware"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

func installReady() firmware.State {
	s := firmware.Initial()
	s.Apply(firmware.Event{Type: "ready"})
	s.Scan()
	s.Apply(firmware.Event{Type: "ports", Devices: []firmware.Device{{ID: "/dev/cu.usbmodem-secret", Name: "serial-secret"}}})
	s.Select("/dev/cu.usbmodem-secret")
	return s
}
func TestInstallRefusesActiveSpeech(t *testing.T) {
	a := &application{installState: installReady()}
	a.coordinator.Active = true
	a.config.Peripheral = "existing-ble"
	a.installFirmware()
	if a.installState.Phase != "ready" || !a.coordinator.Active || a.config.Peripheral != "existing-ble" {
		t.Fatal("active speech changed")
	}
}
func TestInstallDisconnectsAndDoesNotPersistUSB(t *testing.T) {
	a := &application{installState: installReady(), directory: t.TempDir()}
	a.config.Peripheral = "old-ble"
	a.coordinator.Ready = true
	a.subscribed = true
	a.installFirmware()
	if a.config.Peripheral != "" || a.coordinator.Ready || a.subscribed {
		t.Fatal("BLE selection retained")
	}
	if a.installState.Phase != "unavailable" {
		t.Fatal("missing helper not isolated")
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "old-ble") {
		t.Fatal("device identifier persisted")
	}
}
func TestInstallBlocksBLEEvents(t *testing.T) {
	a := &application{installState: installReady()}
	if !a.installState.Install(false) {
		t.Fatal("start")
	}
	a.onBluetooth(bridge.Event{Type: "subscribed"})
	a.onBluetooth(bridge.Event{Type: "connected"})
	if a.subscribed || a.coordinator.Ready || a.view.Snapshot.Connection != "" {
		t.Fatal("BLE events accepted during install")
	}
	// App refresh snapshots expose the same gate as the reducer.
	a.refresh()
	if !a.view.Snapshot.Install.Busy() || a.view.Snapshot.Install.CanInstall(false) {
		t.Fatal("incorrect UI gate")
	}
}

func perm(name, state string) bridge.Event {
	return bridge.Event{Type: "permission", Permission: name, State: state}
}
func TestFirstCompletePermissionReportOpensOnboarding(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial()}
	a.onBluetooth(perm("bluetooth", "granted"))
	a.onSpeech(perm("speech", "undetermined"))
	if a.view.Page != 0 || a.onboarded {
		t.Fatal("decided before every required grant reported")
	}
	a.onSpeech(perm("accessibility", "granted"))
	if a.view.Page != appui.PermissionsPage || !a.onboarded {
		t.Fatalf("page %d", a.view.Page)
	}
	a.view.Page = 0
	a.onSpeech(perm("speech", "denied"))
	if a.view.Page != 0 {
		t.Fatal("onboarding must only open once")
	}
}
func TestSpeechWithoutGrantEndsSessionWithReason(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial(), onboarded: true}
	a.coordinator.Active = true
	a.onSpeech(perm("speech", "undetermined"))
	if a.coordinator.Active || !strings.Contains(a.coordinator.Status, "语音识别未授权") {
		t.Fatalf("status %q", a.coordinator.Status)
	}
}

func TestHotkeyRecordsWithMicrophoneUntilReleased(t *testing.T) {
	a := &application{permissions: permission.Set{permission.Microphone: permission.Granted}, onboarded: true, config: settings{Hotkey: "shift+option"}}
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	c := &a.coordinator
	if !c.Active || !c.Mic || c.Phase != "starting" {
		t.Fatalf("start: %+v", c.Phase)
	}
	id := c.Generation
	a.onSpeech(bridge.Event{Type: "ready", SessionID: id})
	a.onSpeech(bridge.Event{Type: "mic", SessionID: id, Payload: make([]byte, bridge.MicFrame)})
	if c.Meter.Frames != 1 || c.Phase != "recording" {
		t.Fatalf("frames %d phase %s", c.Meter.Frames, c.Phase)
	}
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "up"})
	a.onSpeech(bridge.Event{Type: "mic-end", SessionID: id})
	if c.Phase != "finalizing" {
		t.Fatalf("finish: %s", c.Phase)
	}
	// A second chord during the session is ignored.
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	if c.Generation != id {
		t.Fatal("second session started")
	}
}

func TestHotkeyAbortAndMissingMicrophone(t *testing.T) {
	a := &application{permissions: permission.Set{permission.Microphone: permission.Granted}, onboarded: true, config: settings{Hotkey: "shift+option"}}
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "abort"})
	if a.coordinator.Active || !strings.Contains(a.coordinator.Status, "其他键") {
		t.Fatalf("abort: %v %q", a.coordinator.Active, a.coordinator.Status)
	}
	a.permissions[permission.Microphone] = permission.Denied
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	if a.coordinator.Active || a.coordinator.Status != micDenied {
		t.Fatalf("denied: %q", a.coordinator.Status)
	}
	a.permissions[permission.Microphone] = permission.Undetermined
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	if a.coordinator.Active || a.coordinator.Status != micRequested {
		t.Fatalf("undetermined: %q", a.coordinator.Status)
	}
	a.permissions[permission.Microphone] = permission.Granted
	a.config.Hotkey = "off"
	a.onSpeech(bridge.Event{Type: "hotkey", Text: "down"})
	if a.coordinator.Active {
		t.Fatal("hotkey off")
	}
}

func TestUpdatesUnavailableDetail(t *testing.T) {
	for _, development := range []bool{true, false} {
		if got := updatesUnavailableDetail("", development); !strings.Contains(got, "开发版") {
			t.Fatalf("unversioned build: %q", got)
		}
	}
	if got := updatesUnavailableDetail("0.1.0", true); !strings.Contains(got, "开发版") {
		t.Fatalf("debug build: %q", got)
	}
	got := updatesUnavailableDetail("0.1.0", false)
	if !strings.Contains(got, "0.1.0") || !strings.Contains(got, "~/Applications") || strings.Contains(got, "开发版") {
		t.Fatalf("release build: %q", got)
	}
}

func TestRemoteTestMessage(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{refinement.ErrConfig, "https://"},
		{refinement.ErrKey, "API key"},
		{fmt.Errorf("post: %w", context.DeadlineExceeded), "超时"},
		{refinement.ErrResponse, "接口类型"},
		{&refinement.StatusError{Code: 401}, "HTTP 401"},
		{&refinement.StatusError{Code: 403}, "没有权限"},
		{&refinement.StatusError{Code: 404}, "/v1"},
		{&refinement.StatusError{Code: 429}, "额度"},
		{&refinement.StatusError{Code: 503}, "HTTP 503"},
		{&refinement.StatusError{Code: 400}, "HTTP 400"},
		{errors.New("dial tcp: no such host"), "无法连接"},
	} {
		if got, ok := remoteTestMessage(refinement.TestResult{}, c.err); ok || !strings.Contains(got, c.want) {
			t.Errorf("%v: %q %v", c.err, got, ok)
		}
	}
	long := strings.Repeat("好", 80)
	got, ok := remoteTestMessage(refinement.TestResult{Text: long, ThinkingOff: true, Elapsed: 1200 * time.Millisecond}, nil)
	if !ok || !strings.HasPrefix(got, "连接成功（1.2 秒，已请求关闭思考模式）") || !strings.HasSuffix(got, "…") || strings.Count(got, "好") != 60 {
		t.Fatalf("success: %q %v", got, ok)
	}
	if got, _ := remoteTestMessage(refinement.TestResult{Text: "好"}, nil); !strings.Contains(got, "不接受关闭思考") {
		t.Fatalf("thinking default: %q", got)
	}
}

func TestSettingsMessage(t *testing.T) {
	remote := refinement.Config{Engine: "remote", Mode: "light", Endpoint: "https://provider.example/v1", Model: "m"}
	for _, c := range []struct {
		name  string
		cfg   refinement.Config
		saved error
		typed bool
		key   error
		want  string
		ok    bool
	}{
		{"local", refinement.Default(), nil, false, nil, "设置已保存。", true},
		{"remote with key", remote, nil, true, nil, "设置已保存，API key 已存入钥匙串。", true},
		{"key failed", remote, nil, true, errors.New("denied"), "存入钥匙串失败", false},
		{"file failed", remote, errors.New("disk"), false, nil, "保存设置失败", false},
		{"remote incomplete", refinement.Config{Engine: "remote", Endpoint: "http://x"}, nil, false, nil, "远端润色还需要", false},
	} {
		got, ok := settingsMessage(c.cfg, c.saved, c.typed, c.key)
		if ok != c.ok || !strings.Contains(got, c.want) {
			t.Errorf("%s: %q %v", c.name, got, ok)
		}
	}
}

func TestCredentialPrecheck(t *testing.T) {
	if msg, ok := credentialPrecheck(" ", "k"); ok || !strings.Contains(msg, "端点") {
		t.Fatalf("no endpoint: %q", msg)
	}
	if msg, ok := credentialPrecheck("https://provider.example/v1", ""); ok || !strings.Contains(msg, "API key") {
		t.Fatalf("no key: %q", msg)
	}
	if _, ok := credentialPrecheck("https://provider.example/v1", "k"); !ok {
		t.Fatal("valid credential refused")
	}
}

func TestSaveSettingsWritesFile(t *testing.T) {
	a := &application{directory: filepath.Join(t.TempDir(), "settings"), config: settings{Refinement: refinement.Default()}}
	a.config.Refinement.Model = "m"
	if err := a.writeSettings(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.directory, "settings.json"))
	if err != nil || !strings.Contains(string(b), `"model":"m"`) {
		t.Fatalf("settings %s %v", b, err)
	}
	if info, _ := os.Stat(filepath.Join(a.directory, "settings.json")); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestSpeechAppliesVocabularyAliases(t *testing.T) {
	a := &application{}
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "recording"
	a.onSpeech(bridge.Event{Type: "partial", SessionID: 7, Text: "打开施玉宝"})
	if a.coordinator.Partial != "打开私域宝" {
		t.Fatalf("partial %q", a.coordinator.Partial)
	}
	a.refresh()
	if len(a.view.Snapshot.Replacements) != 1 {
		t.Fatalf("snapshot %+v", a.view.Snapshot.Replacements)
	}
}

func TestVocabularyErrorMessages(t *testing.T) {
	for e, want := range map[error]string{
		vocabulary.ErrReplacementTaken:   "已经有替换词",
		vocabulary.ErrReplacementIsTerm:  "不能被替换",
		vocabulary.ErrInvalidReplacement: "64",
		errors.New("invalid term"):       "未保存",
	} {
		if got := vocabularyError(e); !strings.Contains(got, want) {
			t.Errorf("%v: %q", e, got)
		}
	}
}

func TestRefinementOffInsertsReplacedText(t *testing.T) {
	a := &application{config: settings{Refinement: refinement.Config{Engine: "remote", Mode: "off"}}}
	for _, r := range []vocabulary.Replacement{{From: "石玉宝", To: "私域宝"}, {From: "公寓", To: "公域"}, {From: "公域托客", To: "公域拓客"}} {
		_ = a.store.PutReplacement("", r)
	}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "finalizing"
	a.onSpeech(bridge.Event{Type: "final", SessionID: 7, Text: "打开石玉宝看公寓托客"})
	c := a.coordinator
	if c.Original != "打开私域宝看公域拓客" || c.Refined != c.Original || c.Phase != "inserting" {
		t.Fatalf("original %q refined %q phase %q", c.Original, c.Refined, c.Phase)
	}
}

func TestImportMessageAndReadLimited(t *testing.T) {
	if got := importMessage(vocabulary.ImportResult{TermsAdded: 2, ReplacementsAdded: 1}); got != "已导入：新增 2 个词条、1 条替换词。" {
		t.Fatalf("%q", got)
	}
	got := importMessage(vocabulary.ImportResult{TermsAdded: 1, TermsUpdated: 1, ReplacementsUpdated: 2, Skipped: 3})
	if !strings.Contains(got, "更新 1 个词条、2 条替换词") || !strings.Contains(got, "跳过 3 条") {
		t.Fatalf("%q", got)
	}
	path := filepath.Join(t.TempDir(), "v.json")
	_ = os.WriteFile(path, []byte("12345"), 0o600)
	if b, err := readLimited(path, 5); err != nil || string(b) != "12345" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := readLimited(path, 4); err == nil {
		t.Fatal("oversized file read")
	}
}

// idleSession is an online session that never reports anything.
func idleSession() *asr.Session {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return asr.Start(ctx, asr.Options{})
}

func TestOnlineEventsDriveTheSession(t *testing.T) {
	a := &application{permissions: permission.Set{}, onboarded: true}
	a.config.ASR = asr.Choices[1].Config
	a.applyASR()
	if a.coordinator.StartTimeout <= asr.StartTimeout {
		t.Fatalf("start timeout %v", a.coordinator.StartTimeout)
	}
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "starting"
	a.online, a.onlineID = idleSession(), 7
	a.onOnline(7, asr.Event{Type: "ready"})
	if a.coordinator.Phase != "recording" {
		t.Fatalf("phase %q", a.coordinator.Phase)
	}
	// Online recognition goes on without the Speech grant.
	a.onSpeech(perm("speech", "denied"))
	if !a.coordinator.Active {
		t.Fatal("online session ended for the Speech grant")
	}
	a.onOnline(7, asr.Event{Type: "partial", Text: "打开施玉宝"})
	a.onOnline(6, asr.Event{Type: "partial", Text: "old"})
	if a.coordinator.Partial != "打开私域宝" {
		t.Fatalf("partial %q", a.coordinator.Partial)
	}
	a.coordinator.Phase = "finalizing"
	a.config.Refinement.Mode = "off"
	a.onOnline(7, asr.Event{Type: "final", Text: "打开施玉宝。"})
	if a.online != nil || a.coordinator.Original != "打开私域宝。" || a.coordinator.Phase != "inserting" {
		t.Fatalf("final: %v %q %q", a.online != nil, a.coordinator.Original, a.coordinator.Phase)
	}
}

func TestOnlineFailureEndsSessionWithReason(t *testing.T) {
	a := &application{}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 3, "recording"
	a.online, a.onlineID = idleSession(), 3
	a.onOnline(3, asr.Event{Type: "error", Err: &asr.StatusError{Code: 401}})
	if a.coordinator.Active || a.online != nil || !strings.Contains(a.coordinator.Status, "API key 无效") {
		t.Fatalf("status %q", a.coordinator.Status)
	}
	// Cancelling closes the online session too.
	a.coordinator.Active, a.coordinator.Generation = true, 4
	a.online, a.onlineID = idleSession(), 4
	a.effects(a.coordinator.Cancel("已取消"))
	if a.online != nil {
		t.Fatal("online session left open")
	}
}

func TestASRMessages(t *testing.T) {
	online, local := asr.Choices[1].Config, asr.Choices[0].Config
	for _, tc := range []struct {
		c     asr.Config
		typed bool
		err   error
		want  string
		ok    bool
	}{
		{local, false, nil, "识别引擎：Apple 本地", true},
		{online, true, nil, "API key 已存入钥匙串", true},
		{online, true, errors.New("x"), "存入钥匙串失败", false},
		{online, false, errors.New("missing"), "还没有支付宝 MaaS 的 API key", false},
		{online, false, nil, "识别引擎：支付宝 MaaS · Fun-ASR 实时", true},
		{asr.Normalize(asr.Config{Engine: asr.EngineOmni}), false, errors.New("missing"), "还没有阿里云百炼 的 API key", false},
	} {
		if msg, ok := asrSettingsMessage(tc.c, tc.typed, tc.err); ok != tc.ok || !strings.Contains(msg, tc.want) {
			t.Errorf("%+v: %q %v", tc, msg, ok)
		}
	}
	if msg, ok := asrTestMessage(online, asr.TestResult{Elapsed: 600 * time.Millisecond}, nil); !ok || !strings.Contains(msg, "连接成功（0.6 秒）") {
		t.Fatalf("%q", msg)
	}
	if msg, ok := asrTestMessage(online, asr.TestResult{}, asr.ErrKey); ok || !strings.Contains(msg, "API key") {
		t.Fatalf("%q", msg)
	}
}

func TestSettingsWithoutASRAreLocal(t *testing.T) {
	var s settings
	if err := json.Unmarshal([]byte(`{"peripheral":"","refinement":{"engine":"local","mode":"light"}}`), &s); err != nil {
		t.Fatal(err)
	}
	if asr.Normalize(s.ASR).Online() {
		t.Fatal("old settings must stay local")
	}
}

func TestLoginItemFollowsTheSystem(t *testing.T) {
	enabled, approval := false, errors.New("mygo: allow go-voice to open at login in System Settings > General > Login Items")
	var result error
	restoreSet, restoreRead := setOpenAtLogin, openAtLogin
	defer func() { setOpenAtLogin, openAtLogin = restoreSet, restoreRead }()
	setOpenAtLogin = func(open bool) error {
		enabled = open && result == nil
		return result
	}
	openAtLogin = func() bool { return enabled }
	a := &application{permissions: permission.Set{}, installState: firmware.Initial()}
	s := &a.view.Snapshot
	a.setLoginItem(true)
	if !s.OpenAtLogin || s.LoginApproval || !s.LoginFeedbackOK {
		t.Fatalf("enable: %+v", s)
	}
	a.setLoginItem(false)
	if s.OpenAtLogin || s.LoginFeedback != "已关闭登录时打开。" {
		t.Fatalf("disable: %+v", s)
	}
	result = approval
	a.setLoginItem(true)
	if s.OpenAtLogin || !s.LoginApproval || s.LoginFeedbackOK {
		t.Fatalf("approval: %+v", s)
	}
	enabled = true
	a.readLoginItem()
	if !s.OpenAtLogin || s.LoginApproval || !s.LoginFeedbackOK {
		t.Fatalf("approved in System Settings: %+v", s)
	}
}

func TestLoginMessage(t *testing.T) {
	for _, c := range []struct {
		open bool
		err  error
		want string
		ok   bool
	}{
		{true, nil, "已开启：下次登录 Mac 时 go-voice 会在菜单栏后台启动。", true},
		{false, nil, "已关闭登录时打开。", true},
		{true, errors.New("mygo: starting at login needs an app bundle; run the app with mygo dev or build it with mygo build"), "无法更改：请启动打包后的 go-voice.app，而不是直接运行可执行文件。", false},
		{false, errors.New("mygo: Operation not permitted"), "无法关闭登录时打开：Operation not permitted", false},
	} {
		if got, ok := loginMessage(c.open, c.err); got != c.want || ok != c.ok {
			t.Errorf("loginMessage(%v, %v) = %q, %v", c.open, c.err, got, ok)
		}
	}
}

func TestLoginLaunchShowsMissingPermissions(t *testing.T) {
	a := &application{permissions: permission.Set{}, installState: firmware.Initial(), atLogin: true}
	a.onBluetooth(perm("bluetooth", "granted"))
	a.onSpeech(perm("speech", "denied"))
	a.onSpeech(perm("accessibility", "granted"))
	if a.view.Page != appui.PermissionsPage {
		t.Fatalf("page %d", a.view.Page)
	}
}

func TestJudgeVocabularyAppliesOrSkips(t *testing.T) {
	restore := checkVocabulary
	defer func() { checkVocabulary = restore }()
	checkVocabulary = func(_ context.Context, _ jev.Config, text string, _ refinement.Evidence) (jev.Result, error) {
		return jev.Result{Text: strings.Replace(text, "一致科技", "伊智科技", 1), Keep: []string{"一致"}}, nil
	}
	text, keep, note := judgeVocabulary(context.Background(), jev.Config{Enabled: true}, "一致科技意见一致", refinement.Evidence{Personal: []string{"伊智"}, Text: "一致科技意见一致"})
	if text != "伊智科技意见一致" || len(keep) != 1 || note != "" {
		t.Fatalf("%q %v %q", text, keep, note)
	}
	if text, keep, _ := judgeVocabulary(context.Background(), jev.Config{}, "一致科技", refinement.Evidence{Personal: []string{"伊智"}, Text: "一致科技"}); text != "一致科技" || keep != nil {
		t.Fatalf("off: %q %v", text, keep)
	}
	checkVocabulary = func(_ context.Context, _ jev.Config, text string, _ refinement.Evidence) (jev.Result, error) {
		return jev.Result{Text: "never used"}, &jev.StatusError{Code: 401}
	}
	text, keep, note = judgeVocabulary(context.Background(), jev.Config{Enabled: true}, "一致科技", refinement.Evidence{Personal: []string{"伊智"}, Text: "一致科技"})
	if text != "一致科技" || keep != nil || !strings.Contains(note, "HTTP 401") {
		t.Fatalf("failure: %q %v %q", text, keep, note)
	}
}

func TestJevMessages(t *testing.T) {
	maas := jev.Config{Enabled: true, Endpoint: "https://maas-api.antdigital.com/v1/systemone", Model: "jev-1.13"}
	if msg, ok := jevSettingsMessage(maas, false, errors.New("missing")); ok || !strings.Contains(msg, "还没有 maas-api.antdigital.com 的 API key") {
		t.Fatalf("%q", msg)
	}
	if msg, ok := jevSettingsMessage(maas, true, nil); !ok || msg != "设置已保存，Jev 词库判断已开启（maas-api.antdigital.com，jev-1.13）。API key 已存入钥匙串。" {
		t.Fatalf("%q", msg)
	}
	res := jev.TestResult{Elapsed: 300 * time.Millisecond}
	for _, s := range refinement.Spans(jev.TestText, jev.TestTerms) {
		p := 0.06
		if s.Start < 10 {
			p = 0.93
		}
		res.Decisions = append(res.Decisions, jev.Decision{Span: s, P: p, OriginalP: 1 - p})
	}
	msg, ok := jevTestMessage(res, nil)
	if !ok || !strings.Contains(msg, "“讨论一致科技”93%，替换为“伊智”") || !strings.Contains(msg, "“见很一致”6%，保留原文") {
		t.Fatalf("%q", msg)
	}
	if msg, ok := jevTestMessage(jev.TestResult{}, jev.ErrKey); ok || msg == "" {
		t.Fatalf("%q", msg)
	}
}

func TestSettingsKeepJev(t *testing.T) {
	var s settings
	if json.Unmarshal([]byte(`{"jev":{"enabled":true,"endpoint":"https://maas-api.antdigital.com/v1/systemone","model":"jev-1.13"}}`), &s) != nil || !s.Jev.Enabled || s.Jev.Model != "jev-1.13" {
		t.Fatalf("%+v", s)
	}
	if json.Unmarshal([]byte(`{}`), &s) != nil {
		t.Fatal("old settings")
	}
}

func TestConfigureJevRefusesAnInvalidEndpoint(t *testing.T) {
	a := &application{}
	a.configureJev(jev.Config{Enabled: true, Endpoint: "http://maas-api.antdigital.com/v1/systemone"}, "")
	s := a.view.Snapshot
	if a.config.Jev.Enabled || s.JevFeedbackOK || !strings.HasPrefix(s.JevFeedback, "未保存：接口地址无效") {
		t.Fatalf("%+v %q", a.config.Jev, s.JevFeedback)
	}
}

func TestSessionPaintSkipsVocabularyCopy(t *testing.T) {
	a := &application{}
	if err := a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 50}, false); err != nil {
		t.Fatal(err)
	}
	a.coordinator.Ready = true
	a.coordinator.Phase = "recording"
	a.coordinator.Partial = "hi"
	a.effects(nil)
	if a.view.Snapshot.Terms != nil {
		t.Fatal("session paint copied vocabulary")
	}
	if !a.view.Snapshot.Ready || a.view.Snapshot.Phase != "recording" || a.view.Snapshot.Partial != "hi" {
		t.Fatalf("%+v", a.view.Snapshot)
	}
	a.refresh()
	if len(a.view.Snapshot.Terms) != 1 {
		t.Fatal("full refresh skipped vocabulary")
	}
}

func TestStartDoesNotRewriteVocabulary(t *testing.T) {
	dir := t.TempDir()
	a := &application{directory: dir}
	if err := a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 50}, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "vocabulary.json")
	if err := a.store.Save(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a.coordinator.Ready = true
	a.effects(a.coordinator.Control(protocol.Control{Type: "ptt_down", SessionID: 7, Seq: 0}, time.Now()))
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("vocabulary rewritten: %v %s", err, after)
	}
	if !a.coordinator.Active || len(a.terms) != 1 || a.terms[0] != "伊智" {
		t.Fatalf("start %+v %v", a.coordinator, a.terms)
	}
}

func TestEncodePCMLine(t *testing.T) {
	pcm := bytes.Repeat([]byte{1}, 640)
	line := encodePCM(9, 3, pcm)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		t.Fatal("newline")
	}
	var cmd struct {
		Type          string `json:"type"`
		SessionID     uint32 `json:"sessionId"`
		Sequence      uint32 `json:"sequence"`
		PayloadBase64 string `json:"payloadBase64"`
	}
	if err := json.Unmarshal(line[:len(line)-1], &cmd); err != nil {
		t.Fatal(err)
	}
	if cmd.Type != "pcm" || cmd.SessionID != 9 || cmd.Sequence != 3 {
		t.Fatalf("%+v", cmd)
	}
	raw, err := base64.StdEncoding.DecodeString(cmd.PayloadBase64)
	if err != nil || !bytes.Equal(raw, pcm) || cmd.PayloadBase64 != base64.StdEncoding.EncodeToString(pcm) {
		t.Fatalf("payload %v", err)
	}
}
