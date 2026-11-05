// Package hotkey names the modifier chords that start push-to-talk with the
// Mac microphone. SpeechBridge watches the chord; this package only keeps the
// choice valid. A chord is one of the presets the General page offers or a
// custom combination of at least two modifier keys.
package hotkey

import "strings"

// Off turns the hotkey off.
const Off = "off"

// Default is Shift+Option.
const Default = "shift+option"

// Mods is a set of modifier keys.
type Mods uint8

// The modifier keys a chord is made of, in the order IDs and labels list
// them.
const (
	Command Mods = 1 << iota
	Control
	Shift
	Option
)

// Keys are the modifiers in canonical order with their wire names and the
// symbols the interface shows.
var Keys = []struct {
	Mod          Mods
	Name, Symbol string
}{
	{Command, "command", "⌘"},
	{Control, "control", "⌃"},
	{Shift, "shift", "⇧"},
	{Option, "option", "⌥"},
}

// MinKeys is the fewest modifiers a chord has: a single one is held too
// often while typing.
const MinKeys = 2

// Choice is one chord the General page offers.
type Choice struct{ ID, Label string }

// Choices are the preset chords in the order the General page lists them.
// The IDs are the wire values SpeechBridge accepts.
var Choices = []Choice{
	{"shift+option", "⇧ + ⌥"},
	{"control+option", "⌃ + ⌥"},
	{"control+shift", "⌃ + ⇧"},
	{"command+option", "⌘ + ⌥"},
}

// Count is the number of modifiers in m.
func (m Mods) Count() int {
	n := 0
	for _, k := range Keys {
		if m&k.Mod != 0 {
			n++
		}
	}
	return n
}

// ID is the canonical wire value of m, or "" when it has fewer than MinKeys
// modifiers.
func ID(m Mods) string {
	if m.Count() < MinKeys {
		return ""
	}
	var names []string
	for _, k := range Keys {
		if m&k.Mod != 0 {
			names = append(names, k.Name)
		}
	}
	return strings.Join(names, "+")
}

// Parse reads a chord written as modifier names joined by "+", in any order.
func Parse(id string) (Mods, bool) {
	var m Mods
	for _, name := range strings.Split(id, "+") {
		var bit Mods
		for _, k := range Keys {
			if k.Name == name {
				bit = k.Mod
			}
		}
		if bit == 0 || m&bit != 0 {
			return 0, false
		}
		m |= bit
	}
	return m, m.Count() >= MinKeys
}

// Normalize maps an unset or unknown value to Default and writes a valid
// chord in canonical order.
func Normalize(id string) string {
	if id == Off {
		return Off
	}
	if m, ok := Parse(id); ok {
		return ID(m)
	}
	return Default
}

// Enabled reports whether id names a chord rather than Off.
func Enabled(id string) bool { return Normalize(id) != Off }

// IsPreset reports whether id is one of Choices.
func IsPreset(id string) bool {
	for _, c := range Choices {
		if c.ID == id {
			return true
		}
	}
	return false
}

// IsCustom reports whether id is a chord that is not a preset.
func IsCustom(id string) bool {
	id = Normalize(id)
	return id != Off && !IsPreset(id)
}

// Label is how the interface shows id.
func Label(id string) string {
	id = Normalize(id)
	if id == Off {
		return "关闭"
	}
	m, _ := Parse(id)
	var symbols []string
	for _, k := range Keys {
		if m&k.Mod != 0 {
			symbols = append(symbols, k.Symbol)
		}
	}
	return strings.Join(symbols, " + ")
}
