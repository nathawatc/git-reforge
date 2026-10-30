package logging

import (
	"testing"

	"git-reforge/pkg/ui"
)

func TestLoggerRespectsDisableColors(t *testing.T) {
	ui.DisableColors()
	if ui.Red != "" || ui.Green != "" || ui.Blue != "" {
		t.Fatal("DisableColors did not clear ANSI color constants")
	}

	log := New()
	log.Info("info message")
	log.Success("success message")
	log.Warning("warning message")
	log.Error("error message")
	log.Header("header message")
}
