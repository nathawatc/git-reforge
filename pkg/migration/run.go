package migration

import (
	"github-to-codeberg/pkg/coauthor"
	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/git"
	"github-to-codeberg/pkg/rewrite"
	"github-to-codeberg/pkg/signing"
	"github-to-codeberg/pkg/ui"
)

func Run(cfg *config.Config) error {
	if err := git.Clone(cfg); err != nil {
		return err
	}

	if cfg.Interactive {
		ui.InteractiveEdit(cfg)
		if err := rewrite.RewriteCommitsInteractive(cfg); err != nil {
			return err
		}
	} else {
		if err := rewrite.RewriteCommitsBatch(cfg); err != nil {
			return err
		}
	}

	if err := coauthor.RunCoauthorPhase(cfg); err != nil {
		return err
	}

	if err := signing.HandleSigning(cfg); err != nil {
		return err
	}

	if cfg.SaveDir != "" {
		if err := git.Save(cfg.TempDir, cfg.SaveDir); err != nil {
			return err
		}
	}

	if !cfg.NoPush {
		if err := git.Push(cfg); err != nil {
			return err
		}
	} else {
		cfg.Logger.Info("Skipping push to Codeberg")
	}

	ui.ShowSummary(cfg)
	return nil
}
