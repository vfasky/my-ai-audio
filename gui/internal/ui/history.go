package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/vfasky/my-ai-audio/gui/internal/history"
)

// Pages by index; pages holds them in this order.
const (
	VoicePage = iota
	HistoryPage
	VocabularyPage
	RecognitionPage
	RefinementPage
	FirmwarePage
)

// historyStep is how many entries the history page lists at a time.
const historyStep = 50

var (
	iconClock = icon(`<circle cx="8" cy="8" r="6.2"/><path d="M8 4.6V8l2.4 1.6"/>`)
	iconCopy  = icon(`<rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5V4A1.5 1.5 0 0 0 9 2.5H4A1.5 1.5 0 0 0 2.5 4v5A1.5 1.5 0 0 0 4 10.5h1.5"/>`)
)

// historyState is the label and color of an entry's state.
func historyState(e history.Entry) (string, ui.Color) {
	switch e.State {
	case history.Recording:
		return "进行中", colOrange
	case history.Inserted:
		if strings.HasPrefix(e.Reason, "clipboard") {
			return "在剪贴板", colOrange
		}
		return "已插入", colGreen
	case history.Cancelled:
		return "已取消", colMuted
	}
	return "未完成", colRed
}

// historySource names where the audio came from.
func historySource(s string) string {
	if s == "hotkey" {
		return "Mac 麦克风"
	}
	return "工牌"
}

// HistoryWhen tells when an entry started, relative to now.
func HistoryWhen(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	_, m1, d1 := t.Date()
	switch {
	case sameDay(t, now):
		return "今天 " + t.Format("15:04")
	case sameDay(t.AddDate(0, 0, 1), now):
		return "昨天 " + t.Format("15:04")
	}
	return fmt.Sprintf("%d月%d日 %s", m1, d1, t.Format("15:04"))
}

// historyReason explains how an entry that did not insert ended.
func historyReason(e history.Entry) string {
	switch {
	case e.State == history.Inserted && strings.HasPrefix(e.Reason, "clipboard"):
		return "没有直接插入，文字留在了剪贴板。"
	case e.State == history.Inserted, e.Reason == "":
		return ""
	}
	return e.Reason
}

// matches reports whether e holds query, ignoring case.
func matches(e history.Entry, query string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Text()+"\n"+e.Original+"\n"+e.Live), query)
}

// history lists the dictations of the last days, newest first, so that what
// was said survives a cancelled, failed or misplaced insertion.
func (v *View) history(c *ui.Context) {
	s := v.Snapshot
	notice(c, "history-note", fmt.Sprintf("每次语音识别出的文字都保存在本机，保留 %d 天，不保存音频。取消、失败或没插入到想要的位置时，可以在这里找回并复制。", int(history.Retention/(24*time.Hour))), false)
	ui.Row(c).Gap(10).Children(func() {
		ui.TextInput(c, &v.historyQuery).Placeholder("搜索文字").Grow(1).Label("搜索历史记录")
		if v.historyConfirm {
			if smallPill(c, "history-clear-cancel", "取消", pillGhost, nil, false) {
				v.historyConfirm = false
			}
			if smallPill(c, "history-clear-confirm", "确认清空", pillDanger, nil, false) {
				v.historyConfirm = false
				v.historyOpen = 0
				v.Actions.ClearHistory()
			}
		} else if smallPill(c, "history-clear", "全部清空", pillGhost, nil, len(s.History) == 0) {
			v.historyConfirm = true
		}
	})
	query := strings.ToLower(strings.TrimSpace(v.historyQuery))
	shown := max(v.historyShown, historyStep)
	var list []history.Entry
	more := false
	for _, e := range s.History {
		if !matches(e, query) {
			continue
		}
		if len(list) == shown {
			more = true
			break
		}
		list = append(list, e)
	}
	group(c, "history", "", nil, func() {
		if len(list) == 0 {
			row(c, "history-none", func() {
				ui.Text(c, pick2(query != "", "没有找到包含这段文字的记录", "还没有记录：说完一次话后会出现在这里")).FontSize(13).TextColor(colMuted)
			})
		}
		now := time.Now()
		if s.Now.Unix() > 0 {
			now = s.Now
		}
		for _, e := range list {
			v.historyEntry(c, e, now)
		}
	})
	if more && smallPill(c, "history-more", "显示更多", pillSecondary, nil, false) {
		v.historyShown = shown + historyStep
	}
}

// historyEntry is one dictation: when, how long, how it ended and its
// text, folded to three lines until opened.
func (v *View) historyEntry(c *ui.Context, e history.Entry, now time.Time) {
	key := fmt.Sprint("history-", e.ID)
	open := v.historyOpen == e.ID
	ui.Column(c.Key(key)).Padding(12, 16).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			state, color := historyState(e)
			statusDot(c, color)
			ui.Text(c, state).FontSize(12).FontWeight(600).TextColor(color).SingleLine()
			meta := []string{HistoryWhen(e.Started, now), clock(time.Duration(e.Duration) * time.Millisecond), historySource(e.Source)}
			if e.Engine != "" {
				meta = append(meta, e.Engine)
			}
			ui.Text(c, strings.Join(meta, " · ")).FontSize(12).TextColor(colMuted).SingleLine().Grow(1).MinWidth(0)
			if v.historyCopied == e.ID {
				ui.Text(c, "已复制").FontSize(12).TextColor(colGreen).SingleLine()
			}
			if smallPill(c, "copy", "复制", pillSecondary, iconCopy, false) {
				c.WriteClipboard(e.Text())
				v.historyCopied = e.ID
			}
			if smallPill(c, "toggle", pick2(open, "收起", "展开"), pillGhost, nil, false) {
				v.historyOpen = pickID(open, 0, e.ID)
			}
			if smallPill(c, "delete", "删除", pillDanger, nil, e.State == history.Recording) {
				v.Actions.DeleteHistory(e.ID)
			}
		})
		text := ui.Text(c, e.Text()).FontSize(14).LineHeight(1.5).Selectable()
		if !open {
			text.MaxLines(3)
		}
		if !open {
			return
		}
		if reason := historyReason(e); reason != "" {
			ui.Text(c, reason).FontSize(12).TextColor(colMuted)
		}
		if e.Original != "" && e.Original != e.Text() {
			ui.Text(c, "识别原文").FontSize(12).FontWeight(600).TextColor(colSubtle).Margin(4, 0, 0, 0)
			ui.Text(c, e.Original).FontSize(13).LineHeight(1.5).TextColor(colSubtle).Selectable()
			ui.Row(c).Children(func() {
				if smallPill(c, "copy-original", "复制原文", pillGhost, iconCopy, false) {
					c.WriteClipboard(e.Original)
					v.historyCopied = e.ID
				}
			})
		}
	})
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func pickID(cond bool, a, b uint64) uint64 {
	if cond {
		return a
	}
	return b
}
