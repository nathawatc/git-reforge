package migration

import (
	"os"

	"git-reforge/pkg/coauthor"
	"git-reforge/pkg/config"
	"git-reforge/pkg/git"
	"git-reforge/pkg/rewrite"
	"git-reforge/pkg/signing"
	"git-reforge/pkg/ui"
)

func Run(cfg *config.Config) (retErr error) {
	defer func() {
		if retErr == nil {
			cleanupTempDir(cfg)
		}
	}()

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

	if err := git.PruneOriginalRefs(cfg.WorkDir()); err != nil {
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
		cfg.Logger.Info("Skipping push to destination repository")
	}

	ui.ShowSummary(cfg)
	return nil
}

func cleanupTempDir(cfg *config.Config) {
	if cfg.TempDir == "" {
		return
	}
	if err := os.RemoveAll(cfg.TempDir); err != nil {
		cfg.Logger.Warning("Failed to clean up temp directory: " + cfg.TempDir)
	}
}
