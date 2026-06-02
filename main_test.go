package main

import (
	"os"
	"path/filepath"
	"testing"

	"github-to-codeberg/pkg/config"
)

func validConfig() *config.Config {
	cfg := config.New()
	cfg.SourceURL = "https://github.com/example/repo.git"
	cfg.DestURL = "https://codeberg.org/example/repo.git"
	cfg.NewEmail = "new@example.org"
	cfg.OldNames = []string{"Old Name"}
	return cfg
}

func TestValidateArgsDefaultsSignMode(t *testing.T) {
	cfg := validConfig()
	cfg.SignMode = ""

	if err := validateArgs(cfg); err != nil {
		t.Fatalf("validateArgs() error = %v", err)
	}
	if cfg.SignMode != "keep" {
		t.Fatalf("SignMode = %q want keep", cfg.SignMode)
	}
}

func TestValidateArgsSaveDirWithoutDestImpliesNoPush(t *testing.T) {
	cfg := validConfig()
	cfg.DestURL = ""
	cfg.SaveDir = filepath.Join(t.TempDir(), "saved-repo")

	if err := validateArgs(cfg); err != nil {
		t.Fatalf("validateArgs() error = %v", err)
	}
	if !cfg.NoPush {
		t.Fatal("NoPush = false want true")
	}
}

func TestValidateArgsRejectsInvalidModes(t *testing.T) {
	tests := []struct {
		name string
		edit func(*config.Config)
	}{
		{
			name: "sign mode",
			edit: func(cfg *config.Config) {
				cfg.SignMode = "bad"
			},
		},
		{
			name: "coauthor mode",
			edit: func(cfg *config.Config) {
				cfg.CoauthorMode = "bad"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(cfg)
			if err := validateArgs(cfg); err == nil {
				t.Fatal("validateArgs() error = nil want error")
			}
		})
	}
}

func TestCollectOldNamesSupportsRepeatedAndCommaSeparatedValues(t *testing.T) {
	got := collectOldNames([]string{
		"Old One, Old Two",
		"Old Three",
		"  Old Four  ,",
	})
	want := []string{"Old One", "Old Two", "Old Three", "Old Four"}

	if len(got) != len(want) {
		t.Fatalf("collectOldNames() = %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collectOldNames()[%d] = %q want %q", i, got[i], want[i])
		}
	}
}

func TestValidateArgsAppliesCoauthorDefaults(t *testing.T) {
	cfg := validConfig()
	cfg.NewName = "New Name"
	cfg.CoauthorMode = "rewrite"

	if err := validateArgs(cfg); err != nil {
		t.Fatalf("validateArgs() error = %v", err)
	}
	if cfg.CoauthorOldName != "Old Name" {
		t.Fatalf("CoauthorOldName = %q want Old Name", cfg.CoauthorOldName)
	}
	if cfg.CoauthorNewName != "New Name" {
		t.Fatalf("CoauthorNewName = %q want New Name", cfg.CoauthorNewName)
	}
	if cfg.CoauthorNewEmail != "new@example.org" {
		t.Fatalf("CoauthorNewEmail = %q want new@example.org", cfg.CoauthorNewEmail)
	}
}

func TestCleanupTempDirRemovesExistingTempDir(t *testing.T) {
	cfg := config.New()
	cfg.TempDir = t.TempDir()

	cleanupTempDir(cfg)

	if _, err := os.Stat(cfg.TempDir); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) error = %v want not exist", cfg.TempDir, err)
	}
}
