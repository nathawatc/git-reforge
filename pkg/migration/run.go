package migration

import (
	"fmt"
	"os"
	"strings"

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

	if git.RepoObjectFormat(cfg.WorkDir()) == "sha256" && strings.Contains(cfg.DestURL, "github.com") {
		proceed, err := ui.PromptConvertToSHA1(cfg)
		if err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("migration cancelled: sha256 to sha1 conversion was declined")
		}
		if err := git.ConvertToSHA1(cfg); err != nil {
			return err
		}
	}

	if cfg.Interactive {
		ui.InteractiveEdit(cfg)
		if err := rewrite.RewriteCommitsInteractive(cfg); err != nil {
			return err
		}
		if err := coauthor.RunCoauthorPhase(cfg); err != nil {
			return err
		}
		if err := signing.HandleSigning(cfg); err != nil {
			return err
		}
	} else {
		if err := rewrite.RewriteCommitsBatch(cfg); err != nil {
			return err
		}
		if err := signing.ResignTags(cfg); err != nil {
			return err
		}
	}

	if err := git.PruneOriginalRefs(cfg.WorkDir()); err != nil {
		return err
	}

	if cfg.SaveDir != "" {
		if err := git.Save(cfg.WorkDir(), cfg.SaveDir); err != nil {
			return err
		}
	}

	if !cfg.NoPush {
		if err := git.Push(cfg); err != nil {
			if git.IsHashAlgorithmError(err) && git.RepoObjectFormat(cfg.WorkDir()) == "sha256" {
				proceed, promptErr := ui.PromptConvertToSHA1(cfg)
				if promptErr == nil && proceed {
					if convErr := git.ConvertToSHA1(cfg); convErr == nil {
						if retryErr := git.Push(cfg); retryErr == nil {
							ui.ShowSummary(cfg)
							return nil
						}
					}
				}
				return git.FormatHashAlgorithmError(cfg.SourceURL, "sha256", "sha1")
			}
			if saveErr := ui.PromptSaveOrCleanup(cfg, "push"); saveErr != nil {
				return saveErr
			}
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
