package ui

import (
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/history"
)

func TestHistoryPageListsCopiesAndDeletes(t *testing.T) {
	var rec intents
	v := testView(&rec)
	var deleted []uint64
	cleared := 0
	v.Actions.DeleteHistory = func(id uint64) { deleted = append(deleted, id) }
	v.Actions.ClearHistory = func() { cleared++ }
	now := time.Date(2026, 3, 5, 15, 0, 0, 0, time.Local)
	v.Snapshot.Now = now
	v.Snapshot.History = []history.Entry{
		{ID: 2, Started: now.Add(-time.Hour), Duration: 754000, Source: "badge", State: history.Cancelled, Reason: "已取消", Live: "取消前说的话"},
		{ID: 1, Started: now.Add(-24 * time.Hour), Duration: 3000, Source: "hotkey", Engine: "Apple 本地", State: history.Inserted, Original: "你好世界", Inserted: "你好，世界。"},
	}
	tt := newTester(v)
	if err := tt.Click("历史记录"); err != nil || v.Page != HistoryPage {
		t.Fatalf("navigate: %v page %d", err, v.Page)
	}
	for _, want := range []string{"取消前说的话", "你好，世界。", "已取消", "已插入", "今天 14:00 · 12:34 · 工牌", "昨天 15:00 · 0:03 · Mac 麦克风 · Apple 本地"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("复制"); err != nil || tt.Clipboard() != "取消前说的话" || !tt.HasText("已复制") {
		t.Fatalf("copy: %v %q", err, tt.Clipboard())
	}
	if err := tt.Click("删除"); err != nil || len(deleted) != 1 || deleted[0] != 2 {
		t.Fatalf("delete: %v %v", err, deleted)
	}
	// Opening an entry shows the recognition original beside the inserted text.
	v.Snapshot.History = v.Snapshot.History[1:]
	tt.Frame()
	if err := tt.Click("展开"); err != nil || !tt.HasText("识别原文") || !tt.HasText("你好世界") {
		t.Fatalf("open: %v %q", err, tt.Texts())
	}
	if err := tt.Click("复制原文"); err != nil || tt.Clipboard() != "你好世界" {
		t.Fatalf("copy original: %v %q", err, tt.Clipboard())
	}
	// Clearing asks first.
	if err := tt.Click("全部清空"); err != nil || cleared != 0 {
		t.Fatal(err, cleared)
	}
	if err := tt.Click("确认清空"); err != nil || cleared != 1 {
		t.Fatal(err, cleared)
	}
}

func TestHistoryPageSearchAndEmpty(t *testing.T) {
	var rec intents
	v := testView(&rec)
	v.Page = HistoryPage
	tt := newTester(v)
	if !tt.HasText("还没有记录：说完一次话后会出现在这里") {
		t.Fatalf("empty: %q", tt.Texts())
	}
	v.Snapshot.History = []history.Entry{{ID: 1, Started: time.Now(), State: history.Inserted, Inserted: "Hello World"}}
	v.historyQuery = "world"
	tt.Frame()
	if !tt.HasText("Hello World") {
		t.Fatalf("search ignores case: %q", tt.Texts())
	}
	v.historyQuery = "missing"
	tt.Frame()
	if !tt.HasText("没有找到包含这段文字的记录") || tt.HasText("Hello World") {
		t.Fatalf("search: %q", tt.Texts())
	}
}

func TestLimitNotice(t *testing.T) {
	s := Snapshot{SpeechActive: true, Phase: "recording", Limit: 30 * time.Minute}
	s.Meter.Frames = uint32((28 * time.Minute) / (20 * time.Millisecond))
	if limitNotice(s) != "" {
		t.Fatal("early notice")
	}
	s.Meter.Frames = uint32((29*time.Minute + 15*time.Second) / (20 * time.Millisecond))
	if got := limitNotice(s); got != "还剩 45 秒：到 30 分钟会自动结束并识别" {
		t.Fatal(got)
	}
	s.Limited, s.Phase = true, "finalizing"
	if got := hudPreview(s); got != "已到 30 分钟上限，已自动结束录音并识别" {
		t.Fatal(got)
	}
	if FormatLimit(230*time.Second) != "3 分 50 秒" || clock(754*time.Second) != "12:34" {
		t.Fatal(FormatLimit(230*time.Second), clock(754*time.Second))
	}
}
