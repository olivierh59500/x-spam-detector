package gui

import (
	"x-spam-detector/internal/app"
)

// Run starts the GUI application
func Run(spamApp *app.SpamDetectorApp) {
	window := NewMainWindow(spamApp)
	window.Show()
}