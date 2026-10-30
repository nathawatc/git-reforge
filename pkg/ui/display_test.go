package ui

import (
	"testing"

	"git-reforge/pkg/config"
)

type dummyLogger struct{}

func (dummyLogger) Header(string)  {}
func (dummyLogger) Info(string)    {}
func (dummyLogger) Success(string) {}
func (dummyLogger) Warning(string) {}
func (dummyLogger) Error(string)   {}

func TestDisplayConfigSkipsPromptWhenYesIsTrue(t *testing.T) {
	cfg := config.New()
	cfg.SourceURL = "https://github.com/user/repo.git"
	cfg.DestURL = "https://codeberg.org/user/repo.git"
	cfg.OldNames = []string{"Old Author"}
	cfg.NewName = "New Author"
	cfg.NewEmail = "new@example.org"
	cfg.Yes = true
	cfg.Logger = dummyLogger{}

	// Must return immediately without calling os.Exit or blocking on prompt
	DisplayConfig(cfg)
}
