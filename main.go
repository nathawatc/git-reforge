package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"git-reforge/pkg/config"
	"git-reforge/pkg/logging"
	"git-reforge/pkg/migration"
	"git-reforge/pkg/ui"
)

func main() {
	log := logging.New()
	cfg := config.New()
	cfg.Logger = log

	cmd := &cli.Command{
		Name:  "git-reforge",
		Usage: "Git repository forge migration and history rewriting tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "source",
				Aliases: []string{"s"},
				Usage:   "Source repository URL (e.g. GitHub, Codeberg, GitLab)",
			},
			&cli.StringFlag{
				Name:    "dest",
				Aliases: []string{"d"},
				Usage:   "Destination repository URL (e.g. Codeberg, GitHub, GitLab)",
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
				Name:    "no-push",
				Usage:   "Rewrite history locally but do not push",
			},
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Skip confirmation prompt",
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
			cfg.Yes = c.Bool("yes")
			cfg.CoauthorMode = c.String("coauthor-mode")
			cfg.CoauthorOldName = c.String("coauthor-old")
			cfg.CoauthorOldEmail = c.String("coauthor-old-email")
			cfg.CoauthorNewName = c.String("coauthor-new-name")
			cfg.CoauthorNewEmail = c.String("coauthor-new-email")

			if c.Bool("nosign") {
				cfg.SignMode = "none"
			}

			cfg.OldNames = config.ParseOldNames(c.StringSlice("old-name"))

			if err := config.Validate(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			log.Info("Checking dependencies...")
			if err := config.CheckDependencies(cfg); err != nil {
				log.Error(err.Error())
				return err
			}
			log.Success("All dependencies satisfied")

			ui.DisplayConfig(cfg)

			if err := migration.Run(cfg); err != nil {
				log.Error(err.Error())
				return err
			}

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
