package hotkey

import "testing"

func TestNormalizeDefaultsAndKeepsKnownChords(t *testing.T) {
	for in, want := range map[string]string{
		"": Default, "cmd+x": Default, "control+option": "control+option", Off: Off,
		"shift": Default, "shift+shift": Default, "shift+": Default,
		"option+control+shift":         "control+shift+option",
		"option+shift":                 "shift+option",
		"shift+option+command+control": "command+control+shift+option",
	} {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
	if Enabled(Off) || !Enabled("") || Label("") != "⇧ + ⌥" {
		t.Fatal("default chord")
	}
}

func TestPresetsAreCanonicalAndLabelled(t *testing.T) {
	for _, c := range Choices {
		if Normalize(c.ID) != c.ID || !IsPreset(c.ID) || IsCustom(c.ID) || Label(c.ID) != c.Label {
			t.Fatalf("preset %+v", c)
		}
	}
}

func TestCustomChords(t *testing.T) {
	if !IsCustom("control+shift+option") || IsCustom(Off) {
		t.Fatal("custom")
	}
	if got := Label("option+control+command"); got != "⌘ + ⌃ + ⌥" {
		t.Fatalf("label %q", got)
	}
	if ID(Control) != "" || ID(Control|Command) != "command+control" {
		t.Fatal("ID needs two modifiers")
	}
	if m, ok := Parse("shift+command"); !ok || m != Shift|Command {
		t.Fatal("parse")
	}
}
