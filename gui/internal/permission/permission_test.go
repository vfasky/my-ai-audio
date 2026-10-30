package permission

import (
	"strings"
	"testing"
)

func TestParseRejectsUnknownNamesAndStates(t *testing.T) {
	for _, c := range [][2]string{{"camera", "granted"}, {"speech", "maybe"}, {"", ""}, {"speech", ""}} {
		if _, _, ok := Parse(c[0], c[1]); ok {
			t.Fatalf("accepted %q", c)
		}
	}
	if n, s, ok := Parse("automation", "unknown"); !ok || n != Automation || s != Unknown {
		t.Fatal("unknown automation state")
	}
}

func TestMissingIsOrderedAndSkipsOptionalAutomation(t *testing.T) {
	s := Set{Bluetooth: Granted, Speech: Denied}
	got := s.Missing()
	if len(got) != 2 || got[0] != Speech || got[1] != Accessibility {
		t.Fatalf("missing %v", got)
	}
	s[Speech], s[Accessibility] = Granted, Granted
	if len(s.Missing()) != 0 {
		t.Fatal("automation and microphone must stay optional")
	}
	if Required(Microphone) {
		t.Fatal("microphone is only for the hotkey")
	}
}

func TestPromptsOnlyBeforeAnAnswer(t *testing.T) {
	if !Prompts(Undetermined) || !Prompts(Unknown) || Prompts(Denied) || Prompts(Restricted) || Prompts(Granted) {
		t.Fatal("prompt policy")
	}
	for _, n := range Names {
		if !strings.HasPrefix(SettingsURL(n), "x-apple.systempreferences:com.apple.preference.security?Privacy_") || Title(n) == "" || Purpose(n) == "" {
			t.Fatalf("metadata for %s", n)
		}
	}
}
