package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/history"
	appui "github.com/vfasky/my-ai-audio/gui/internal/ui"
)

// historyCheckpoint is how often a running session's words are saved, so
// that a crash loses at most this much.
const historyCheckpoint = 10 * time.Second

// trayRecent is how many dictations the menu bar offers to copy.
const trayRecent = 5

// dictations keeps the history of the app: the entry of the running
// session, and a writer that saves off the main thread.
type dictations struct {
	store *history.Store
	path  string
	// run is the entry of session gen while it runs; cancelled: the
	// session ended by cancel rather than insertion.
	run       uint64
	gen       uint32
	cancelled bool
	dirty     bool
	saved     time.Time
	// The writer: the latest file waiting, a kick, and a lock so that the
	// quit flush and the writer never write at once.
	mu      sync.Mutex
	pending []byte
	kick    chan struct{}
}

// loadHistory reads the history; an unreadable file is set aside rather
// than overwritten.
func (a *application) loadHistory() {
	h := &dictations{path: filepath.Join(a.directory, "history.json"), kick: make(chan struct{}, 1)}
	s, err := history.Load(h.path, time.Now())
	if err != nil {
		if _, statErr := os.Stat(h.path); statErr == nil {
			_ = os.Rename(h.path, h.path+".bad")
		}
		a.view.Snapshot.Status = "历史记录无法读取，已另存为 history.json.bad 并重新开始"
		s = &history.Store{}
	}
	h.store = s
	a.history = h
	go h.writer(a.ctx)
	a.syncHistory()
}

// writer saves the latest history handed to it.
func (h *dictations) writer(ctx context.Context) {
	done := ctxOrBackground(ctx).Done()
	for {
		select {
		case <-done:
			return
		case <-h.kick:
			h.write()
		}
	}
}

func (h *dictations) write() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pending == nil {
		return
	}
	_ = history.Write(h.path, h.pending)
	h.pending = nil
}

