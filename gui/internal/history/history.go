// Package history keeps the dictations of the last days, so that a session
// that was cancelled, failed or inserted somewhere unseen can still be read
// and copied. It stores text only, never audio.
package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Retention is how long an entry is kept.
const Retention = 7 * 24 * time.Hour

// MaxEntries bounds the file however many dictations a week holds;
// MaxText bounds each text of an entry, past a 30-minute dictation.
const (
	MaxEntries = 1000
	MaxText    = 40000
	maxFile    = 64 << 20
)

// States of an entry.
const (
	Recording = "recording" // still running, or the app quit meanwhile
	Inserted  = "inserted"
	Cancelled = "cancelled"
	Failed    = "failed"
)

// Entry is one dictation.
type Entry struct {
	ID      uint64    `json:"id"`
	Started time.Time `json:"started"`
	Updated time.Time `json:"updated"`
	// Duration is the audio recorded, in milliseconds.
	Duration int64 `json:"duration_ms"`
	// Source is "badge" or "hotkey"; Engine names the recognition engine.
	Source string `json:"source"`
	Engine string `json:"engine,omitempty"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	// Live is the latest recognized text while the session runs, Original
	// the final recognition and Inserted the text that was inserted.
	Live     string `json:"live,omitempty"`
	Original string `json:"original,omitempty"`
	Inserted string `json:"inserted,omitempty"`
}

// Text is the most finished text of e.
func (e Entry) Text() string {
	switch {
	case e.Inserted != "":
		return e.Inserted
	case e.Original != "":
		return e.Original
	}
	return e.Live
}

// Empty: e holds no words.
func (e Entry) Empty() bool { return strings.TrimSpace(e.Text()) == "" }

// Store holds the entries, oldest first.
type Store struct {
	Entries []Entry `json:"entries"`
	next    uint64
}

type file struct {
	Format  string  `json:"format"`
	Entries []Entry `json:"entries"`
}

const format = "go-voice-history"

// Load reads path; a missing file is an empty store. Entries past the
// retention are dropped, and a running entry left by a quit becomes failed.
func Load(path string, now time.Time) (*Store, error) {
	s := &Store{}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if len(b) > maxFile {
		return s, errors.New("history: file too large")
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil || f.Format != format {
		return s, errors.New("history: invalid file")
	}
	for _, e := range f.Entries {
		if e.ID == 0 {
			continue
		}
		if e.State == Recording {
			e.State, e.Reason = Failed, "伊言在本次语音结束前退出"
		}
		s.Entries = append(s.Entries, clip(e))
		s.next = max(s.next, e.ID)
	}
	s.Prune(now)
	return s, nil
}

// Save replaces path atomically, readable by the user only.
func (s *Store) Save(path string) error {
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	return Write(path, b)
}

// Marshal encodes the store as its file.
func (s *Store) Marshal() ([]byte, error) {
	return json.Marshal(file{Format: format, Entries: s.Entries})
}

// Write replaces path with b atomically, readable by the user only.
func Write(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Begin adds a running entry and returns its ID.
func (s *Store) Begin(now time.Time, source, engine string) uint64 {
	s.next++
	s.Entries = append(s.Entries, Entry{ID: s.next, Started: now, Updated: now, Source: source, Engine: engine, State: Recording})
	if len(s.Entries) > MaxEntries {
		s.Entries = append([]Entry(nil), s.Entries[len(s.Entries)-MaxEntries:]...)
	}
	return s.next
}

// Update changes the entry id; it reports whether there is one.
func (s *Store) Update(id uint64, now time.Time, change func(*Entry)) bool {
	// The running entry is the newest: look from the end.
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if s.Entries[i].ID == id {
			change(&s.Entries[i])
			s.Entries[i] = clip(s.Entries[i])
			s.Entries[i].Updated = now
			return true
		}
	}
	return false
}

// Get returns the entry id.
func (s *Store) Get(id uint64) (Entry, bool) {
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if s.Entries[i].ID == id {
			return s.Entries[i], true
		}
	}
	return Entry{}, false
}

// Delete removes the entry id.
func (s *Store) Delete(id uint64) bool {
	for i, e := range s.Entries {
		if e.ID == id {
			s.Entries = append(s.Entries[:i:i], s.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// Clear removes every entry but the running one, keep, when it is not zero.
func (s *Store) Clear(keep uint64) {
	var kept []Entry
	for _, e := range s.Entries {
		if keep != 0 && e.ID == keep {
			kept = append(kept, e)
		}
	}
	s.Entries = kept
}

// Prune drops entries older than the retention; it reports whether it
// dropped any.
func (s *Store) Prune(now time.Time) bool {
	cut := now.Add(-Retention)
	kept := s.Entries[:0]
	for _, e := range s.Entries {
		if e.Updated.After(cut) {
			kept = append(kept, e)
		}
	}
	dropped := len(kept) != len(s.Entries)
	clear(s.Entries[len(kept):])
	s.Entries = kept
	return dropped
}

// Recent returns the entries with words, newest first, matching query
// when it is not empty, at most limit of them when limit is positive.
func (s *Store) Recent(query string, limit int) []Entry {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []Entry
	for i := len(s.Entries) - 1; i >= 0; i-- {
		e := s.Entries[i]
		if e.Empty() {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Text()+"\n"+e.Original+"\n"+e.Live), query) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

// clip bounds the texts of e.
func clip(e Entry) Entry {
	e.Live, e.Original, e.Inserted = clipText(e.Live), clipText(e.Original), clipText(e.Inserted)
	if utf8.RuneCountInString(e.Reason) > 200 {
		e.Reason = string([]rune(e.Reason)[:200])
	}
	return e
}

func clipText(s string) string {
	if len(s) <= MaxText || utf8.RuneCountInString(s) <= MaxText {
		return s
	}
	return string([]rune(s)[:MaxText])
}
