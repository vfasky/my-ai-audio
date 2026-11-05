package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTailTextKeepsNewestWords(t *testing.T) {
	if tailText("短句", 5) != "短句" {
		t.Fatal("short text changed")
	}
	if got := tailText("一二三四五六七", 3); got != "…五六七" {
		t.Fatalf("tail %q", got)
	}
}

func TestHUDLineWordsAndPreviewFollowPhase(t *testing.T) {
	cases := []struct {
		s                    Snapshot
		line, words, preview string
	}{
		{Snapshot{Phase: "starting"}, "正在准备…", "", ""},
		{Snapshot{Phase: "recording"}, "请说话…", "", ""},
		{Snapshot{Phase: "recording", Partial: "你好"}, "请说话…", "你好", ""},
		{Snapshot{Phase: "finalizing", Partial: "你好世界"}, "正在识别…", "你好世界", ""},
		{Snapshot{Phase: "refining", Partial: "你好", Original: "你好。"}, "AI 润色中…  可直接插入原文", "", "你好。"},
		{Snapshot{Phase: "ready", Status: "已取消", Partial: "旧文字"}, "已取消", "", ""},
	}
	for _, c := range cases {
		if line, words, preview := hudLine(c.s), hudWords(c.s), hudPreview(c.s); line != c.line || words != c.words || preview != c.preview {
			t.Fatalf("%+v: %q %q %q", c.s, line, words, preview)
		}
	}
}

func TestHUDShowsWordsInTheCapsule(t *testing.T) {
	var rec intents
	v := testView(&rec)
	long := strings.Repeat("一二三四五六七八九十", 12) + "最新的话"
	v.Snapshot.SpeechActive, v.Snapshot.Phase, v.Snapshot.Partial = true, "recording", long
	tt := ui.NewTester(v.DrawHUD, HUDWidth, HUDHeight)
	if r, ok := tt.Find(tailText(long, tickerTail)); !ok || r.W != tickerWidth || tt.HasText("请说话…") {
		t.Fatalf("words %q %+v", tt.Texts(), r)
	}
	v.Snapshot.Partial = ""
	tt.Frame()
	if !tt.HasText("请说话…") {
		t.Fatalf("placeholder %q", tt.Texts())
	}
}

func TestSlideMovesNewWordsIn(t *testing.T) {
	var s slide
	start := time.Unix(100, 0)
	if got := s.step(120, start); got != 120 {
		t.Fatalf("first words slid: %v", got)
	}
	mid := s.step(180, start.Add(35*time.Millisecond))
	if mid <= 120 || mid >= 180 {
		t.Fatalf("new words jumped: %v", mid)
	}
	if got := s.step(180, start.Add(600*time.Millisecond)); got != 180 {
		t.Fatalf("slide did not settle: %v", got)
	}
	if got := s.step(40, start.Add(3*time.Second)); got != 40 {
		t.Fatalf("a new line after a pause slid: %v", got)
	}
}

func TestHUDCommitOnlyWhenAllowed(t *testing.T) {
	var rec intents
	commits := 0
	v := testView(&rec)
	v.Actions.Commit = func() { commits++ }
	v.Snapshot.SpeechActive, v.Snapshot.Phase, v.Snapshot.Partial = true, "recording", "实现波形的"
	tt := ui.NewTester(v.DrawHUD, HUDWidth, HUDHeight)
	_ = tt.Click("立即插入原文")
	if commits != 0 {
		t.Fatal("commit while recording")
	}
	if err := tt.Click("取消"); err != nil || rec.cancel != 1 {
		t.Fatalf("cancel: %v", err)
	}
	v.Snapshot.Phase, v.Snapshot.Original, v.Snapshot.CanCommit = "refining", "实现波形的，能小小文字的？", true
	tt.Frame()
	if err := tt.Click("立即插入原文"); err != nil || commits != 1 {
		t.Fatalf("commit: %v", err)
	}
	if dir := os.Getenv("GOVOICE_UI_SNAPSHOTS"); dir != "" {
		v.Snapshot.Phase, v.Snapshot.Partial = "recording", "今天下午三点我们在会议室讨论 Go Voice 的发布计划，先确认签名和公证流程，再安排第一批用户测试自动更新"
		for i := range v.Snapshot.Meter.Bins {
			v.Snapshot.Meter.Bins[i] = []float64{0.05, 0.3, 0.12, 0.5, 0.2}[i%5]
		}
		tt.SetScale(2)
		tt.Frame()
		tt.Advance(time.Second)
		f, err := os.Create(filepath.Join(dir, "hud.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHUDOffersToUndoLearnedWords(t *testing.T) {
	var rec intents
	v := testView(&rec)
	undone := 0
	v.Actions.UndoLearned = func() { undone++ }
	v.Snapshot.Learned = []string{"Plaud"}
	tt := ui.NewTester(v.DrawHUD, HUDWidth, HUDHeight)
	if !tt.HasText("「Plaud」已加入词库") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("撤销"); err != nil {
		t.Fatal(err)
	}
	if undone != 1 {
		t.Fatalf("undo %d", undone)
	}
	if learnedLine([]string{"A1", "Bx"}) != "「A1」「Bx」已加入词库" {
		t.Fatal(learnedLine([]string{"A1", "Bx"}))
	}
}