// save hands the history to the writer.
func (h *dictations) save(now time.Time) {
	b, err := h.store.Marshal()
	if err != nil {
		return
	}
	h.mu.Lock()
	h.pending = b
	h.mu.Unlock()
	h.dirty, h.saved = false, now
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

// flushHistory writes the history now, as the app quits.
func (a *application) flushHistory() {
	if a.history == nil {
		return
	}
	a.trackHistory()
	a.history.save(time.Now())
	a.history.write()
}

// beginHistory opens the entry of session id.
func (a *application) beginHistory(id uint32) {
	h := a.history
	if h == nil {
		return
	}
	source := "badge"
	if a.coordinator.Mic {
		source = "hotkey"
	}
	engine := "Apple 本地"
	if a.config.ASR.Online() {
		engine = asr.ChoiceOf(a.config.ASR).Label
	}
	h.run = h.store.Begin(time.Now(), source, engine)
	h.gen, h.cancelled, h.dirty = id, false, false
	h.saved = time.Now()
}

// noteCancel marks the running entry as ended by a cancel effect.
func (a *application) noteCancel(id uint32) {
	if a.history != nil && a.history.run != 0 && a.history.gen == id {
		a.history.cancelled = true
	}
}

// trackHistory copies the running session into its entry, closes it when
// the session ended, and saves at checkpoints and at the end.
func (a *application) trackHistory() {
	h := a.history
	if h == nil || h.run == 0 {
		return
	}
	c := &a.coordinator
	now := time.Now()
	ended := !c.Active || c.Generation != h.gen
	changed, timed := false, false
	var current history.Entry
	h.store.Update(h.run, now, func(e *history.Entry) {
		before := *e
		e.Duration = c.Meter.Duration().Milliseconds()
		if c.Generation == h.gen {
			if c.Partial != "" {
				e.Live = c.Partial
			}
			if c.Original != "" {
				e.Original = c.Original
			}
		}
		if ended {
			switch {
			case h.cancelled:
				e.State, e.Reason = cancelState(c.Status)
			default:
				e.State, e.Inserted = history.Inserted, c.Refined
				if strings.HasPrefix(c.Status, "clipboard") {
					e.Reason = c.Status
				}
			}
		}
		timed = e.Duration != before.Duration
		before.Duration, before.Updated = e.Duration, e.Updated
		changed = *e != before
		current = *e
	})
	switch {
	case changed:
		h.dirty = true
		a.syncHistory()
	case timed:
		// Only the duration moved, as every audio frame does: patch the
		// shown entry rather than list the history again.
		if list := a.view.Snapshot.History; len(list) > 0 && list[0].ID == current.ID {
			list[0].Duration = current.Duration
		}
	}
	if !ended {
		if h.dirty && now.Sub(h.saved) >= historyCheckpoint {
			h.save(now)
		}
		return
	}
	if e, ok := h.store.Get(h.run); ok && e.Empty() {
		h.store.Delete(h.run)
	}
	h.run = 0
	h.store.Prune(now)
	h.save(now)
	a.syncHistory()
	a.updateTray()
}

// cancelState is how a session that was cancelled ended, with its reason
// in the interface's language.
func cancelState(reason string) (string, string) {
	switch reason {
	case "已取消", "Cancelled":
		return history.Cancelled, "已取消"
	case "Application exiting":
		return history.Failed, "伊言在本次语音结束前退出"
	case "Disconnected; transcript not inserted":
		return history.Failed, "设备断开，文字没有插入"
	case "Processing timeout":
		return history.Failed, "识别或润色超时"
	case "Speech start timeout":
		return history.Failed, "语音识别启动超时"
	case "Empty transcription":
		return history.Failed, "没有识别出文字"
	case "Device error":
		return history.Failed, "设备出错"
	case "":
		return history.Failed, "未完成"
	}
	return history.Failed, reason
}

// syncHistory shows the entries with words, newest first.
func (a *application) syncHistory() {
	if a.history == nil {
		return
	}
	a.view.Snapshot.History = a.history.store.Recent("", 0)
	a.mark(paintView)
}

func (a *application) deleteHistory(id uint64) {
	h := a.history
	if h == nil || id == h.run {
		return
	}
	if h.store.Delete(id) {
		h.save(time.Now())
		a.syncHistory()
		a.updateTray()
	}
	a.flush()
}

func (a *application) clearHistory() {
	h := a.history
	if h == nil {
		return
	}
	h.store.Clear(h.run)
	h.save(time.Now())
	a.syncHistory()
	a.updateTray()
	a.flush()
}

// showHistory opens the main window on the history page.
func (a *application) showHistory() {
	a.view.Page = appui.HistoryPage
	a.showMain()
	a.refresh()
}

// copyText puts text on the clipboard.
func copyText(text string) error {
	return mygo.Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(text)))))
}

// trayLabel is a dictation as the menu bar lists it.
func trayLabel(e history.Entry, now time.Time) string {
	text := strings.Join(strings.Fields(e.Text()), " ")
	if utf8.RuneCountInString(text) > 24 {
		text = string([]rune(text)[:24]) + "…"
	}
	return appui.HistoryWhen(e.Started, now) + "  " + text
}

// recentMenu lists the latest dictations to copy, and the history page.
func (a *application) recentMenu() []*mygo.MenuItem {
	var items []*mygo.MenuItem
	if a.history != nil {
		now := time.Now()
		for _, e := range a.history.store.Recent("", trayRecent) {
			text := e.Text()
			items = append(items, &mygo.MenuItem{Label: trayLabel(e, now), ToolTip: "复制这段文字", Click: func(*mygo.MenuItem, *mygo.Window) {
				if copyText(text) != nil {
					a.view.Snapshot.Status = "无法复制到剪贴板"
					a.refresh()
				}
			}})
		}
	}
	if len(items) == 0 {
		items = append(items, &mygo.MenuItem{Label: "暂无记录", Disabled: true})
	}
	return append(items, mygo.Separator(), &mygo.MenuItem{Label: "显示全部历史记录…", Click: func(*mygo.MenuItem, *mygo.Window) { a.showHistory() }})
}

// updateTray refreshes the recent dictations of the menu bar.
func (a *application) updateTray() {
	if a.tray != nil {
		a.tray.SetMenu(a.trayMenu())
	}
}
