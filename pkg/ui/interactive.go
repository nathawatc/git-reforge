package ui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/util"
)

func InteractiveEdit(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Interactive Commit Editing")

	log.Info("Fetching commits from author(s): " + strings.Join(cfg.OldNames, ", "))

	// Get all commits by old authors
	var commits []string
	for _, oldName := range cfg.OldNames {
		commits = append(commits, getCommitsByAuthor(cfg.TempDir, oldName)...)
	}

	// Remove duplicates
	commits = uniqueList(commits)

	if len(commits) == 0 {
		log.Warning("No commits found from author(s): " + strings.Join(cfg.OldNames, ", "))
		return
	}

	log.Info(fmt.Sprintf("Found %d commit(s) to review", len(commits)))
	fmt.Println()

	current := 0
	for _, commitHash := range commits {
		current++
		authorName, authorEmail, authorDate, subject, _ := getCommitDetails(cfg.TempDir, commitHash)

		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
		fmt.Printf("%sCommit %d of %d%s\n", Bold, current, len(commits), NC)
		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
		fmt.Printf("  %sHash:%s       %s\n", Cyan, NC, commitHash[:12])
		fmt.Printf("  %sAuthor:%s     %s <%s>\n", Cyan, NC, authorName, authorEmail)
		fmt.Printf("  %sDate:%s       %s\n", Cyan, NC, authorDate)
		fmt.Printf("  %sMessage:%s    %s\n", Cyan, NC, subject)
		fmt.Println()

		hash := plumbing.NewHash(commitHash)
		msgStatus := "(unchanged)"
		if _, ok := cfg.CommitMessages[hash]; ok {
			msgStatus = fmt.Sprintf("%s(customized)%s", Yellow, NC)
		}
		fmt.Printf("  %sDefault changes:%s\n", Bold, NC)
		fmt.Printf("    Name:    %s → %s\n", authorName, cfg.NewName)
		fmt.Printf("    Email:   %s → %s\n", authorEmail, cfg.NewEmail)
		fmt.Printf("    Message: %s\n", msgStatus)
		fmt.Println()

		choice := promptCommitChoice(commitHash, cfg)
		if choice == "quit" {
			return
		}
		fmt.Println()
	}
}

func getCommitsByAuthor(tmpDir string, authorName string) []string {
	// Escape regex metacharacters
	escaped := authorName
	re := strings.NewReplacer(
		"[", "\\[", "]", "\\]", "\\", "\\\\", ".", "\\.", "^", "\\^",
		"$", "\\$", "*", "\\*", "+", "\\+", "?", "\\?", "(", "\\(",
		")", "\\)", "|", "\\|", "{", "\\{", "}", "\\}",
	)
	escaped = re.Replace(escaped)

	cmd := exec.Command("git", "log", "--all", "--format=%H", "--author="+escaped)
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var commits []string
	lines := util.SplitLines(string(output))
	for _, line := range lines {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits
}

func getCommitDetails(tmpDir string, hash string) (authorName, authorEmail, authorDate, subject, body string) {
	cmd := exec.Command("git", "log", "-1", "--format=%an", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ := cmd.Output()
	authorName = strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--format=%ae", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ = cmd.Output()
	authorEmail = strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--format=%ai", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ = cmd.Output()
	authorDate = strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--format=%s", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ = cmd.Output()
	subject = strings.TrimSpace(string(output))

	cmd = exec.Command("git", "log", "-1", "--format=%B", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ = cmd.Output()
	body = string(output)

	return
}

func promptCommitChoice(commitHash string, cfg *config.Config) string {
	hash := plumbing.NewHash(commitHash)
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("  %s[E]dit%s - Customize name/email for this commit\n", Yellow, NC)
		fmt.Printf("  %s[M]essage%s - Edit commit message\n", Yellow, NC)
		fmt.Printf("  %s[A]pply%s - Apply default changes\n", Green, NC)
		fmt.Printf("  %s[S]kip%s - Keep original (no changes)\n", Blue, NC)
		fmt.Printf("  %s[D]efault all%s - Apply default to all remaining\n", Cyan, NC)
		fmt.Printf("  %s[Q]uit%s - Stop and apply changes made so far\n", Red, NC)
		fmt.Println()

		fmt.Print("  Choice [E/A/M/S/D/Q]: ")
		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(strings.ToUpper(input))

		switch choice {
		case "E":
			fmt.Println()
			fmt.Print("  New author name [" + cfg.NewName + "]: ")
			input, _ := reader.ReadString('\n')
			customName := strings.TrimSpace(input)
			if customName == "" {
				customName = cfg.NewName
			}

			fmt.Print("  New author email [" + cfg.NewEmail + "]: ")
			input, _ = reader.ReadString('\n')
			customEmail := strings.TrimSpace(input)
			if customEmail == "" {
				customEmail = cfg.NewEmail
			}

			cfg.CommitOverrides[hash] = config.CommitOverride{
				Name:  customName,
				Email: customEmail,
			}
			cfg.Logger.Success("Custom name/email saved for commit " + commitHash[:12])
			fmt.Println()
			return ""
		case "M":
			fmt.Println()
			fmt.Printf("  %sCurrent message:%s\n", Cyan, NC)
			fmt.Printf("    %s\n", getCommitSubject(cfg.TempDir, commitHash))
			fmt.Println()
			fmt.Printf("  %sEnter new commit message (or press Enter to keep current):%s\n", Cyan, NC)
			input, _ := reader.ReadString('\n')
			customMessage := strings.TrimSpace(input)
			if customMessage != "" {
				cfg.CommitMessages[hash] = customMessage
				cfg.Logger.Success("Custom message saved for commit " + commitHash[:12])
			} else {
				cfg.Logger.Info("Keeping original message for commit " + commitHash[:12])
			}
			fmt.Println()
			return ""
		case "A":
			cfg.CommitOverrides[hash] = config.CommitOverride{
				Name:  cfg.NewName,
				Email: cfg.NewEmail,
			}
			cfg.Logger.Success("Changes will be applied to commit " + commitHash[:12])
			return ""
		case "S":
			cfg.CommitOverrides[hash] = config.CommitOverride{Skip: true}
			cfg.Logger.Info("Commit " + commitHash[:12] + " will be kept unchanged")
			return ""
		case "D":
			cfg.Logger.Info("Applying default changes to all remaining commits...")
			cfg.CommitOverrides[hash] = config.CommitOverride{
				Name:  cfg.NewName,
				Email: cfg.NewEmail,
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

func getCommitSubject(tmpDir string, hash string) string {
	cmd := exec.Command("git", "log", "-1", "--format=%s", hash)
	cmd.Dir = tmpDir + "/working"
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output))
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
