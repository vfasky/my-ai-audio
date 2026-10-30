package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/permission"
)

// PermissionsPage is the index of the permissions onboarding in pages.
const PermissionsPage = 5

var permissionIcons = map[permission.Name]*ui.SVG{
	permission.Bluetooth:     iconBolt,
	permission.Speech:        iconWave,
	permission.Accessibility: iconCursor,
	permission.Automation:    iconSparkle,
	permission.Microphone:    iconWave,
}

func permissionTone(s permission.State) (fg, bg ui.Color) {
	switch s {
	case permission.Granted:
		return colGreen, ui.RGBA(48, 209, 88, 0.14)
	case permission.Denied, permission.Restricted:
		return ui.Hex("#FF6961"), colRedSoft
	}
	return colOrange, colOrangeSoft
}

// permissionBanner sends the user to the onboarding before a session can fail.
func (v *View) permissionBanner(c *ui.Context, missing []permission.Name) {
	names := make([]string, len(missing))
	for i, n := range missing {
		names[i] = permission.Title(n)
	}
	ui.Row(c.Key("permission-banner")).Padding(12, 12, 12, 16).Gap(12).Radius(14).Background(colOrangeSoft).Children(func() {
		ui.Icon(c, iconShield).Size(16, 16).TextColor(colOrange)
		ui.Text(c, "开始使用前还需授权："+strings.Join(names, "、")+"。").FontSize(13).TextColor(ui.Hex("#FFC4A3")).Grow(1)
		if smallPill(c, "goto-permissions", "前往授权", pillPrimary, nil, false) {
			v.Page = PermissionsPage
		}
	})
}

func (v *View) permissions(c *ui.Context) {
	s := v.Snapshot.Permissions
	missing := s.Missing()
	required := 0
	for _, n := range permission.Names {
		if permission.Required(n) {
			required++
		}
	}
	ui.Column(c).Background(colCard).Radius(26).Padding(26, 28).Gap(16).Children(func() {
		ui.Row(c).Gap(16).Children(func() {
			done := len(missing) == 0
			ui.Box(c).Size(52, 52).Radius(16).Background(colWell).Border(1, colHairline).Center().Children(func() {
				ui.Icon(c, iconShield).Size(24, 24).TextColor(pick(done, colGreen, colOrange))
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				title := "完成授权后即可开始使用"
				if done {
					title = "必需权限均已授权"
				}
				ui.Text(c, title).FontSize(19).FontWeight(500)
				ui.Text(c, fmt.Sprintf("必需权限已完成 %d / %d 项。授权后无需重启，状态会自动刷新。", required-len(missing), required)).
					FontSize(13).TextColor(colSubtle).FontFeatures("tnum")
			})
		})
		ui.Progress(c, float64(required-len(missing))/float64(required)).Label("必需权限进度")
	})
	group(c, "required", "必需", nil, func() {
		for _, n := range permission.Names {
			if permission.Required(n) {
				v.permissionRow(c, n, s[n])
			}
		}
	})
	group(c, "optional", "可选", nil, func() {
		for _, n := range permission.Names {
			if !permission.Required(n) {
				v.permissionRow(c, n, s[n])
			}
		}
	})
	notice(c, "permission-note", "辅助功能与自动化需要在系统设置中打开 go-voice 的开关。只有按住麦克风热键时才会打开 Mac 麦克风；关闭热键后不需要麦克风权限。", false)
}

func (v *View) permissionRow(c *ui.Context, n permission.Name, st permission.State) {
	row(c, string(n), func() {
		ui.Box(c).Size(34, 34).Radius(17).Background(colPill).Center().Children(func() {
			ui.Icon(c, permissionIcons[n]).Size(15, 15).TextColor(pick(st == permission.Granted, colGreen, colSubtle))
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, permission.Title(n)).FontSize(14).FontWeight(500).SingleLine()
			ui.Text(c, permission.Purpose(n)).FontSize(12).TextColor(colMuted).LineHeight(1.4)
			if n == permission.Accessibility && st != permission.Granted {
				// macOS keeps the old grant after a re-signed build but refuses it.
				ui.Text(c, "若系统设置中已打开仍显示未授权：先在列表中选中 go-voice 并点“−”移除，再点“授权”重新添加。").
					FontSize(11.5).TextColor(ui.Hex("#FF9A62")).LineHeight(1.4)
			}
		})
		fg, bg := permissionTone(st)
		ui.Badge(c, permission.Label(st)).Background(bg).TextColor(fg)
		if st == permission.Granted {
			return
		}
		// Bluetooth asks when the helper starts; once answered, only System
		// Settings can change any grant.
		if n != permission.Bluetooth && permission.Prompts(st) {
			if smallPill(c, "request", "授权", pillPrimary, nil, false) {
				v.Actions.RequestPermission(n)
			}
			return
		}
		if smallPill(c, "settings", "打开系统设置", pillSecondary, iconArrow, false) {
			v.Actions.OpenSettings(n)
		}
	})
}
