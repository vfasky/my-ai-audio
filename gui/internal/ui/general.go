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
	group(c, "startup", "", nil, func() {
		row(c, "open-at-login", func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
				ui.Icon(c, iconPower).Size(15, 15).TextColor(pick(s.OpenAtLogin, colGreen, colSubtle))
			})
			label(c, "登录时打开")
			on := s.OpenAtLogin
			if ui.Switch(c, &on).Label("登录时打开伊言").Changed() {
				v.Actions.SetOpenAtLogin(on)
			}
		})
	})
	if s.LoginApproval {
		ui.Row(c.Key("login-approval")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "需在“系统设置 › 通用 › 登录项”中允许伊言。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "login-items", "打开登录项", pillPrimary, iconArrow, false) {
				v.Actions.OpenLoginItems()
			}
		})
	} else if s.LoginFeedback != "" {
		feedback(c, s.LoginFeedback, s.LoginFeedbackOK)
	}
	v.hotkey(c)
}

// customSegment is the segment that opens the modifier picker.
var customSegment = len(hotkey.Choices)

// hotkey chooses the chord held to record with the Mac microphone: a preset,
// a custom combination of modifiers, or off.
func (v *View) hotkey(c *ui.Context) {
	s := v.Snapshot
	current := hotkey.Normalize(s.Hotkey)
	labels := make([]string, 0, len(hotkey.Choices)+2)
	for _, ch := range hotkey.Choices {
		labels = append(labels, ch.Label)
	}
	labels = append(labels, "自定义", "关闭")
	i := len(labels) - 1
	if hotkey.IsCustom(current) && !v.hotkeyCustom {
		// A custom chord saved earlier: open the picker on it.
		v.hotkeyCustom = true
		v.hotkeyDraft, _ = hotkey.Parse(current)
	}
	switch {
	case current == hotkey.Off:
	case hotkey.IsCustom(current) || v.hotkeyCustom:
		i = customSegment
	default:
		for j, ch := range hotkey.Choices {
			if ch.ID == current {
				i = j
			}
		}
	}
	custom := i == customSegment
	if !custom {
		v.hotkeyCustom = false
	}
	group(c, "hotkey", "", nil, func() {
		row(c, "hotkey-chord", func() {
			ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
				ui.Icon(c, iconWave).Size(15, 15).TextColor(pick(hotkey.Enabled(current), colOrange, colSubtle))
			})
			label(c, "按住说话")
			if segmented(c, "hotkey-seg", &i, labels) {
				v.chooseHotkey(i, current)
			}
		})
		if custom {
			row(c, "hotkey-mods", func() {
				label(c, "组合键")
				draft := v.hotkeyDraft
				for _, k := range hotkey.Keys {
					if modChip(c, "hotkey-mod-"+k.Name, k.Symbol, draft&k.Mod != 0) {
						v.toggleHotkeyMod(k.Mod)
					}
				}
			})
		}
	})
	if custom && v.hotkeyDraft.Count() < hotkey.MinKeys {
		notice(c, "hotkey-min", "至少选择两个修饰键，才不会和平时的打字冲突。", true)
	}
	if !hotkey.Enabled(current) {
		return
	}
	if s.HotkeyBusy {
		ui.Row(c.Key("hotkey-busy")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "另一个伊言正在使用热键，退出后自动接管。").
				FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
		})
	}
	switch st := s.Permissions[permission.Microphone]; st {
	case permission.Granted:
	case permission.Denied, permission.Restricted:
		ui.Row(c.Key("mic-denied")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconWarn).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "麦克风未授权").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "mic-settings", "打开系统设置", pillPrimary, iconArrow, false) {
				v.Actions.OpenSettings(permission.Microphone)
			}
		})
	default:
		ui.Row(c.Key("mic-request")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
			ui.Icon(c, iconShield).Size(16, 16).TextColor(colOrange)
			ui.Text(c, "需要麦克风权限").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
			if smallPill(c, "mic-request", "授权麦克风", pillPrimary, nil, false) {
				v.Actions.RequestPermission(permission.Microphone)
			}
		})
	}
}

// chooseHotkey applies the segment i of the chord control. Custom starts from
// the chord in use, so that opening it never changes the shortcut by itself.
func (v *View) chooseHotkey(i int, current string) {
	switch {
	case i < len(hotkey.Choices):
		v.hotkeyCustom = false
		v.Actions.SetHotkey(hotkey.Choices[i].ID)
	case i == customSegment:
		v.hotkeyCustom = true
		m, ok := hotkey.Parse(current)
		if !ok {
			m, _ = hotkey.Parse(hotkey.Default)
		}
		v.hotkeyDraft = m
		v.Actions.SetHotkey(hotkey.ID(m))
	default:
		v.hotkeyCustom = false
		v.Actions.SetHotkey(hotkey.Off)
	}
}

// toggleHotkeyMod flips one modifier of the custom chord; the chord is saved
// once it has enough modifiers, and the previous one stays active until then.
func (v *View) toggleHotkeyMod(m hotkey.Mods) {
	v.hotkeyDraft ^= m
	if id := hotkey.ID(v.hotkeyDraft); id != "" {
		v.Actions.SetHotkey(id)
	}
}

// modChip is a toggle for one modifier key.
func modChip(c *ui.Context, key, symbol string, on bool) bool {
	b := ui.ButtonBase(c.Key(key)).Height(28).Width(40).Radius(14).Shrink(0).Justify(ui.Center).Label(symbol)
	switch {
	case on:
		b.Background(ui.Hex("#FFFFFF")).TextColor(colCanvas)
	case b.Hovered():
		b.Background(colPillHover).TextColor(colText)
	default:
		b.Background(colPill).TextColor(colMuted)
	}
	b.Children(func() { ui.Text(c, symbol).FontSize(14).FontWeight(500).SingleLine() })
	return b.Clicked()
}
