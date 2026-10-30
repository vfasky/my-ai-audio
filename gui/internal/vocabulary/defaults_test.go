package vocabulary

import "testing"

func TestDefaultsLoad(t *testing.T) {
	s := Defaults()
	if len(s.Terms) != 16 || len(s.Replacements) != 15 {
		t.Fatalf("terms %d replacements %d", len(s.Terms), len(s.Replacements))
	}
	if got := s.Correct("石玉宝"); got != "私域宝" {
		t.Fatalf("got %q", got)
	}
}
