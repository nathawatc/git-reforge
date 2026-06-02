package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github-to-codeberg/pkg/coauthor"
	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/git"
	"github-to-codeberg/pkg/logging"
	"github-to-codeberg/pkg/rewrite"
	"github-to-codeberg/pkg/signing"
	"github-to-codeberg/pkg/ui"
)

func main() {
	log := logging.New()
	cfg := config.New()
	cfg.Logger = log

	cmd := &cli.Command{
		Name:  "migrate",
		Usage: "GitHub to Codeberg migration script",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "source",
				Aliases: []string{"s"},
				Usage:   "GitHub repository URL",
			},
			&cli.StringFlag{
				Name:    "dest",
				Aliases: []string{"d"},
				Usage:   "Codeberg repository URL",
			},
			&cli.StringFlag{
				Name:    "new-name",
				Aliases: []string{"n"},
				Usage:   "New author name",
			},
			&cli.StringFlag{
				Name:    "new-email",
				Aliases: []string{"e"},
				Usage:   "New author email",
			},
			&cli.StringSliceFlag{
				Name:    "old-name",
				Aliases: []string{"o"},
				Usage:   "Old author name(s) to replace",
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "Enable interactive mode for per-commit editing",
			},
			&cli.StringFlag{
				Name:  "sign-mode",
				Usage: "Signing mode: keep|gpg|ssh|none (default: keep)",
			},
			&cli.StringFlag{
				Name:  "gpg-key",
				Usage: "GPG key ID for re-signing",
			},
			&cli.StringFlag{
				Name:  "ssh-key",
				Usage: "SSH key path for re-signing",
			},
			&cli.BoolFlag{
				Name:  "nosign",
				Usage: "Shorthand for --sign-mode=none",
			},
			&cli.StringFlag{
				Name:  "coauthor-mode",
				Usage: "Co-authored-by trailer mode: rewrite|remove|interactive",
			},
			&cli.StringFlag{
				Name:  "coauthor-old",
				Usage: "Name to match in Co-authored-by trailers",
			},
			&cli.StringFlag{
				Name:  "coauthor-old-email",
				Usage: "Email to match in Co-authored-by trailers",
			},
			&cli.StringFlag{
				Name:  "coauthor-new-name",
				Usage: "Replacement name for Co-authored-by",
			},
			&cli.StringFlag{
				Name:  "coauthor-new-email",
				Usage: "Replacement email for Co-authored-by",
			},
			&cli.StringFlag{
				Name:  "save-dir",
				Usage: "Save the rewritten repository to <path>",
			},
			&cli.BoolFlag{
				Name:  "no-push",
				Usage: "Rewrite history locally but do not push",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			cfg.SourceURL = c.String("source")
			cfg.DestURL = c.String("dest")
			cfg.NewName = c.String("new-name")
			cfg.NewEmail = c.String("new-email")
			cfg.Interactive = c.Bool("interactive")
			cfg.SignMode = c.String("sign-mode")
			cfg.GPGKey = c.String("gpg-key")
			cfg.SSHKey = c.String("ssh-key")
			cfg.SaveDir = c.String("save-dir")
			cfg.NoPush = c.Bool("no-push")
			cfg.CoauthorMode = c.String("coauthor-mode")
			cfg.CoauthorOldName = c.String("coauthor-old")
			cfg.CoauthorOldEmail = c.String("coauthor-old-email")
			cfg.CoauthorNewName = c.String("coauthor-new-name")
			cfg.CoauthorNewEmail = c.String("coauthor-new-email")

			if c.Bool("nosign") {
				cfg.SignMode = "none"
			}

			cfg.OldNames = append(cfg.OldNames, collectOldNames(c.StringSlice("old-name"))...)

			if err := validateArgs(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			checkDependencies(cfg)
			ui.DisplayConfig(cfg)
			defer cleanupTempDir(cfg)

			if err := git.Clone(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			if cfg.Interactive {
				ui.InteractiveEdit(cfg)
				if err := rewrite.RewriteCommitsInteractive(cfg); err != nil {
					log.Error(err.Error())
					return err
				}
			} else {
				if err := rewrite.RewriteCommitsBatch(cfg); err != nil {
					log.Error(err.Error())
					return err
				}
			}

			if err := coauthor.RunCoauthorPhase(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			if err := signing.HandleSigning(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			if cfg.SaveDir != "" {
				if err := git.Save(cfg.TempDir, cfg.SaveDir); err != nil {
					log.Error(err.Error())
					return err
				}
			}

			if !cfg.NoPush {
				if err := git.Push(cfg); err != nil {
					log.Error(err.Error())
					return err
				}
			} else {
				log.Info("Skipping push to Codeberg")
			}

			ui.ShowSummary(cfg)

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cleanupTempDir(cfg *config.Config) {
	if cfg.TempDir == "" {
		return
	}
	if err := os.RemoveAll(cfg.TempDir); err != nil {
		cfg.Logger.Warning("Failed to clean up temp directory: " + cfg.TempDir)
	}
}

func validateArgs(cfg *config.Config) error {
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

func checkDependencies(cfg *config.Config) {
	log := cfg.Logger
	log.Info("Checking dependencies...")

	if _, err := exec.LookPath("git"); err != nil {
		log.Error("git is not installed")
		os.Exit(1)
	}

	if cfg.SignMode == "gpg" {
		if _, err := exec.LookPath("gpg"); err != nil {
			log.Error("gpg is not installed (required for GPG signing)")
			os.Exit(1)
		}
		if err := exec.Command("gpg", "--list-secret-keys", cfg.GPGKey).Run(); err != nil {
			log.Error("GPG key not found: " + cfg.GPGKey)
			os.Exit(1)
		}
	}

	if cfg.SignMode == "ssh" {
		if _, err := exec.LookPath("ssh-keygen"); err != nil {
			log.Error("ssh-keygen is not installed (required for SSH signing)")
			os.Exit(1)
		}
		if _, err := os.Stat(cfg.SSHKey); os.IsNotExist(err) {
			log.Error("SSH key file not found: " + cfg.SSHKey)
			os.Exit(1)
		}
	}

	log.Success("All dependencies satisfied")
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

func collectOldNames(values []string) []string {
	var names []string
	for _, value := range values {
		names = append(names, splitOldNames(value)...)
	}
	return names
}
