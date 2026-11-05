package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/jev"
)

// jevPresets are providers of Jev the tab fills in with one click; any
// other System One endpoint can be typed.
var jevPresets = []struct {
	key, label string
	config     jev.Config
}{
	{"jev-typesafe", "TypeSafe 官方", jev.Config{Endpoint: jev.DefaultEndpoint, Model: jev.DefaultModel}},
	{"jev-maas", "支付宝 MaaS", jev.Config{Endpoint: "https://maas-api.antdigital.com/v1/systemone", Model: "jev-1.13"}},
}

// jevAccount is the keychain account of the key of c's endpoint, or ""
// while that endpoint cannot be saved.
func jevAccount(c jev.Config) string {
	if _, err := jev.Normalize(c).URL(); err != nil {
		return ""
	}
	return jev.KeyAccount(c)
}

// judge is the Jev tab of the vocabulary page: Jev decides, with a
// probability, whether a span that sounds like a term is the term. It
// runs at TypeSafe or at any provider of its System One API.
func (v *View) judge(c *ui.Context) {
	s := v.Snapshot
	group(c, "jev", "", nil, func() {
		row(c, "jev-enabled", func() {
			label(c, "同音词判断")
			ui.Switch(c, &v.Jev.Enabled).Label("启用 Jev 词库判断")
		})
		row(c, "jev-presets", func() {
			label(c, "服务商")
			current := jev.Normalize(v.Jev)
			for _, p := range jevPresets {
				on := current.Endpoint == p.config.Endpoint && current.Model == p.config.Model
				if smallPill(c, p.key, p.label, pickPill(on, pillPrimary, pillSecondary), nil, false) {
					v.Jev.Endpoint, v.Jev.Model = p.config.Endpoint, p.config.Model
				}
			}
		})
		field(c, "接口地址", func() {
			ui.TextInput(c, &v.Jev.Endpoint).Placeholder(jev.DefaultEndpoint).Grow(1).Label("Jev 接口地址")
		})
		field(c, "模型", func() {
			ui.TextInput(c, &v.Jev.Model).Placeholder(jev.DefaultModel).Grow(1).Label("Jev 模型")
		})
		field(c, "API key", func() {
			ui.TextInput(c, &v.jevSecret).Password().Placeholder(v.keyPlaceholder(jevAccount(v.Jev), "")).Grow(1).Label("Jev API key")
		})
	})
	if v.Jev.Enabled {
		notice(c, "jev-privacy", "命中候选时，识别文字将发送到 "+v.Jev.Host()+"，不含音频和完整词库。", true)
	}
	ui.Row(c).Gap(12).Wrap().GapY(12).Children(func() {
		if pill(c, "jev-delete-key", "删除凭据", pillDanger, nil, nil, false) {
			v.Actions.DeleteJevKey()
		}
		ui.Box(c).Grow(1).MinWidth(24).Height(1).Background(colRule)
		if pill(c, "jev-test", pickText(s.JevTesting, "正在测试…", "测试连接"), pillSecondary, nil, nil, s.JevTesting) {
			v.Actions.TestJev(v.Jev, v.jevSecret)
		}
		if pill(c, "jev-save", "保存设置", pillPrimary, nil, iconArrow, false) {
			v.Actions.ConfigureJev(v.Jev, v.jevSecret)
			v.jevSecret = ""
		}
	})
	if s.JevFeedback != "" && !s.JevTesting && jev.Normalize(s.JevFeedbackFor) == jev.Normalize(v.Jev) {
		feedback(c, s.JevFeedback, s.JevFeedbackOK)
	}
}

func pickPill(cond bool, a, b pillKind) pillKind {
	if cond {
		return a
	}
	return b
}
