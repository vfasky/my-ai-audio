package hotkey

import "testing"

func TestNormalizeDefaultsAndKeepsKnownChords(t *testing.T) {
	for in, want := range map[string]string{"": Default, "cmd+x": Default, "control+option": "control+option", Off: Off} {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if Enabled(Off) || !Enabled("") || Label("") != "⇧ + ⌥" {
		t.Fatal("default chord")
	}
}
