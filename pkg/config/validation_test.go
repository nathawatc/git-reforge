package config

import (
	"path/filepath"
	"testing"
)

func validConfig() *Config {
	cfg := New()
	cfg.SourceURL = "https://github.com/example/repo.git"
	cfg.DestURL = "https://codeberg.org/example/repo.git"
	cfg.NewEmail = "new@example.org"
	cfg.OldNames = []string{"Old Name"}
	return cfg
}

func TestValidateDefaultsSignMode(t *testing.T) {
	cfg := validConfig()
	cfg.SignMode = ""

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.SignMode != "keep" {
		t.Fatalf("SignMode = %q want keep", cfg.SignMode)
	}
}

func TestValidateSaveDirWithoutDestImpliesNoPush(t *testing.T) {
	cfg := validConfig()
	cfg.DestURL = ""
	cfg.SaveDir = filepath.Join(t.TempDir(), "saved-repo")

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !cfg.NoPush {
		t.Fatal("NoPush = false want true")
	}
}

func TestValidateRejectsInvalidModes(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{
			name: "sign mode",
			edit: func(cfg *Config) {
				cfg.SignMode = "bad"
			},
		},
		{
			name: "coauthor mode",
			edit: func(cfg *Config) {
				cfg.CoauthorMode = "bad"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(cfg)
			if err := Validate(cfg); err == nil {
				t.Fatal("Validate() error = nil want error")
			}
		})
	}
}

func TestParseOldNamesSupportsRepeatedAndCommaSeparatedValues(t *testing.T) {
	got := ParseOldNames([]string{
		"Old One, Old Two",
		"Old Three",
		"  Old Four  ,",
	})
	want := []string{"Old One", "Old Two", "Old Three", "Old Four"}

	if len(got) != len(want) {
		t.Fatalf("ParseOldNames() = %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseOldNames()[%d] = %q want %q", i, got[i], want[i])
		}
	}
}

func TestValidateAppliesCoauthorDefaults(t *testing.T) {
	cfg := validConfig()
	cfg.NewName = "New Name"
	cfg.CoauthorMode = "rewrite"

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
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

func TestValidateRejectsCoauthorRewriteWithoutNewName(t *testing.T) {
	cfg := validConfig()
	cfg.NewName = ""
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorNewName = ""

	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil want error when coauthor rewrite has no new name")
	}
}
