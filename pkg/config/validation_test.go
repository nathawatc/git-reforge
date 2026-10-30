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

func TestValidateInfersSignModeFromGPGKey(t *testing.T) {
	cfg := validConfig()
	cfg.SignMode = ""
	cfg.GPGKey = "ABCD1234"

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.SignMode != "gpg" {
		t.Fatalf("SignMode = %q want gpg", cfg.SignMode)
	}
}

func TestValidateInfersSignModeFromSSHKey(t *testing.T) {
	cfg := validConfig()
	cfg.SignMode = ""
	cfg.SSHKey = "/path/to/key"

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.SignMode != "ssh" {
		t.Fatalf("SignMode = %q want ssh", cfg.SignMode)
	}
}

func TestValidateRejectsConflictingSigningOptions(t *testing.T) {
	t.Run("both gpg and ssh", func(t *testing.T) {
		cfg := validConfig()
		cfg.GPGKey = "ABCD"
		cfg.SSHKey = "/path"
		if err := Validate(cfg); err == nil {
			t.Fatal("Validate() error = nil want error for both gpg and ssh")
		}
	})

	t.Run("no-sign with gpg", func(t *testing.T) {
		cfg := validConfig()
		cfg.SignMode = "none"
		cfg.GPGKey = "ABCD"
		if err := Validate(cfg); err == nil {
			t.Fatal("Validate() error = nil want error for no-sign with gpg")
		}
	})

	t.Run("no-sign with ssh", func(t *testing.T) {
		cfg := validConfig()
		cfg.SignMode = "none"
		cfg.SSHKey = "/path"
		if err := Validate(cfg); err == nil {
			t.Fatal("Validate() error = nil want error for no-sign with ssh")
		}
	})
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
	if len(cfg.CoauthorFrom) != 1 || cfg.CoauthorFrom[0] != "Old Name" {
		t.Fatalf("CoauthorFrom = %v want [Old Name]", cfg.CoauthorFrom)
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

func TestValidateUsesExplicitCoauthorFromAndTo(t *testing.T) {
	cfg := validConfig()
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorFrom = []string{"Charlie", "Dave <dave@example.org>"}
	cfg.CoauthorNewName = "Eve"
	cfg.CoauthorNewEmail = "eve@example.org"

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(cfg.CoauthorFrom) != 2 || cfg.CoauthorFrom[0] != "Charlie" || cfg.CoauthorFrom[1] != "Dave <dave@example.org>" {
		t.Fatalf("CoauthorFrom = %v want [Charlie, Dave <dave@example.org>]", cfg.CoauthorFrom)
	}
	if cfg.CoauthorNewName != "Eve" {
		t.Fatalf("CoauthorNewName = %q want Eve", cfg.CoauthorNewName)
	}
	if cfg.CoauthorNewEmail != "eve@example.org" {
		t.Fatalf("CoauthorNewEmail = %q want eve@example.org", cfg.CoauthorNewEmail)
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

func TestValidateRequiresSource(t *testing.T) {
	cfg := validConfig()
	cfg.SourceURL = ""

	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil want error when SourceURL is empty")
	}
}

func TestValidateRequiresDestUnlessNoPushOrSaveDir(t *testing.T) {
	cfg := validConfig()
	cfg.DestURL = ""
	cfg.NoPush = false
	cfg.SaveDir = ""

	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil want error when DestURL is empty and NoPush/SaveDir not set")
	}
}

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"https://github.com/org/repo.git", false},
		{"https://codeberg.org/org/repo", false},
		{"http://gitlab.com/org/repo", false},
		{"git@github.com:org/repo.git", false},
		{"ssh://git@codeberg.org:22/org/repo.git", false},
		{"/tmp/bare.git", false},
		{"./local-dir", true},
		{"../local-dir", true},
		{"/tmp/saved-repo", true},
		{"~/saved-repo", true},
		{"local-dir", true},
	}

	for _, tt := range tests {
		got := IsLocalPath(tt.path)
		if got != tt.want {
			t.Errorf("IsLocalPath(%q) = %v want %v", tt.path, got, tt.want)
		}
	}
}

func TestParseAuthor(t *testing.T) {
	tests := []struct {
		input     string
		wantName  string
		wantEmail string
		wantErr   bool
	}{
		{"Jane Doe <jane@example.com>", "Jane Doe", "jane@example.com", false},
		{"<jane@example.com>", "", "jane@example.com", false},
		{"jane@example.com", "", "jane@example.com", false},
		{"  Alice <alice@example.org>  ", "Alice", "alice@example.org", false},
		{"", "", "", false},
		{"Jane Doe", "", "", true},
		{"not-an-email", "", "", true},
	}

	for _, tt := range tests {
		name, email, err := ParseAuthor(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseAuthor(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if name != tt.wantName {
			t.Errorf("ParseAuthor(%q) name = %q, want %q", tt.input, name, tt.wantName)
		}
		if email != tt.wantEmail {
			t.Errorf("ParseAuthor(%q) email = %q, want %q", tt.input, email, tt.wantEmail)
		}
	}
}
