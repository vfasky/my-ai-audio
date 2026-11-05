package vocabulary

import (
	"encoding/json"
	"errors"
	"slices"
)

// ExportFormat names the JSON files that Export writes and Import reads.
const ExportFormat = "go-voice-vocabulary"

// maxImport bounds the size of a file Import reads.
const maxImport = 2 * 1024 * 1024

// ErrImportFormat is a file that is no vocabulary export.
var ErrImportFormat = errors.New("not a vocabulary export")

// exported is the file of Export: the terms and replacements without the
// usage history of this Mac. Import also reads the vocabulary.json of the
// app, which uses the same keys.
type exported struct {
	Format       string            `json:"format,omitempty"`
	Version      int               `json:"version,omitempty"`
	Terms        []exportedTerm    `json:"terms"`
	Replacements []exportedRewrite `json:"replacements"`
}

type exportedTerm struct {
	Term   string `json:"term"`
	Locked bool   `json:"locked,omitempty"`
	Weight *int   `json:"weight,omitempty"`
}

type exportedRewrite struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Export writes the terms and replacements as indented JSON.
func (s *Store) Export() ([]byte, error) {
	e := exported{Format: ExportFormat, Version: 1, Terms: []exportedTerm{}, Replacements: []exportedRewrite{}}
	for _, t := range s.Terms {
		w := t.Weight
		e.Terms = append(e.Terms, exportedTerm{Term: t.Term, Locked: t.Locked, Weight: &w})
	}
	for _, r := range s.Replacements {
		e.Replacements = append(e.Replacements, exportedRewrite{From: r.From, To: r.To})
	}
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ImportResult counts what Import did.
type ImportResult struct {
	TermsAdded, TermsUpdated               int
	ReplacementsAdded, ReplacementsUpdated int
	// Skipped are the entries that were invalid or conflict with the
	// vocabulary, such as a replacement of a term.
	Skipped int
}

// Import merges a file of Export into the vocabulary: new terms and
// replacements are added, those already there take the file's lock,
// weight and target, and nothing is removed. Invalid entries are skipped.
// A file that cannot be read changes nothing.
func (s *Store) Import(b []byte) (ImportResult, error) {
	var res ImportResult
	if len(b) > maxImport {
		return res, ErrImportFormat
	}
	var e exported
	if err := json.Unmarshal(b, &e); err != nil || (e.Format != "" && e.Format != ExportFormat) || (e.Terms == nil && e.Replacements == nil) {
		return res, ErrImportFormat
	}
	next := s.Snapshot()
	for _, t := range e.Terms {
		term := Term{Term: t.Term, Source: "imported", Weight: 50, Locked: t.Locked}
		if t.Weight != nil {
			term.Weight = *t.Weight
		}
		existed := false
		for _, v := range next.Terms {
			if v.NormalizedTerm == Normalize(t.Term) {
				existed = true
				term.Source = v.Source
			}
		}
		if err := next.Put(term, false); err != nil {
			res.Skipped++
		} else if existed {
			res.TermsUpdated++
		} else {
			res.TermsAdded++
		}
	}
	for _, r := range e.Replacements {
		existed := slices.ContainsFunc(next.Replacements, func(v Replacement) bool { return Normalize(v.From) == Normalize(r.From) })
		previous := ""
		if existed {
			previous = r.From
		}
		if err := next.PutReplacement(previous, Replacement{From: r.From, To: r.To}); err != nil {
			res.Skipped++
		} else if existed {
			res.ReplacementsUpdated++
		} else {
			res.ReplacementsAdded++
		}
	}
	*s = next
	return res, nil
}
