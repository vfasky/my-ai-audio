package vocabulary

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Term struct {
	Term           string    `json:"term"`
	NormalizedTerm string    `json:"normalizedTerm"`
	Source         string    `json:"source"`
	Weight         int       `json:"weight"`
	Locked         bool      `json:"locked"`
	CreatedAt      time.Time `json:"createdAt"`
	LastUsedAt     time.Time `json:"lastUsedAt"`
	// Aliases are the replacements of version 0.1 files, read once and
	// moved into Store.Replacements.
	Aliases []string `json:"aliases,omitempty"`
}

type Store struct {
	Terms []Term `json:"terms"`
	// Replacements correct misrecognitions before anything else.
	Replacements []Replacement `json:"replacements,omitempty"`
	// Ignored are the normalized words the user undid or deleted after they
	// were learned automatically; they are not learned again.
	Ignored []string `json:"ignored,omitempty"`
}

func Normalize(s string) string            { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func (s *Store) Put(t Term, ai bool) error { return s.put(t, ai) }

// Update saves t in place of the term previous, which an edit may rename.
func (s *Store) Update(previous string, t Term) error {
	if e := s.put(t, false); e != nil {
		return e
	}
	if previous != "" && Normalize(previous) != Normalize(t.Term) {
		return s.Delete(previous, false)
	}
	return nil
}

func (s *Store) put(t Term, ai bool) error {
	t.Term = strings.TrimSpace(t.Term)
	t.NormalizedTerm = Normalize(t.Term)
	if t.NormalizedTerm == "" || len(t.Term) > 256 || !utf8.ValidString(t.Term) || t.Weight < 0 || t.Weight > 100 {
		return errors.New("invalid vocabulary term")
	}
	t.Aliases = nil
	switch t.Source {
	case "manual", "correction", "ai", "imported", "auto":
	default:
		return errors.New("invalid source")
	}
	for i, v := range s.Terms {
		if v.NormalizedTerm == t.NormalizedTerm {
			if ai && v.Locked {
				return errors.New("term is locked")
			}
			t.CreatedAt = v.CreatedAt
			t.LastUsedAt = v.LastUsedAt
			s.Terms[i] = t
			return nil
		}
	}
	if len(s.Terms) >= 2000 {
		return errors.New("vocabulary full")
	}
	t.CreatedAt = time.Now()
	s.Terms = append(s.Terms, t)
	return nil
}

func (s *Store) Delete(term string, ai bool) error {
	for i, v := range s.Terms {
		if v.NormalizedTerm == Normalize(term) {
			if ai && v.Locked {
				return errors.New("term is locked")
			}
			if v.Source == "auto" {
				// A learned word the user deletes stays unlearned.
				s.ignore(v.NormalizedTerm)
			}
			s.Terms = append(s.Terms[:i], s.Terms[i+1:]...)
			return nil
		}
	}
	return nil
}
func (s *Store) Top(n int) []string {
	return s.ranked(min(n, 100))
}

// ranked is shared by the bounded recognition hints and the full correction list.
func (s *Store) ranked(n int) []string {
	n = max(0, min(n, len(s.Terms)))
	if n == 0 {
		return nil
	}
	idx := make([]int, len(s.Terms))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := &s.Terms[idx[i]], &s.Terms[idx[j]]
		if a.Locked != b.Locked {
			return a.Locked
		}
		return a.Weight > b.Weight
	})
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = s.Terms[idx[i]].Term
	}
	return out
}
func Load(path string) (Store, error) {
	var s Store
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	if len(b) > 2*1024*1024 {
		return s, errors.New("vocabulary too large")
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return Store{}, e
	}
	validated := Store{}
	var migrated []Replacement
	for _, t := range s.Terms {
		if e = validated.Put(t, false); e != nil {
			return Store{}, e
		}
		for _, alias := range t.Aliases {
			migrated = append(migrated, Replacement{From: alias, To: strings.TrimSpace(t.Term)})
		}
		for i := range validated.Terms {
			if validated.Terms[i].NormalizedTerm == Normalize(t.Term) {
				if !t.CreatedAt.IsZero() {
					validated.Terms[i].CreatedAt = t.CreatedAt
				}
				validated.Terms[i].LastUsedAt = t.LastUsedAt
			}
		}
	}
	for _, r := range s.Replacements {
		if e = validated.PutReplacement("", r); e != nil {
			return Store{}, e
		}
	}
	// The aliases of version 0.1 move into the replacements; one that no
	// longer fits is dropped rather than failing the whole vocabulary.
	for _, r := range migrated {
		_ = validated.PutReplacement("", r)
	}
	for _, w := range s.Ignored {
		validated.ignore(Normalize(w))
	}
	return validated, nil
}
func (s *Store) Save(path string) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(filepath.Dir(path), ".vocabulary-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e != nil {
		tmp.Close()
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
