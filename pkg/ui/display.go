package ui

import (
	"fmt"
	"os"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/git"
)

var ErrCancelled = fmt.Errorf("migration cancelled")

func DisplayConfig(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Migration Configuration")

	fmt.Printf("  %sSource:%s        %s\n", Bold, NC, cfg.SourceURL)
	if cfg.DestURL != "" {
		fmt.Printf("  %sDestination:%s   %s\n", Bold, NC, cfg.DestURL)
	} else {
		fmt.Printf("  %sDestination:%s   %s(none — push skipped)%s\n", Bold, NC, Yellow, NC)
	}
	fmt.Println()

	displayedOld := cfg.DisplayedOldAuthors()

	if len(displayedOld) == 1 {
		fmt.Printf("  %sOld Author:%s    %s\n", Bold, NC, displayedOld[0])
	} else if len(displayedOld) > 1 {
		fmt.Printf("  %sOld Authors:%s   %s\n", Bold, NC, displayedOld[0])
		for _, oldName := range displayedOld[1:] {
			fmt.Printf("                 %s\n", oldName)
		}
	}
	if cfg.NewName != "" {
		fmt.Printf("  %sNew Author:%s    %s\n", Bold, NC, cfg.NewName)
	} else {
		fmt.Printf("  %sNew Author:%s    %s(keep original)%s\n", Bold, NC, Yellow, NC)
	}
	fmt.Printf("  %sNew Email:%s     %s\n", Bold, NC, cfg.NewEmail)
	fmt.Println()
	fmt.Printf("  %sInteractive:%s   %t\n", Bold, NC, cfg.Interactive)
	fmt.Printf("  %sSign Mode:%s     %s\n", Bold, NC, cfg.SignMode)
	if cfg.GPGKey != "" {
		fmt.Printf("  %sGPG Key:%s       %s\n", Bold, NC, cfg.GPGKey)
	}
	if cfg.SSHKey != "" {
		fmt.Printf("  %sSSH Key:%s       %s\n", Bold, NC, cfg.SSHKey)
	}
	if cfg.NoPush {
		fmt.Printf("  %sPush to Dest:%s  %s\n", Bold, NC, "no (--no-push)")
	} else {
		fmt.Printf("  %sPush to Dest:%s  %s\n", Bold, NC, "yes")
	}
	if cfg.SaveDir != "" {
		fmt.Printf("  %sSave To:%s       %s\n", Bold, NC, cfg.SaveDir)
	}

	if cfg.CoauthorMode != "" {
		fmt.Println()
		fmt.Printf("  %sCo-author Mode:%s  %s\n", Bold, NC, cfg.CoauthorMode)
		coauthorMatch := strings.Join(cfg.CoauthorFrom, ", ")
		if coauthorMatch == "" {
			coauthorMatch = strings.Join(cfg.OldNames, ", ")
		}
		fmt.Printf("  %sCo-author Match:%s %s\n", Bold, NC, coauthorMatch)
		if cfg.CoauthorMode == "rewrite" {
			fmt.Printf("  %sCo-author New:%s   %s <%s>\n", Bold, NC, cfg.CoauthorNewName, cfg.CoauthorNewEmail)
		}
	}

	if len(cfg.StripTitlePatterns) > 0 {
		fmt.Printf("  %sStrip Title:%s     %s\n", Bold, NC, strings.Join(cfg.StripTitlePatterns, ", "))
	}
	if len(cfg.StripBodyPatterns) > 0 {
		fmt.Printf("  %sStrip Body:%s      %s\n", Bold, NC, strings.Join(cfg.StripBodyPatterns, ", "))
	}
	fmt.Println()

	if cfg.NoPush && cfg.SaveDir == "" {
		fmt.Printf("  %sNote:%s --no-push is set and --save-dir is not. The rewritten\n", Yellow, NC)
		fmt.Printf("  %s      %s repository will only exist in the temporary directory\n", Yellow, NC)
		fmt.Printf("  %s      %s until the script exits. Use --save-dir <path> to keep it.\n", Yellow, NC)
		fmt.Println()
	}

	if cfg.Yes {
		return nil
	}

	prompts := NewPromptSession()
	defer func() {
		if err := prompts.Close(); err != nil {
			log.Warning(err.Error())
		}
	}()
	reply, err := prompts.ReadLine("Proceed with migration? [y/N] ")
	if err != nil {
		log.Warning(err.Error())
		log.Info("Migration cancelled")
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(reply), "Y") {
		log.Info("Migration cancelled")
		return ErrCancelled
	}
	return nil
}

