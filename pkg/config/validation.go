package config

import (
	"fmt"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// ParseOldAuthors parses author identities from strings that may be:
// - "Name <email>"
// - "email@domain.com"
// - "Name"
// Values may be comma-separated or repeated.
func ParseOldAuthors(values []string) (names []string, emails []string) {
	for _, value := range values {
		for _, part := range splitOldNames(value) {
			name, email := parseAuthorOrName(part)
			if name != "" && !slices.Contains(names, name) {
				names = append(names, name)
			}
			if email != "" && !slices.Contains(emails, email) {
				emails = append(emails, email)
			}
			if name == "" && email == "" && !slices.Contains(names, part) {
				names = append(names, part)
			}
		}
	}
	return names, emails
}

func parseAuthorOrName(raw string) (name, email string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if addr, err := mail.ParseAddress(raw); err == nil && addr.Address != "" {
		return addr.Name, addr.Address
	}
	if strings.Contains(raw, "@") && !strings.Contains(raw, " ") {
		return "", strings.Trim(raw, "<>")
	}
	return raw, ""
}

// ParseAuthor parses a Git author identity in "Name <email>" or bare "email" format.
func ParseAuthor(raw string) (name, email string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", nil
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid author format: %q (expected \"Name <email>\" or \"email\")", raw)
	}
	return addr.Name, addr.Address, nil
}

func Validate(cfg *Config) error {
	if cfg.SourceURL == "" {
		return fmt.Errorf("source repository is required (<source>)")
	}

	if cfg.DestURL == "" && !cfg.NoPush && cfg.SaveDir == "" {
		return fmt.Errorf("destination is required (<dest>) unless --no-push or --save-dir is given")
	}

	if cfg.NewEmail == "" {
		return fmt.Errorf("new author is required (-t, --to)")
	}

	if len(cfg.OldNames) == 0 && len(cfg.OldEmails) == 0 {
		return fmt.Errorf("at least one old author name or email is required (-f, --from)")
	}

	if slices.Contains(cfg.OldNames, "") {
		return fmt.Errorf("empty string is not a valid old author name (-f, --from)")
	}

	for _, p := range cfg.StripTitlePatterns {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("empty regex pattern is not allowed for --strip-title")
		}
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid regex for --strip-title %q: %w", p, err)
		}
	}

	for _, p := range cfg.StripBodyPatterns {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("empty regex pattern is not allowed for --strip-body")
		}
		if _, err := regexp.Compile(p); err != nil {
			return fmt.Errorf("invalid regex for --strip-body %q: %w", p, err)
		}
	}

	if cfg.GPGKey != "" && cfg.SSHKey != "" {
		return fmt.Errorf("cannot use both --gpg and --ssh")
	}

	if cfg.SignMode == "none" && (cfg.GPGKey != "" || cfg.SSHKey != "") {
		return fmt.Errorf("cannot combine --no-sign with --gpg or --ssh")
	}

	if cfg.SignMode == "" {
		switch {
		case cfg.GPGKey != "":
			cfg.SignMode = "gpg"
		case cfg.SSHKey != "":
			cfg.SignMode = "ssh"
		default:
			cfg.SignMode = "keep"
		}
	}

	if cfg.SignMode == "gpg" && cfg.GPGKey == "" {
		return fmt.Errorf("GPG key is required (--gpg)")
	}

	if cfg.SignMode == "ssh" && cfg.SSHKey == "" {
		return fmt.Errorf("SSH key path is required (--ssh)")
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
		if cfg.CoauthorMode == "interactive" && !cfg.Interactive {
			cfg.Interactive = true
		}
		if len(cfg.CoauthorFrom) == 0 {
			switch {
			case len(cfg.OldRawAuthors) > 0:
				cfg.CoauthorFrom = cfg.OldRawAuthors
			case len(cfg.OldNames) > 0:
				cfg.CoauthorFrom = cfg.OldNames
			default:
				cfg.CoauthorFrom = cfg.OldEmails
			}
		}
		if cfg.CoauthorNewName == "" {
			cfg.CoauthorNewName = cfg.NewName
		}
		if cfg.CoauthorNewEmail == "" {
			cfg.CoauthorNewEmail = cfg.NewEmail
		}
		if cfg.CoauthorMode == "rewrite" && cfg.CoauthorNewName == "" {
			return fmt.Errorf("coauthor replacement name is required when --coauthor=rewrite (--coauthor-to or -t)")
		}
	}

	return nil
}

// IsLocalPath reports whether a destination path refers to a local directory
// rather than a remote Git repository URL.
func IsLocalPath(s string) bool {
	if strings.Contains(s, "://") || strings.HasPrefix(s, "git@") || strings.HasSuffix(s, ".git") {
		return false
	}
	return filepath.IsAbs(s) || strings.HasPrefix(s, ".") || strings.HasPrefix(s, "~") || !strings.Contains(s, ":")
}

func CheckDependencies(cfg *Config) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed")
	}

	if _, err := exec.LookPath("git-filter-repo"); err != nil {
		return fmt.Errorf("git-filter-repo is not installed (required dependency)")
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
		fi, err := os.Stat(cfg.SSHKey)
		if err != nil {
			return fmt.Errorf("SSH key file error: %w", err)
		}
		if fi.IsDir() {
			return fmt.Errorf("SSH key path is a directory: %s", cfg.SSHKey)
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
