// Package hotkey names the modifier chords that start push-to-talk with the
// Mac microphone. SpeechBridge watches the chord; this package only keeps the
// choice valid.
package hotkey

// Off turns the hotkey off.
const Off = "off"

// Default is Shift+Option.
const Default = "shift+option"

// Choice is one chord the General page offers.
type Choice struct{ ID, Label string }

// Choices are the chords in the order the General page lists them. The IDs
// are the wire values SpeechBridge accepts.
var Choices = []Choice{
	{"shift+option", "⇧ + ⌥"},
	{"control+option", "⌃ + ⌥"},
	{"control+shift", "⌃ + ⇧"},
	{"command+option", "⌘ + ⌥"},
	{Off, "关闭"},
}

// Normalize maps an unset or unknown value to Default.
func Normalize(id string) string {
	for _, c := range Choices {
		if c.ID == id {
			return id
		}
	}
	return Default
}

// Enabled reports whether id names a chord rather than Off.
func Enabled(id string) bool { return Normalize(id) != Off }

// Label is how the interface shows id.
func Label(id string) string {
	id = Normalize(id)
	for _, c := range Choices {
		if c.ID == id {
			return c.Label
		}
	}
	return ""
}