func ShowSummary(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Migration Complete!")

	fmt.Printf("  %s✓%s Repository history rewritten successfully\n", Green, NC)
	if cfg.NewName != "" {
		displayedOld := cfg.DisplayedOldAuthors()

		if len(displayedOld) == 1 {
			fmt.Printf("  %s✓%s Author '%s' changed to '%s'\n", Green, NC, displayedOld[0], cfg.NewName)
		} else {
			fmt.Printf("  %s✓%s Authors changed to '%s':\n", Green, NC, cfg.NewName)
			for _, oldName := range displayedOld {
				fmt.Printf("      - %s\n", oldName)
			}
		}
	}
	fmt.Printf("  %s✓%s Email changed to '%s'\n", Green, NC, cfg.NewEmail)
	fmt.Printf("  %s✓%s Original commit dates preserved\n", Green, NC)

	if cfg.CoauthorMode != "" {
		switch cfg.CoauthorMode {
		case "rewrite":
			fmt.Printf("  %s✓%s Co-authored-by trailers rewritten to '%s <%s>'\n",
				Green, NC, cfg.CoauthorNewName, cfg.CoauthorNewEmail)
		case "remove":
			fmt.Printf("  %s✓%s Matching Co-authored-by trailers removed\n", Green, NC)
		case "interactive":
			rewriteCount := 0
			removeCount := 0
			for _, action := range cfg.State.CoauthorActions {
				switch action {
				case "rewrite":
					rewriteCount++
				case "remove":
					removeCount++
				}
			}
			fmt.Printf("  %s✓%s Co-authored-by trailers: %d rewritten, %d removed\n",
				Green, NC, rewriteCount, removeCount)
		}
	}

	if len(cfg.StripTitlePatterns) > 0 {
		fmt.Printf("  %s✓%s Stripped %d title pattern(s) from commit history\n",
			Green, NC, len(cfg.StripTitlePatterns))
	}
	if len(cfg.StripBodyPatterns) > 0 {
		fmt.Printf("  %s✓%s Stripped %d body pattern(s) from commit history\n",
			Green, NC, len(cfg.StripBodyPatterns))
	}

	switch cfg.SignMode {
	case "keep":
		fmt.Printf("  %s!%s Original signatures kept (but are now invalid)\n", Yellow, NC)
	case "none":
		fmt.Printf("  %s✓%s Signatures removed\n", Green, NC)
	case "gpg":
		fmt.Printf("  %s✓%s Commits re-signed with GPG key: %s\n", Green, NC, cfg.GPGKey)
	case "ssh":
		fmt.Printf("  %s✓%s Commits re-signed with SSH key: %s\n", Green, NC, cfg.SSHKey)
	}

	fmt.Println()
	if cfg.NoPush {
		fmt.Printf("  %s!%s Push to destination repository was skipped\n", Yellow, NC)
	} else {
		fmt.Printf("  %sDestination Repository:%s %s\n", Bold, NC, cfg.DestURL)
	}

	if cfg.SaveDir != "" {
		fmt.Printf("  %s✓%s Rewritten repository saved to: %s\n", Green, NC, cfg.SaveDir)
		if cfg.NoPush && cfg.DestURL != "" {
			fmt.Println()
			fmt.Printf("  %sTo push later, run:%s\n", Cyan, NC)
			fmt.Printf("    cd %s\n", cfg.SaveDir)
			fmt.Printf("    git remote set-url origin %s\n", cfg.DestURL)
			fmt.Printf("    git push --all --force\n")
			fmt.Printf("    git push --force origin \"refs/tags/*:refs/tags/*\"\n")
		}
	}

	fmt.Println()
}

