package main

import (
	_ "embed"

	"github.com/vfasky/my-ai-audio/gui/internal/app"
)

// The packaged app icon, used for the updater dialog and app bundle.
//
//go:embed resources/icon.png
var icon []byte

// The transparent sidebar logo, derived from assets/brand/go-voice-logo.png.
//
//go:embed resources/logo.png
var logo []byte

// The menu-bar template icon, from gui/scripts/generate-tray-icon.mjs.
//
//go:embed resources/tray.png
var tray []byte

func main() { app.Run(icon, logo, tray) }
