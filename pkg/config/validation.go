package config

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func ParseOldNames(values []string) []string {
	var names []string
	for _, value := range values {
		names = append(names, splitOldNames(value)...)
	}
	return names
}

func Validate(cfg *Config) error {
	if cfg.SourceURL == "" {
		return fmt.Errorf("source URL is required (-s, --source)")
	}

	if cfg.DestURL == "" && !cfg.NoPush && cfg.SaveDir == "" {
		return fmt.Errorf("destination URL is required (-d, --dest) unless --no-push or --save-dir is given")
	}

	if cfg.NewEmail == "" {
		return fmt.Errorf("new email is required (-e, --new-email)")
	}

	if len(cfg.OldNames) == 0 {
		return fmt.Errorf("at least one old author name is required (-o, --old-name)")
	}

	if slices.Contains(cfg.OldNames, "") {
		return fmt.Errorf("empty string is not a valid old author name (-o, --old-name)")
	}

	if cfg.SignMode == "" {
		cfg.SignMode = "keep"
	}

	if cfg.SignMode == "gpg" && cfg.GPGKey == "" {
		return fmt.Errorf("GPG key is required when using --sign-mode=gpg (--gpg-key)")
	}

	if cfg.SignMode == "ssh" && cfg.SSHKey == "" {
		return fmt.Errorf("SSH key path is required when using --sign-mode=ssh (--ssh-key)")
	}

	if !slices.Contains([]string{"keep", "gpg", "ssh", "none"}, cfg.SignMode) {
		return fmt.Errorf("invalid sign mode: %s (must be: keep|gpg|ssh|none)", cfg.SignMode)
	}

	if cfg.CoauthorMode != "" && !slices.Contains([]string{"rewrite", "remove", "interactive"}, cfg.CoauthorMode) {
		return fmt.Errorf("invalid coauthor mode: %s (must be: rewrite|remove|interactive)", cfg.CoauthorMode)
	}

	if cfg.SaveDir != "" {
		if _, err := os.Stat(cfg.SaveDir); err == nil {
			return fmt.Errorf("save directory already exists: %s", cfg.SaveDir)
		}
	}

	if cfg.DestURL == "" && cfg.SaveDir != "" {
		cfg.NoPush = true
	}

	if cfg.CoauthorMode != "" {
		if cfg.CoauthorOldName == "" && len(cfg.OldNames) > 0 {
			cfg.CoauthorOldName = cfg.OldNames[0]
		}
		if cfg.CoauthorNewName == "" {
			cfg.CoauthorNewName = cfg.NewName
		}
		if cfg.CoauthorNewEmail == "" {
			cfg.CoauthorNewEmail = cfg.NewEmail
		}
	}

	return nil
}

func CheckDependencies(cfg *Config) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}

	if cfg.SignMode == "gpg" {
		if _, err := exec.LookPath("gpg"); err != nil {
			return fmt.Errorf("gpg is not installed (required for GPG signing)")
		}
		if err := exec.Command("gpg", "--list-secret-keys", cfg.GPGKey).Run(); err != nil {
			return fmt.Errorf("GPG key not found: %s", cfg.GPGKey)
		}
	}

	if cfg.SignMode == "ssh" {
		if _, err := exec.LookPath("ssh-keygen"); err != nil {
			return fmt.Errorf("ssh-keygen is not installed (required for SSH signing)")
		}
		if _, err := os.Stat(cfg.SSHKey); os.IsNotExist(err) {
			return fmt.Errorf("SSH key file not found: %s", cfg.SSHKey)
		}
	}

	return nil
}

func splitOldNames(s string) []string {
	var names []string
	for part := range strings.SplitSeq(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}