func PromptSaveOrCleanup(cfg *config.Config, failedStep string) (retErr error) {
	if cfg.SaveDir != "" {
		cfg.Logger.Warning("Push failed during " + failedStep + ".")
		cfg.Logger.Info("The rewritten repository will still be saved to: " + cfg.SaveDir)
		return nil
	}

	prompts := NewPromptSession()
	defer func() {
		if err := prompts.Close(); err != nil && retErr == nil {
			retErr = fmt.Errorf("failed to close prompt session: %w", err)
		}
	}()

	if !prompts.HasTTY() {
		return fmt.Errorf("failed during %s and no terminal is available to choose save or cleanup", failedStep)
	}

	if _, err := prompts.Printf(
		"\n"+
			"The %s to destination repository failed.\n"+
			"The rewritten repository is still available in the temporary directory.\n"+
			"\n"+
			"  [S]ave    - copy the rewritten repository to a local path\n"+
			"  [C]leanup - discard the temporary directory and exit\n"+
			"\n",
		failedStep,
	); err != nil {
		return fmt.Errorf("failed to write push recovery message: %w", err)
	}

	for {
		choice, err := prompts.ReadLine("  Choice [S/C]: ")
		if err != nil {
			return fmt.Errorf("failed to read push recovery choice: %w", err)
		}

		switch strings.ToUpper(strings.TrimSpace(choice)) {
		case "S":
			return promptSavePath(cfg, prompts)
		case "C":
			cfg.Logger.Info("Cleanup selected. The temporary directory will be removed on exit.")
			if err := os.RemoveAll(cfg.TempDir); err != nil {
				return fmt.Errorf("failed to clean up temp directory %s: %w", cfg.TempDir, err)
			}
			cfg.TempDir = ""
			return nil
		default:
			cfg.Logger.Warning("Invalid choice. Please enter S to save or C to clean up.")
		}
	}
}

func promptSavePath(cfg *config.Config, prompts *PromptSession) error {
	for {
		savePath, err := prompts.ReadLine("  Enter destination path: ")
		if err != nil {
			return fmt.Errorf("failed to read save path: %w", err)
		}

		savePath = strings.TrimSpace(savePath)
		if savePath == "" {
			cfg.Logger.Warning("Path cannot be empty. Please enter a valid directory path.")
			continue
		}
		if _, err := os.Stat(savePath); err == nil {
			cfg.Logger.Warning("Path already exists: " + savePath + " - choose a path that does not exist.")
			continue
		}

		cfg.Logger.Info("Saving rewritten repository to: " + savePath)
		if err := git.Save(cfg.WorkDir(), savePath); err != nil {
			return fmt.Errorf("failed to save repository to %s: %w", savePath, err)
		}

		cfg.Logger.Success("Repository saved to: " + savePath)
		cfg.Logger.Info("You can push it later with:")
		if _, err := prompts.Printf(
			"    cd %s\n"+
				"    git remote set-url origin %s\n"+
				"    git push --all --force\n"+
				"    git push --force origin \"refs/tags/*:refs/tags/*\"\n",
			savePath,
			cfg.DestURL,
		); err != nil {
			return fmt.Errorf("failed to write save instructions: %w", err)
		}
		return nil
	}
}

// PromptConvertToSHA1 asks the user whether to convert a sha256 repository to sha1.
func PromptConvertToSHA1(cfg *config.Config) (bool, error) {
	if cfg.Yes {
		return true, nil
	}

	prompts := NewPromptSession()
	defer func() {
		_ = prompts.Close()
	}()

	if !prompts.HasTTY() {
		return false, fmt.Errorf("source repository uses sha256 but destination requires sha1 (no terminal available; use -y to auto-convert)")
	}

	if _, err := prompts.Printf(
		"\n  %sNotice:%s Source repository uses %ssha256%s, but destination requires %ssha1%s (e.g. GitHub).\n",
		Yellow, NC, Bold, NC, Bold, NC,
	); err != nil {
		return false, err
	}

	reply, err := prompts.ReadLine("  Convert repository from sha256 to sha1? [y/N] ")
	if err != nil {
		return false, fmt.Errorf("failed to read sha1 conversion choice: %w", err)
	}

	return strings.EqualFold(strings.TrimSpace(reply), "Y"), nil
}
