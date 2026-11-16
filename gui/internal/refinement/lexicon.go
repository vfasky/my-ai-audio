package refinement

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
)

// MaxRunes is the longest dictation the model steps take: a longer one,
// such as a long recording, is inserted as recognized.
const MaxRunes = maxRetrievalRunes

const (
	maxRetrievalRunes = 8000
	maxRetrievedTerms = 256
	maxRetrievedSpans = 4096
)

// ErrVocabularyLimit declines the whole model step rather than silently
// removing competing terms or truncating the speaker's context.
var ErrVocabularyLimit = errors.New("vocabulary: text or candidates exceed retrieval limits")

type soundKey struct {
	first  string
	length int
}

type indexedTerm struct {
	text     string
	runes    []rune
	readings [][]string
	pinyin   string
}

// Lexicon is an immutable local index of a vocabulary snapshot. Indexing the
// first folded syllable and length avoids scanning every term at every position,
// without expanding the Cartesian product of polyphonic character readings.
// Construction and lookup belong on a worker, never on the UI thread.
type Lexicon struct {
	terms   []indexedTerm
	bySound map[soundKey][]int
	lengths []int
}

func NewLexicon(terms []string) *Lexicon {
	l := &Lexicon{bySound: map[soundKey][]int{}}
	seen := map[string]bool{}
	readings := map[rune][]string{}
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		t := indexedTerm{text: term, runes: []rune(term)}
		if len(t.runes) >= 2 && allHan(t.runes) {
			var spelled []string
			for _, r := range t.runes {
				p, ok := readings[r]
				if !ok {
					p = pinyin.SinglePinyin(r, pinyinArgs)
					readings[r] = p
				}
				t.readings = append(t.readings, p)
				if len(p) > 0 {
					spelled = append(spelled, p[0])
				}
			}
			t.pinyin = strings.Join(spelled, " ")
			for _, first := range foldedReadings(t.readings[0]) {
				key := soundKey{first, len(t.runes)}
				l.bySound[key] = append(l.bySound[key], len(l.terms))
			}
			if !slices.Contains(l.lengths, len(t.runes)) {
				l.lengths = append(l.lengths, len(t.runes))
			}
		}
		l.terms = append(l.terms, t)
	}
	slices.SortFunc(l.lengths, func(a, b int) int { return b - a })
	return l
}

func foldedReadings(readings []string) []string {
	var out []string
	for _, reading := range readings {
		folded := foldSound(reading)
		if !slices.Contains(out, folded) {
			out = append(out, folded)
		}
	}
	return out
}

// match returns all competitors and all literal terms, including longer terms
// that protect their interior from correction. No proper name is replaced here.
func (l *Lexicon) match(text string) ([]Span, map[string]bool, error) {
	if utf8.RuneCountInString(text) > maxRetrievalRunes {
		return nil, nil, ErrVocabularyLimit
	}
	runes := []rune(text)
	protected := make([]bool, len(runes))
	present := map[string]bool{}
	for _, term := range l.terms {
		// Literal matching uses byte offsets; convert only actual occurrences
		// to rune offsets. This also preserves overlapping known terms.
		for from := 0; from < len(text); {
			at := strings.Index(text[from:], term.text)
			if at < 0 {
				break
			}
			at += from
			present[term.text] = true
			if len(term.runes) >= 2 {
				start := utf8.RuneCountInString(text[:at])
				for j := start; j < start+len(term.runes); j++ {
					protected[j] = true
				}
			}
			_, size := utf8.DecodeRuneInString(text[at:])
			from = at + size
		}
	}
	readings := make([][]string, len(runes))
	cache := map[rune][]string{}
	for i, r := range runes {
		if !unicode.Is(unicode.Han, r) {
			continue
		}
		p, ok := cache[r]
		if !ok {
			p = pinyin.SinglePinyin(r, pinyinArgs)
			cache[r] = p
		}
		readings[i] = p
	}
	// Collect by term to retain the previous longest-term, vocabulary-order,
	// occurrence-order contract, independent of map iteration order.
	byTerm := make([][]Span, len(l.terms))
	count := 0
	for start := range runes {
		if protected[start] {
			continue
		}
		firsts := foldedReadings(readings[start])
		for _, length := range l.lengths {
			end := start + length
			if end > len(runes) || slices.Contains(protected[start:end], true) {
				continue
			}
			seen := map[int]bool{}
			for _, first := range firsts {
				for _, id := range l.bySound[soundKey{first, length}] {
					if seen[id] {
						continue
					}
					seen[id] = true
					term := l.terms[id]
					match := soundMatch(readings[start:end], term.readings)
					if match == "" {
						continue
					}
					count++
					if count > maxRetrievedSpans {
						return nil, nil, ErrVocabularyLimit
					}
					byTerm[id] = append(byTerm[id], Span{Candidate: Candidate{
						Heard: string(runes[start:end]), Term: term.text, Pinyin: term.pinyin, Match: match,
					}, Start: start, End: end})
				}
			}
		}
	}
	var spans []Span
	for _, matches := range byTerm {
		spans = append(spans, matches...)
	}
	slices.SortStableFunc(spans, func(a, b Span) int { return (b.End - b.Start) - (a.End - a.Start) })
	return spans, present, nil
}

// Retrieve queries all terms locally. Only matching Chinese candidates and
// literal terms reach the model. Non-Chinese recognition hints remain available
// for transliteration/case/spacing, which a Han-only pinyin matcher cannot recall.
// Overflow declines the whole operation; it never drops some homophone choices.
func (l *Lexicon) Retrieve(text string, recognitionHints []string) ([]string, error) {
	spans, selected, err := l.match(text)
	if err != nil {
		return nil, err
	}
	for _, s := range spans {
		selected[s.Term] = true
	}
	for _, hint := range recognitionHints {
		if !allHan([]rune(hint)) {
			selected[hint] = true
		}
	}
	// Case and spacing matches can retrieve non-Chinese terms outside Top 100.
	folded := strings.ToLower(strings.Join(strings.Fields(text), " "))
	var out []string
	for _, term := range l.terms {
		if selected[term.text] || (!allHan(term.runes) && strings.Contains(folded, strings.ToLower(strings.Join(strings.Fields(term.text), " ")))) {
			out = append(out, term.text)
			if len(out) > maxRetrievedTerms {
				return nil, ErrVocabularyLimit
			}
		}
	}
	return out, nil
}
