package ui

import (
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

func TestPublicWordlistsDefaultsToggleAndSave(t *testing.T) {
	var rec intents
	v := testView(&rec)
	var saved []wordlists.Config
	v.Actions.ConfigurePacks = func(c wordlists.Config) { saved = append(saved, c) }
	v.Page = 1
	tt := newTester(v)
	if err := tt.Click("公共词库"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("通用基础词库 · 96336 词") || !tt.HasText("IT 行业包 · 12854 词") || v.Packs.ITDisabled || v.Packs.BaseDisabled {
		t.Fatalf("default packs: %v", tt.Texts())
	}
	if err := tt.Click("启用 IT 行业包"); err != nil || !v.Packs.ITDisabled {
		t.Fatalf("disable: %v %+v", err, v.Packs)
	}
	if err := tt.Click("保存"); err != nil || len(saved) != 1 || !saved[0].ITDisabled || saved[0].BaseDisabled {
		t.Fatalf("save: %v %+v", err, saved)
	}
	v.Snapshot.PacksFeedback = "词库设置已保存，下次语音生效。"
	v.Snapshot.PacksFeedbackFor = v.Packs
	v.Snapshot.PacksFeedbackOK = true
	tt.Frame()
	if !tt.HasText("词库设置已保存，下次语音生效。") {
		t.Fatal("missing save feedback")
	}
	if err := tt.Click("启用 IT 行业包"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("词库设置已保存，下次语音生效。") {
		t.Fatal("stale feedback")
	}
}
