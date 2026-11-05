package vocabulary

import (
	"slices"
	"strings"
)

// Contextual returns at most 100 recognition hints. Explicit terms keep their
// lock/weight order; unique replacement targets fill remaining slots.
// Wrong spellings are never injected into recognition or treated as known terms.
func (s *Store) Contextual(n int) []string {
	n = max(0, min(n, 100))
	return s.appendTargets(s.Top(n), n)
}

// CorrectionTerms returns the complete, ranked vocabulary, including unique
// replacement targets. It is a local retrieval source, not a model prompt.
func (s *Store) CorrectionTerms() []string {
	return s.appendTargets(s.ranked(len(s.Terms)), len(s.Terms)+len(s.Replacements))
}

func (s *Store) appendTargets(out []string, n int) []string {
	seen := map[string]bool{}
	for _, term := range out {
		seen[Normalize(term)] = true
	}
	for _, rule := range s.Replacements {
		if len(out) >= n {
			break
		}
		term := strings.TrimSpace(rule.To)
		if key := Normalize(term); key != "" && !seen[key] {
			out = append(out, term)
			seen[key] = true
		}
	}
	return out
}

// Snapshot copies the data read by a background correction, so editing/importing
// the vocabulary on the UI thread cannot change an in-flight operation.
func (s *Store) Snapshot() Store {
	return Store{Terms: slices.Clone(s.Terms), Replacements: slices.Clone(s.Replacements), Ignored: slices.Clone(s.Ignored)}
}
