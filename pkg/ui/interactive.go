package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/git"
)

func InteractiveEdit(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Interactive Commit Editing")

	targets := cfg.DisplayedOldAuthors()
	log.Info("Fetching commits from author(s): " + strings.Join(targets, ", "))

	// Get all candidate commits by old authors or committers
	var candidates []string
	for _, oldName := range cfg.OldNames {
		candidates = append(candidates, getCommitsByIdentity(cfg.WorkDir(), oldName)...)
	}
	for _, oldEmail := range cfg.OldEmails {
		candidates = append(candidates, getCommitsByIdentity(cfg.WorkDir(), oldEmail)...)
	}

	// Verify candidate commits match old author/committer identities exactly
	var commits []string
	seen := make(map[string]bool)
	for _, h := range candidates {
		if seen[h] {
			continue
		}
		seen[h] = true
		info, err := git.CommitDetails(cfg.WorkDir(), h)
		if err != nil {
			continue
		}
		if slices.Contains(cfg.OldNames, info.AuthorName) || slices.Contains(cfg.OldEmails, info.AuthorEmail) ||
			slices.Contains(cfg.OldNames, info.CommitterName) || slices.Contains(cfg.OldEmails, info.CommitterEmail) {
			commits = append(commits, h)
		}
	}

	if len(commits) == 0 {
		log.Warning("No commits found from author(s): " + strings.Join(targets, ", "))
		return
	}

	log.Info(fmt.Sprintf("Found %d commit(s) to review", len(commits)))
	fmt.Println()

	prompts := NewPromptSession()
	defer func() {
		if err := prompts.Close(); err != nil {
			log.Warning(err.Error())
		}
	}()

	current := 0
	for _, commitHash := range commits {
		current++
		remainingCommits := commits[current-1:]
		for {
			override, hasOverride := cfg.State.CommitOverrides[commitHash]
			info, _ := git.CommitDetails(cfg.WorkDir(), commitHash)

			fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
			fmt.Printf("%sCommit %d of %d%s\n", Bold, current, len(commits), NC)
			fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
			fmt.Printf("  %sHash:%s       %s\n", Cyan, NC, commitHash[:12])
			fmt.Printf("  %sAuthor:%s     %s <%s>\n", Cyan, NC, info.AuthorName, info.AuthorEmail)
			fmt.Printf("  %sDate:%s       %s\n", Cyan, NC, info.AuthorDate)
			fmt.Printf("  %sMessage:%s    %s\n", Cyan, NC, info.Subject)
			fmt.Println()

			switch {
			case hasOverride && !override.Skip:
				fmt.Printf("  %sChanges:%s\n", Bold, NC)
				fmt.Printf("    Name:    %s\n", override.Name)
				fmt.Printf("    Email:   %s\n", override.Email)
			case hasOverride && override.Skip:
				fmt.Printf("  %sSkipped%s - No changes applied\n", Bold, NC)
			default:
				fmt.Printf("  %sDefault changes:%s\n", Bold, NC)
				targetName := cfg.NewName
				if targetName == "" {
					targetName = info.AuthorName
				}
				fmt.Printf("    Name:    %s → %s\n", info.AuthorName, targetName)
				fmt.Printf("    Email:   %s → %s\n", info.AuthorEmail, cfg.NewEmail)
			}

			msgStatus := "(unchanged)"
			if _, ok := cfg.State.CommitMessages[commitHash]; ok {
				msgStatus = fmt.Sprintf("%s(customized)%s", Yellow, NC)
			}
			fmt.Printf("    Message: %s\n", msgStatus)
			fmt.Println()

			choice := promptCommitChoice(commitHash, remainingCommits, hasOverride, info, cfg, prompts)
			if choice == "quit" {
				return
			}
			if choice == "next" {
				break
			}
			// "" means E or M was selected, loop re-reads state and re-prompts
		}
		fmt.Println()
	}
}

func getCommitsByIdentity(workDir string, name string) []string {
	escaped := regexp.QuoteMeta(name)

	var commits []string
	commits = append(commits, getCommitsByLogFilter(workDir, "--author="+escaped)...)
	commits = append(commits, getCommitsByLogFilter(workDir, "--committer="+escaped)...)
	return uniqueList(commits)
}

