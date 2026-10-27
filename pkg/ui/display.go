package ui

import (
	"fmt"
	"os"
	"strings"

	"github-to-codeberg/pkg/config"
)

func DisplayConfig(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Migration Configuration")

	fmt.Printf("  %sSource:%s        %s\n", Bold, NC, cfg.SourceURL)
	if cfg.DestURL != "" {
		fmt.Printf("  %sDestination:%s   %s\n", Bold, NC, cfg.DestURL)
	} else {
		fmt.Printf("  %sDestination:%s   %s(none — push skipped)%s\n", Bold, NC, Yellow, NC)
	}
	fmt.Println()

	if len(cfg.OldNames) == 1 {
		fmt.Printf("  %sOld Author:%s    %s\n", Bold, NC, cfg.OldNames[0])
	} else {
		fmt.Printf("  %sOld Authors:%s   %s\n", Bold, NC, cfg.OldNames[0])
		for _, oldName := range cfg.OldNames[1:] {
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
		coauthorMatch := cfg.CoauthorOldName
		if cfg.CoauthorOldEmail != "" {
			coauthorMatch += " <" + cfg.CoauthorOldEmail + ">"
		}
		fmt.Printf("  %sCo-author Match:%s %s\n", Bold, NC, coauthorMatch)
		if cfg.CoauthorMode == "rewrite" {
			fmt.Printf("  %sCo-author New:%s   %s <%s>\n", Bold, NC, cfg.CoauthorNewName, cfg.CoauthorNewEmail)
		}
	}
	fmt.Println()

	if cfg.NoPush && cfg.SaveDir == "" {
		fmt.Printf("  %sNote:%s --no-push is set and --save-dir is not. The rewritten\n", Yellow, NC)
		fmt.Printf("  %s      %s repository will only exist in the temporary directory\n", Yellow, NC)
		fmt.Printf("  %s      %s until the script exits. Use --save-dir <path> to keep it.\n", Yellow, NC)
		fmt.Println()
	}

	if cfg.Yes {
		return
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
		os.Exit(0)
	}
	if !strings.EqualFold(strings.TrimSpace(reply), "Y") {
		log.Info("Migration cancelled")
		os.Exit(0)
	}
}

func ShowSummary(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Migration Complete!")

	fmt.Printf("  %s✓%s Repository migrated from GitHub to Codeberg\n", Green, NC)
	if cfg.NewName != "" {
		if len(cfg.OldNames) == 1 {
			fmt.Printf("  %s✓%s Author '%s' changed to '%s'\n", Green, NC, cfg.OldNames[0], cfg.NewName)
		} else {
			fmt.Printf("  %s✓%s Authors changed to '%s':\n", Green, NC, cfg.NewName)
			for _, oldName := range cfg.OldNames {
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
		fmt.Printf("  %s!%s Push to Codeberg was skipped\n", Yellow, NC)
	} else {
		fmt.Printf("  %sCodeberg Repository:%s %s\n", Bold, NC, cfg.DestURL)
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
