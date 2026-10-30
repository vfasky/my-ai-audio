package vocabulary

import (
	_ "embed"
)

//go:embed defaults.json
var defaultsJSON []byte

// Defaults returns the vocabulary that ships with the app. The app starts
// from it when the user has no vocabulary file yet.
func Defaults() Store {
	var s Store
	if _, err := s.Import(defaultsJSON); err != nil {
		return Store{}
	}
	for i := range s.Terms {
		s.Terms[i].Source = "manual"
	}
	return s
}
