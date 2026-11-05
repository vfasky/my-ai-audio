package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/device"
	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/vocabulary"
)

func TestSessionVocabularyIncludesReplacementTargets(t *testing.T) {
	a := &application{}
	_ = a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 50}, false)
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.effects([]device.Effect{{Type: "start", ID: 1}})
	if !slices.Equal(a.terms, []string{"伊智", "私域宝"}) {
		t.Fatal(a.terms)
	}
	_ = a.store.PutReplacement("施玉宝", vocabulary.Replacement{From: "施玉宝", To: "私域助手"})
	if !slices.Equal(a.terms, []string{"伊智", "私域宝"}) {
		t.Fatal("active session vocabulary changed", a.terms)
	}
	a.effects([]device.Effect{{Type: "start", ID: 2}})
	if !slices.Equal(a.terms, []string{"伊智", "私域助手"}) {
		t.Fatal("new session missed the target edit", a.terms)
	}
}

func TestSessionRetrievesBeyondRecognitionLimitAndFreezesVocabulary(t *testing.T) {
	a := &application{}
	for i := 0; i < 120; i++ {
		_ = a.store.Put(vocabulary.Term{Term: fmt.Sprintf("测试条目%d", i), Source: "manual", Weight: 100}, false)
	}
	_ = a.store.Put(vocabulary.Term{Term: "伊智", Source: "manual", Weight: 1}, false)
	_ = a.store.Put(vocabulary.Term{Term: "一致性", Source: "manual", Weight: 1}, false)
	_ = a.store.PutReplacement("", vocabulary.Replacement{From: "施玉宝", To: "私域宝"})
	a.effects([]device.Effect{{Type: "start", ID: 1}})
	if len(a.terms) != 100 || slices.Contains(a.terms, "伊智") || slices.Contains(a.terms, "私域宝") {
		t.Fatal("recognition budget changed", a.terms)
	}
	text := "保持一致性，联系一致科技，再打开私欲宝"
	got, err := refinement.NewLexicon(a.correctionTerms).Retrieve(text, a.terms)
	for _, term := range []string{"伊智", "一致性", "私域宝"} {
		if err != nil || !slices.Contains(got, term) {
			t.Fatalf("missing %s: %v %v", term, got, err)
		}
	}
	_ = a.store.Delete("伊智", false)
	_ = a.store.PutReplacement("施玉宝", vocabulary.Replacement{From: "施玉宝", To: "私域助手"})
	if !slices.Contains(a.correctionTerms, "伊智") || !slices.Contains(a.correctionTerms, "私域宝") {
		t.Fatal("editing changed an active session")
	}
	a.effects([]device.Effect{{Type: "start", ID: 2}})
	if slices.Contains(a.correctionTerms, "伊智") || slices.Contains(a.correctionTerms, "私域宝") || !slices.Contains(a.correctionTerms, "私域助手") {
		t.Fatal("new session retained old vocabulary", a.correctionTerms)
	}
}

func TestInsertionLearnsNewWordsAndUndo(t *testing.T) {
	a := &application{directory: t.TempDir()}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "inserting"
	a.coordinator.Refined = "可以链接 Plaud 吗？"
	a.onSpeech(bridge.Event{Type: "inserted", SessionID: 7, Text: "inserted"})
	if a.coordinator.Active || !slices.Equal(a.view.Snapshot.Learned, []string{"Plaud"}) || len(a.store.Terms) != 1 || a.store.Terms[0].Source != "auto" {
		t.Fatalf("learned %q terms %+v", a.view.Snapshot.Learned, a.store.Terms)
	}
	saved, err := vocabulary.Load(filepath.Join(a.directory, "vocabulary.json"))
	if err != nil || len(saved.Terms) != 1 {
		t.Fatalf("saved %+v %v", saved.Terms, err)
	}
	a.view.Actions.UndoLearned = a.undoLearned
	a.undoLearned()
	if len(a.store.Terms) != 0 || a.view.Snapshot.Learned != nil {
		t.Fatalf("undo %+v %q", a.store.Terms, a.view.Snapshot.Learned)
	}
	// Turned off, or a cancelled session, learns nothing.
	a.config.LearnOff = true
	a.store.Ignored = nil
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 8, "inserting"
	a.coordinator.Refined = "用 Notion 记"
	a.onSpeech(bridge.Event{Type: "inserted", SessionID: 8})
	if len(a.store.Terms) != 0 {
		t.Fatalf("learned while off %+v", a.store.Terms)
	}
}

func TestRefinedInsertionLearnsWhatTheModelNamed(t *testing.T) {
	a := &application{directory: t.TempDir()}
	a.coordinator.Active, a.coordinator.Generation, a.coordinator.Phase = true, 7, "inserting"
	a.coordinator.Original, a.coordinator.Refined = "跟泽泽说用 Plaud", "跟泽泽说，用 Plaud。"
	a.aiTerms, a.aiTermsID, a.aiTermsOK = []string{"泽泽", "Monday"}, 7, true
	a.onSpeech(bridge.Event{Type: "inserted", SessionID: 7})
	// The model's answer replaces the local rule: Plaud is not learned, and
	// Monday, which the text lacks, is refused.
	if !slices.Equal(a.view.Snapshot.Learned, []string{"泽泽"}) || len(a.store.Terms) != 1 {
		t.Fatalf("learned %q terms %+v", a.view.Snapshot.Learned, a.store.Terms)
	}
	if a.aiTermsOK || a.aiTerms != nil {
		t.Fatal("model terms not cleared")
	}
}
