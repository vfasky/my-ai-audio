package vocabulary

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of the automatic vocabulary.
const (
	// MaxLearned bounds the new terms one dictation adds.
	MaxLearned = 3
	// LearnedWeight ranks a learned term below one the user typed.
	LearnedWeight = 30
	maxIgnored    = 1000
	maxLearnedLen = 32
	// proseWords: a run of more Latin words is an English phrase rather
	// than a name inside Chinese dictation.
	proseWords = 3
)

// common are Latin words and acronyms that dictation contains so often that
// learning them would only be noise.
var common = map[string]bool{
	"ok": true, "okay": true, "ai": true, "app": true, "ppt": true, "pdf": true, "id": true, "pc": true,
	"tv": true, "usb": true, "ceo": true, "vip": true, "it": true, "ui": true, "hi": true, "hello": true,
	"yes": true, "no": true, "bye": true, "wifi": true, "the": true, "and": true, "but": true, "so": true,
	"am": true, "pm": true, "mr": true, "ms": true, "dr": true, "thanks": true, "thank": true, "sorry": true,
}

// NewTerms finds the names in dictated text that the vocabulary lacks: Latin
// words with an inner capital, a digit or only capitals, such as iPhone,
// GPT-4o or GPT, and a word only capitalized at the start, such as Plaud,
// when it stands between Chinese words: in English text such a capital
// mostly starts a sentence (Good morning, See you). Lowercase words are
// ordinary words. Terms, replacement sources and targets, and words the user
// undid are never suggested again. It is cheap enough for the UI thread.
func (s *Store) NewTerms(text string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(text) > 8000 {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, run := range latinRuns(text) {
		// Only a short run that touches Chinese text is a name in Chinese
		// dictation; anything else is English.
		chinese := run.han && len(run.words) <= proseWords
		for _, word := range run.words {
			key := Normalize(word)
			if seen[key] || !nameLike(word, chinese) || common[key] || s.known(word) {
				continue
			}
			seen[key] = true
			out = append(out, word)
			if len(out) == limit {
				return out
			}
		}
	}
	return out
}

// latinRun is a run of Latin words that only spaces separate; han: the
// character next to it, past spaces, is Chinese on either side.
type latinRun struct {
	words []string
	han   bool
}

// latinRuns splits text into runs of Latin words; Chinese text and
// punctuation end a run.
func latinRuns(text string) []latinRun {
	var runs []latinRun
	var run latinRun
	var before rune // the last character before the run that is not a space
	flush := func(after rune) {
		if len(run.words) > 0 {
			run.han = run.han || unicode.Is(unicode.Han, after)
			runs = append(runs, run)
		}
		run = latinRun{}
	}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		switch {
		case asciiWord(r):
			end := i + size
			for end < len(text) {
				c := text[end]
				if asciiWord(rune(c)) {
					end++
					continue
				}
				// Inner joiners keep names such as Node.js, C-3PO or GPT-4o whole.
				if (c == '.' || c == '-' || c == '_') && end+1 < len(text) && asciiWord(rune(text[end+1])) {
					end += 2
					continue
				}
				break
			}
			if len(run.words) == 0 {
				run.han = unicode.Is(unicode.Han, before)
			}
			run.words = append(run.words, text[i:end])
			i = end
			continue
		case r == ' ' || r == '\t':
		default:
			flush(r)
			before = r
		}
		i += size
	}
	flush(0)
	return runs
}

// nameLike reports whether word looks like a name: an inner capital, a digit
// next to letters, an acronym, or, in Chinese dictation, a capital start.
func nameLike(word string, chinese bool) bool {
	if len(word) < 2 || len(word) > maxLearnedLen {
		return false
	}
	letters, upper, digits := 0, 0, 0
	inner := false
	for i, r := range word {
		switch {
		case unicode.IsUpper(r):
			letters++
			upper++
			inner = inner || i > 0
		case unicode.IsLetter(r):
			letters++
		case unicode.IsDigit(r):
			digits++
		}
	}
	switch {
	case letters == 0:
		return false
	case digits > 0:
		return letters >= 2
	case inner:
		return true
	case upper == 0:
		return false
	}
	// Only the first letter is a capital: a name in Chinese dictation, but in
	// English it mostly just starts a sentence.
	return chinese
}

// known reports whether the vocabulary has word: as a term, a word of a term,
// a replacement, or a word the user undid.
func (s *Store) known(word string) bool {
	key := Normalize(word)
	for _, t := range s.Terms {
		if t.NormalizedTerm == key || containsWord(t.NormalizedTerm, key) {
			return true
		}
	}
	for _, r := range s.Replacements {
		if Normalize(r.From) == key || containsWord(Normalize(r.To), key) {
			return true
		}
	}
	return slices.Contains(s.Ignored, key)
}

// containsWord reports whether word occurs in text as a whole Latin word.
func containsWord(text, word string) bool {
	for from := 0; from < len(text); {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return false
		}
		at += from
		if wordEdge(text, at, at+len(word), word) {
			return true
		}
		from = at + 1
	}
	return false
}

// Accept keeps the new terms that refinement named which can be learned:
// they occur in text, the inserted dictation, as written, are not in the
// vocabulary, are not ignored, and are not common words. A Chinese term
// needs two characters; a Latin one must not cut a word of text.
func (s *Store) Accept(found []string, text string, limit int) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range found {
		t = strings.Join(strings.Fields(t), " ")
		key := Normalize(t)
		if len(out) == limit || seen[key] || !learnable(t) || common[key] || s.known(t) || !occurs(text, t) {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
}

// learnable bounds a term from a model: a short name with letters, two or
// more of them, and no sentence punctuation.
func learnable(t string) bool {
	if t == "" || utf8.RuneCountInString(t) > maxLearnedLen || !utf8.ValidString(t) {
		return false
	}
	letters := 0
	for _, r := range t {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsDigit(r), r == ' ', r == '.', r == '-', r == '_', r == '+', r == '#', r == '&', r == '·':
		default:
			return false
		}
	}
	return letters >= 2
}

// occurs reports whether term is in text, ignoring case, without cutting a
// Latin word.
func occurs(text, term string) bool {
	return containsWord(strings.ToLower(text), strings.ToLower(term))
}

// Learn adds words as automatic terms and returns those it added.
func (s *Store) Learn(words []string) []string {
	var added []string
	for _, w := range words {
		if s.known(w) {
			continue
		}
		if s.Put(Term{Term: w, Source: "auto", Weight: LearnedWeight}, true) == nil {
			added = append(added, strings.TrimSpace(w))
		}
	}
	return added
}

// Unlearn removes the automatic terms of words, unless the user has since
// edited or locked them, and keeps them from being learned again.
func (s *Store) Unlearn(words []string) {
	for _, w := range words {
		key := Normalize(w)
		if key == "" {
			continue
		}
		s.Terms = slices.DeleteFunc(s.Terms, func(t Term) bool {
			return t.NormalizedTerm == key && t.Source == "auto" && !t.Locked
		})
		s.ignore(key)
	}
}

func (s *Store) ignore(key string) {
	if key == "" || slices.Contains(s.Ignored, key) {
		return
	}
	s.Ignored = append(s.Ignored, key)
	if len(s.Ignored) > maxIgnored {
		s.Ignored = slices.Clone(s.Ignored[len(s.Ignored)-maxIgnored:])
	}
}
