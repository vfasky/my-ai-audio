package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLifecycleSaveAndLoad(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	path := filepath.Join(t.TempDir(), "sub", "history.json")
	s, err := Load(path, now)
	if err != nil || len(s.Entries) != 0 {
		t.Fatal(err, s.Entries)
	}
	id := s.Begin(now, "badge", "Apple 本地")
	if len(s.Recent("", 0)) != 0 {
		t.Fatal("empty entry listed")
	}
	s.Update(id, now.Add(time.Second), func(e *Entry) { e.Live = "你好" })
	s.Update(id, now.Add(2*time.Second), func(e *Entry) { e.Original, e.Inserted, e.State = "你好世界", "你好，世界。", Inserted })
	other := s.Begin(now.Add(3*time.Second), "hotkey", "")
	s.Update(other, now.Add(4*time.Second), func(e *Entry) { e.Live, e.State, e.Reason = "说了一半", Cancelled, "已取消" })
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal(err, info.Mode())
	}
	got, err := Load(path, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	r := got.Recent("", 0)
	if len(r) != 2 || r[0].ID != other || r[0].Text() != "说了一半" || r[1].Text() != "你好，世界。" {
		t.Fatal(r)
	}
	if r := got.Recent("世界", 0); len(r) != 1 || r[0].ID != id {
		t.Fatal("search", r)
	}
	if next := got.Begin(now, "badge", ""); next <= other {
		t.Fatal("ID reused", next)
	}
}

func TestRetentionAndQuitDuringSession(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	path := filepath.Join(t.TempDir(), "history.json")
	s := &Store{}
	old := s.Begin(now, "badge", "")
	s.Update(old, now, func(e *Entry) { e.Live = "旧的" })
	running := s.Begin(now.Add(6*24*time.Hour), "badge", "")
	s.Update(running, now.Add(6*24*time.Hour), func(e *Entry) { e.Live = "进行中" })
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, now.Add(7*24*time.Hour+time.Minute))
	if err != nil || len(got.Entries) != 1 {
		t.Fatal(err, got.Entries)
	}
	if e := got.Entries[0]; e.ID != running || e.State != Failed || e.Reason == "" {
		t.Fatal(e)
	}
}

func TestDeleteClearAndBounds(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	s := &Store{}
	for i := 0; i < MaxEntries+5; i++ {
		s.Begin(now, "badge", "")
	}
	if len(s.Entries) != MaxEntries || s.Entries[0].ID != 6 {
		t.Fatal(len(s.Entries), s.Entries[0].ID)
	}
	if !s.Delete(6) || s.Delete(6) {
		t.Fatal("delete")
	}
	s.Clear(s.next)
	if len(s.Entries) != 1 || s.Entries[0].ID != s.next {
		t.Fatal("clear keeps the running entry", s.Entries)
	}
	s.Update(s.next, now, func(e *Entry) { e.Original = strings.Repeat("字", MaxText+10) })
	if e, _ := s.Get(s.next); len([]rune(e.Original)) != MaxText {
		t.Fatal(len([]rune(e.Original)))
	}
}

func TestLoadRefusesOtherFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	_ = os.WriteFile(path, []byte(`{"entries":[]}`), 0o600)
	if _, err := Load(path, time.Now()); err == nil {
		t.Fatal("accepted a file without the format")
	}
}
