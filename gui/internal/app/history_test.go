package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/asr"
	"github.com/vfasky/my-ai-audio/gui/internal/history"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
)

func historyApp(t *testing.T) *application {
	t.Helper()
	a := &application{directory: t.TempDir(), onboarded: true}
	a.config.Refinement = refinement.Config{Engine: "local", Mode: "off"}
	a.loadHistory()
	return a
}

// startMic runs a microphone session up to recording.
func startMic(a *application) uint32 {
	a.effects(a.coordinator.MicStart(time.Now()))
	id := a.coordinator.Generation
	a.onSpeech(bridge.Event{Type: "ready", SessionID: id})
	return id
}

func TestHistoryKeepsInsertedAndCancelledDictations(t *testing.T) {
	a := historyApp(t)
	id := startMic(a)
	a.onSpeech(bridge.Event{Type: "mic", SessionID: id, Payload: make([]byte, 640)})
	a.onSpeech(bridge.Event{Type: "partial", SessionID: id, Text: "说到一半"})
	if h := a.view.Snapshot.History; len(h) != 1 || h[0].State != history.Recording || h[0].Live != "说到一半" || h[0].Source != "hotkey" || h[0].Engine != "Apple 本地" {
		t.Fatalf("running entry %+v", h)
	}
	// Cancelled: the words stay, with the reason.
	a.effects(a.coordinator.Cancel("已取消"))
	h := a.view.Snapshot.History
	if len(h) != 1 || h[0].State != history.Cancelled || h[0].Reason != "已取消" || h[0].Text() != "说到一半" || h[0].Duration != 20 {
		t.Fatalf("cancelled %+v", h)
	}
	// Inserted: original and inserted text.
	id = startMic(a)
	a.effects(a.coordinator.MicEnd(id, time.Now()))
	a.onSpeech(bridge.Event{Type: "final", SessionID: id, Text: "第二段话"})
	a.onSpeech(bridge.Event{Type: "inserted", SessionID: id, Text: "inserted"})
	h = a.view.Snapshot.History
	if len(h) != 2 || h[0].State != history.Inserted || h[0].Original != "第二段话" || h[0].Inserted != "第二段话" || h[0].Reason != "" {
		t.Fatalf("inserted %+v", h)
	}
	// A session without words leaves nothing.
	startMic(a)
	a.effects(a.coordinator.Cancel("已取消"))
	if len(a.history.store.Entries) != 2 {
		t.Fatalf("empty session kept: %+v", a.history.store.Entries)
	}
	a.flushHistory()
	b, err := os.ReadFile(filepath.Join(a.directory, "history.json"))
	if err != nil || !strings.Contains(string(b), "说到一半") || !strings.Contains(string(b), "第二段话") {
		t.Fatalf("saved %v %s", err, b)
	}
	// Delete and clear, then reload.
	a.deleteHistory(h[1].ID)
	if len(a.view.Snapshot.History) != 1 {
		t.Fatal("delete")
	}
	a.clearHistory()
	a.flushHistory()
	if s, err := history.Load(filepath.Join(a.directory, "history.json"), time.Now()); err != nil || len(s.Entries) != 0 {
		t.Fatal(err, s.Entries)
	}
}

func TestHistoryNamesFailures(t *testing.T) {
	a := historyApp(t)
	a.config.ASR = asr.Choices[1].Config
	id := startMic(a)
	a.onSpeech(bridge.Event{Type: "partial", SessionID: id, Text: "断开前"})
	a.effects(a.coordinator.Cancel("Processing timeout"))
	h := a.view.Snapshot.History
	if len(h) != 1 || h[0].State != history.Failed || h[0].Reason != "识别或润色超时" || h[0].Engine != asr.Choices[1].Label {
		t.Fatalf("%+v", h)
	}
}

func TestHistoryCorruptFileIsSetAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	_ = os.WriteFile(path, []byte("not json"), 0o600)
	a := &application{directory: dir}
	a.loadHistory()
	if a.history == nil || !strings.Contains(a.view.Snapshot.Status, "history.json.bad") {
		t.Fatal(a.view.Snapshot.Status)
	}
	if b, err := os.ReadFile(path + ".bad"); err != nil || string(b) != "not json" {
		t.Fatal(err, string(b))
	}
}

func TestLongDictationSkipsRefinement(t *testing.T) {
	a := &application{config: settings{Refinement: refinement.Config{Engine: "local", Mode: "light"}}}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 5, "finalizing"
	long := strings.Repeat("很长的录音内容。", refinement.MaxRunes/8+1)
	a.effects(a.coordinator.Speech("final", 5, long, time.Now()))
	if a.coordinator.Phase != "inserting" || a.coordinator.Refined != long || a.refineCancel != nil {
		t.Fatalf("phase %q", a.coordinator.Phase)
	}
	if a.view.Snapshot.Status != longTextStatus {
		t.Fatal(a.view.Snapshot.Status)
	}
}

func TestEngineLimitsTheRecording(t *testing.T) {
	a := &application{}
	for _, c := range asr.Choices {
		a.config.ASR = c.Config
		a.applyASR()
		want := 30 * time.Minute
		switch {
		case c.Config.Engine == asr.EngineOmni:
			want = asr.OmniMaxDuration
		case c.Config.Online() && !c.Config.Streaming():
			want = asr.WholeMaxDuration
		}
		if got := a.coordinator.LimitDuration(); got != want {
			t.Fatalf("%s: %v", c.ID, got)
		}
	}
}