func getCommitsByLogFilter(workDir string, filter string) []string {
	cmd := exec.Command("git", "log", "--all", "--format=%H", filter)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	var commits []string
	for line := range strings.SplitSeq(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits
}

func promptCommitChoice(commitHash string, remainingCommits []string, hasOverride bool, info git.CommitInfo, cfg *config.Config, prompts *PromptSession) string {

	for {
		if hasOverride {
			fmt.Printf("  %s[A]pply%s - Apply changes\n", Green, NC)
		} else {
			fmt.Printf("  %s[A]pply%s - Apply default changes\n", Green, NC)
		}
		fmt.Printf("  %s[E]dit%s - Customize name/email for this commit\n", Yellow, NC)
		fmt.Printf("  %s[M]essage%s - Edit commit message\n", Yellow, NC)
		fmt.Printf("  %s[S]kip%s - Keep original (no changes)\n", Blue, NC)
		fmt.Printf("  %s[D]efault all%s - Apply default to all remaining\n", Cyan, NC)
		fmt.Printf("  %s[Q]uit%s - Stop and apply changes made so far\n", Red, NC)
		fmt.Println()

		choice, err := prompts.ReadLine("  Choice [E/A/M/S/D/Q]: ")
		if err != nil {
			cfg.Logger.Warning("Failed to read interactive choice: " + err.Error())
			return "quit"
		}
		choice = strings.ToUpper(choice)

		switch choice {
		case "E":
			fmt.Println()
			customName, err := prompts.ReadLine("  New author name [" + cfg.NewName + "]: ")
			if err != nil {
				cfg.Logger.Warning("Failed to read author name: " + err.Error())
				return "quit"
			}
			if customName == "" {
				customName = cfg.NewName
			}

			customEmail, err := prompts.ReadLine("  New author email [" + cfg.NewEmail + "]: ")
			if err != nil {
				cfg.Logger.Warning("Failed to read author email: " + err.Error())
				return "quit"
			}
			if customEmail == "" {
				customEmail = cfg.NewEmail
			}

			cfg.State.CommitOverrides[commitHash] = config.CommitOverride{
				Name:  customName,
				Email: customEmail,
			}
			cfg.Logger.Success("Custom name/email saved for commit " + commitHash[:12])
			fmt.Println()
			return ""
		case "M":
			fmt.Println()
			fmt.Printf("  %sCurrent message:%s\n", Cyan, NC)
			fmt.Printf("    %s\n", info.Subject)
			fmt.Println()
			fmt.Printf("  %sEnter new commit message (or press Enter to keep current):%s\n", Cyan, NC)
			customMessage, err := prompts.ReadLine("")
			if err != nil {
				cfg.Logger.Warning("Failed to read commit message: " + err.Error())
				return "quit"
			}
			if customMessage != "" {
				cfg.State.CommitMessages[commitHash] = customMessage
				cfg.Logger.Success("Custom message saved for commit " + commitHash[:12])
			} else {
				cfg.Logger.Info("Keeping original message for commit " + commitHash[:12])
			}
			fmt.Println()
			return ""
		case "A":
			if !hasOverride {
				targetName := cfg.NewName
				if targetName == "" {
					targetName = info.AuthorName
				}
				cfg.State.CommitOverrides[commitHash] = config.CommitOverride{
					Name:  targetName,
					Email: cfg.NewEmail,
				}
			}
			cfg.Logger.Success("Changes will be applied to commit " + commitHash[:12])
			return "next"
		case "S":
			cfg.State.CommitOverrides[commitHash] = config.CommitOverride{Skip: true}
			cfg.Logger.Info("Commit " + commitHash[:12] + " will be kept unchanged")
			return "next"
		case "D":
			cfg.Logger.Info("Applying default changes to all remaining commits...")
			for _, remainingHash := range remainingCommits {
				cfg.State.CommitOverrides[remainingHash] = config.CommitOverride{
					Name:  cfg.NewName,
					Email: cfg.NewEmail,
				}
			}
			return "quit"
		case "Q":
			cfg.Logger.Info("Stopping interactive mode. Changes made so far will be applied.")
			return "quit"
		default:
			cfg.Logger.Warning("Invalid choice. Please select E, M, A, S, D, or Q.")
		}
	}
}

func uniqueList(list []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
