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
		Name:      "git-reforge",
		Usage:     "Git repository forge migration and history rewriting tool",
		ArgsUsage: "<source> [dest]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "to",
				Aliases: []string{"t"},
				Usage:   "New author identity (\"Name <email>\" or \"email\")",
			},
			&cli.StringSliceFlag{
				Name:    "from",
				Aliases: []string{"f", "old-name", "o"},
				Usage:   "Old author name(s) to replace",
			},
			&cli.BoolFlag{
				Name:    "interactive",
				Aliases: []string{"i"},
				Usage:   "Enable interactive mode for per-commit editing",
			},
			&cli.StringFlag{
				Name:  "gpg",
				Usage: "GPG key ID for re-signing commits and tags",
			},
			&cli.StringFlag{
				Name:  "ssh",
				Usage: "SSH key path for re-signing commits and tags",
			},
			&cli.BoolFlag{
				Name:    "no-sign",
				Aliases: []string{"nosign"},
				Usage:   "Strip all commit and tag signatures",
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
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Skip confirmation prompt",
			},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			cfg.SaveDir = c.String("save-dir")
			cfg.NoPush = c.Bool("no-push")

			cfg.SourceURL = c.Args().Get(0)
			if dest := c.Args().Get(1); dest != "" {
				if config.IsLocalPath(dest) {
					if cfg.SaveDir == "" {
						cfg.SaveDir = dest
					}
					cfg.NoPush = true
				} else {
					cfg.DestURL = dest
				}
			}

			if to := c.String("to"); to != "" {
				name, email, err := config.ParseAuthor(to)
				if err != nil {
					log.Error(err.Error())
					return err
				}
				cfg.NewName = name
				cfg.NewEmail = email
			}

			if gpg := c.String("gpg"); gpg != "" {
				cfg.SignMode = "gpg"
				cfg.GPGKey = gpg
			}
			if ssh := c.String("ssh"); ssh != "" {
				cfg.SignMode = "ssh"
				cfg.SSHKey = ssh
			}
			if c.Bool("no-sign") {
				cfg.SignMode = "none"
			}

			cfg.Interactive = c.Bool("interactive")
			cfg.Yes = c.Bool("yes")
			cfg.CoauthorMode = c.String("coauthor-mode")
			cfg.CoauthorOldName = c.String("coauthor-old")
			cfg.CoauthorOldEmail = c.String("coauthor-old-email")
			cfg.CoauthorNewName = c.String("coauthor-new-name")
			cfg.CoauthorNewEmail = c.String("coauthor-new-email")

			cfg.OldNames = config.ParseOldNames(c.StringSlice("from"))

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
