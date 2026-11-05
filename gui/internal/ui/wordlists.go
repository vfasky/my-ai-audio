package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func (v *View) wordlists(c *ui.Context) {
	group(c, "public-wordlists", "", nil, func() {
		for _, source := range wordlists.Catalog() {
			row(c, "pack-"+source.ID, func() {
				name, toggle := "通用基础词库", "启用通用基础词库"
				disabled := &v.Packs.BaseDisabled
				if source.ID == "it" {
					name, toggle = "IT 行业包", "启用 IT 行业包"
					disabled = &v.Packs.ITDisabled
				}
				ui.Text(c, fmt.Sprintf("%s · %d 词", name, source.Terms)).FontSize(14).FontWeight(500).Grow(1).MinWidth(0)
				enabled := !*disabled
				ui.Switch(c, &enabled).Label(toggle)
				*disabled = !enabled
			})
		}
	})
	ui.Row(c).Justify(ui.End).Children(func() {
		if pill(c, "save-packs", "保存", pillPrimary, nil, nil, false) {
			v.Actions.ConfigurePacks(v.Packs)
		}
	})
	s := v.Snapshot
	if s.PacksFeedback != "" && s.PacksFeedbackFor == v.Packs {
		feedback(c, s.PacksFeedback, s.PacksFeedbackOK)
	}
}
