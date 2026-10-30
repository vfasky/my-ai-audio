package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/hotkey"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
)

// GeneralPage is the index of the app's own settings in pages.
const GeneralPage = 6

// general holds how go-voice itself behaves; the system's login items are the
// truth for opening at login, so the switch shows what they report.
func (v *View) general(c *ui.Context) {
	s := v.Snapshot
	group(c, "startup", "启动", nil, func() {
		row(c, "open-at-login", func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
				ui.Icon(c, iconPower).Size(15, 15).TextColor(pick(s.OpenAtLogin, colGreen, colSubtle))
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "登录时打开").FontSize(14).FontWeight(500).SingleLine()
				ui.Text(c, "登录 Mac 后在菜单栏后台运行，不打开窗口；缺少必需权限时会打开“权限”页。").
					FontSize(12).TextColor(colMuted).LineHeight(1.4)
			})
			on := s.OpenAtLogin
			if ui.Switch(c, &on).Label("登录时打开 go-voice").Changed() {
				v.Actions.SetOpenAtLogin(on)
			}
		})
	})
	if s.LoginApproval {
		ui.Row(c.Key("login-approval")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "还需在“系统设置 › 通用 › 登录项”中允许 go-voice，允许后这里会自动更新。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "login-items", "打开登录项", pillPrimary, iconArrow, false) {
				v.Actions.OpenLoginItems()
			}
		})
	} else if s.LoginFeedback != "" {
		feedback(c, s.LoginFeedback, s.LoginFeedbackOK)
	}
	notice(c, "login-note", "也可以在“系统设置 › 通用 › 登录项”中关闭；这里的开关始终与系统设置一致。", false)
	v.hotkey(c)
}

// hotkey chooses the chord held to record with the Mac microphone.
func (v *View) hotkey(c *ui.Context) {
	s := v.Snapshot
	current := hotkey.Normalize(s.Hotkey)
	labels := make([]string, len(hotkey.Choices))
	i := 0
	for j, ch := range hotkey.Choices {
		labels[j] = ch.Label
		if ch.ID == current {
			i = j
		}
	}
	group(c, "hotkey", "麦克风热键", nil, func() {
		row(c, "hotkey-chord", func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
				ui.Icon(c, iconWave).Size(15, 15).TextColor(pick(hotkey.Enabled(current), colOrange, colSubtle))
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, "按住说话").FontSize(14).FontWeight(500).SingleLine()
				ui.Text(c, "在任何应用中按住这组按键即用 Mac 麦克风录音，松开后识别并插入文字。按住时按下其他键视为普通快捷键，不录音。").
					FontSize(12).TextColor(colMuted).LineHeight(1.4)
			})
		})
		row(c, "hotkey-choice", func() {
			label(c, "按键", "默认 ⇧ Shift + ⌥ Option。")
			if segmented(c, "hotkey-seg", &i, labels) {
				v.Actions.SetHotkey(hotkey.Choices[i].ID)
			}
		})
	})
	if !hotkey.Enabled(current) {
		return
	}
	switch st := s.Permissions[permission.Microphone]; st {
	case permission.Granted:
	case permission.Denied, permission.Restricted:
		ui.Row(c.Key("mic-denied")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "麦克风未授权，热键无法录音。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "mic-settings", "打开系统设置", pillPrimary, iconArrow, false) {
				v.Actions.OpenSettings(permission.Microphone)
			}
		})
	default:
		ui.Row(c.Key("mic-request")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconShield).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "第一次使用热键前需要授权麦克风。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "mic-request", "授权麦克风", pillPrimary, nil, false) {
				v.Actions.RequestPermission(permission.Microphone)
			}
		})
	}
	notice(c, "hotkey-note", "只在按住热键时打开麦克风，音频不保存。热键录音使用“语音识别”页选择的引擎，选择在线引擎时会把这段音频发送给该服务商。", false)
}
