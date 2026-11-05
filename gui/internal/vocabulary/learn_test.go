package vocabulary

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestNewTermsFindsNames(t *testing.T) {
	var s Store
	_ = s.Put(Term{Term: "WorkTony", Source: "manual", Weight: 50}, false)
	_ = s.PutReplacement("", Replacement{From: "go", To: "Go Voice"})
	cases := map[string][]string{
		"可以链接 Plaud 吗？我想每周帮我清洗":         {"Plaud"},
		"用 GPT-4o 和 iPhone 测一下 Node.js": {"GPT-4o", "iPhone", "Node.js"},
		"打开 WorkTony 和 Go Voice":        nil,
		"这个 review 一下，OK 吗，用 AI 做 PPT":  nil,
		// In English text a capital start is mostly a sentence start: only
		// stronger evidence (inner capital, digit, acronym) makes a name.
		"I think Plaud is great":       nil,
		"Please send it to me":         nil,
		"Good morning":                 nil,
		"I think so. See you tomorrow": nil,
		"好的。See you tomorrow":          nil,
		"Ask Alice":                    nil,
		"I use GPT-4o daily":           {"GPT-4o"},
		"用 Plaud。":                     {"Plaud"},
		"Plaud 怎么样":                    {"Plaud"},
		"版本 2026 发布":                   nil,
		"用 Plaud 和 plaud 还有 PLAUD":     {"Plaud"},
	}
	for text, want := range cases {
		if got := s.NewTerms(text, MaxLearned); !slices.Equal(got, want) {
			t.Errorf("%q: %q, want %q", text, got, want)
		}
	}
	if got := s.NewTerms("AB1 CD2 EF3 GH4", 2); len(got) != 2 {
		t.Fatalf("limit %q", got)
	}
}

func TestLearnUndoAndIgnore(t *testing.T) {
	var s Store
	added := s.Learn([]string{"Plaud", "Notion"})
	if !slices.Equal(added, []string{"Plaud", "Notion"}) || s.Terms[0].Source != "auto" || s.Terms[0].Weight != LearnedWeight {
		t.Fatalf("added %q %+v", added, s.Terms)
	}
	if len(s.Learn([]string{"plaud"})) != 0 {
		t.Fatal("learned twice")
	}
	s.Unlearn([]string{"Plaud"})
	if len(s.Terms) != 1 || s.NewTerms("用 Plaud", 3) != nil {
		t.Fatalf("undo %+v %q", s.Terms, s.Ignored)
	}
	// A learned term the user deletes is not learned again either.
	_ = s.Delete("Notion", false)
	if s.NewTerms("用 Notion", 3) != nil {
		t.Fatalf("deleted relearned %q", s.Ignored)
	}
	// Undo keeps a learned term the user locked since.
	s.Learn([]string{"Figma"})
	_ = s.Update("Figma", Term{Term: "Figma", Source: "manual", Weight: 50, Locked: true})
	s.Unlearn([]string{"Figma"})
	if len(s.Terms) != 1 {
		t.Fatalf("locked removed %+v", s.Terms)
	}
	path := filepath.Join(t.TempDir(), "vocabulary.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !slices.Equal(loaded.Ignored, s.Ignored) {
		t.Fatalf("loaded %q %v", loaded.Ignored, err)
	}
}

func TestAcceptKeepsTermsOfTheText(t *testing.T) {
	var s Store
	_ = s.Put(Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	s.Ignored = []string{"notion"}
	text := "跟伊智的泽泽说，用 Plaud 和 Notion 记一下 OK"
	got := s.Accept([]string{"Plaud", "泽泽", "伊智", "Notion", "OK", "Figma", "Pla", "记一下，好吗", "泽"}, text, MaxLearned)
	if !slices.Equal(got, []string{"Plaud", "泽泽"}) {
		t.Fatalf("%q", got)
	}
}
