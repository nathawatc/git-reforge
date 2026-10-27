package coauthor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/shell"
	"github-to-codeberg/pkg/ui"
)

func RewriteBatch(cfg *config.Config) error {
	if cfg.CoauthorMode == "" || cfg.CoauthorMode == "interactive" {
		return nil
	}

	log := cfg.Logger
	log.Header("Rewriting Co-authored-by Trailers")

	coauthorMsgFilter := filepath.Join(cfg.TempDir, "coauthor_msg_filter.sh")
	if err := buildCoauthorMsgFilter(cfg, coauthorMsgFilter, "batch"); err != nil {
		return err
	}

	log.Info("Applying Co-authored-by " + cfg.CoauthorMode + " to all commits...")
	cmd := exec.Command("git", "filter-branch", "-f",
		"--msg-filter", "bash "+shell.QuoteSingle(coauthorMsgFilter),
		"--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	log.Success("Co-authored-by trailers updated")
	return nil
}

func RewriteInteractive(cfg *config.Config) error {
	if cfg.CoauthorMode != "interactive" {
		return nil
	}
	if len(cfg.State.CoauthorActions) == 0 {
		return nil
	}

	log := cfg.Logger
	log.Header("Applying Co-authored-by Interactive Decisions")

	coauthorMsgFilter := filepath.Join(cfg.TempDir, "coauthor_msg_filter.sh")
	if err := buildCoauthorMsgFilter(cfg, coauthorMsgFilter, "interactive"); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Applying %d Co-authored-by decision(s)...", len(cfg.State.CoauthorActions)))
	cmd := exec.Command("git", "filter-branch", "-f",
		"--msg-filter", "bash "+shell.QuoteSingle(coauthorMsgFilter),
		"--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	log.Success("Co-authored-by trailers updated (interactive)")
	return nil
}

func buildCoauthorMsgFilter(cfg *config.Config, outputPath string, modeType string) error {
	if cfg.CoauthorOldName == "" {
		return fmt.Errorf("coauthor-old-name is required for coauthor operations")
	}

	var script strings.Builder

	script.WriteString("#!/usr/bin/env sh\n")

	sedEscapedName := shell.EscapeSed(cfg.CoauthorOldName)
	var sedMatchPattern string
	if cfg.CoauthorOldEmail != "" {
		sedEscapedEmail := shell.EscapeSed(cfg.CoauthorOldEmail)
		sedMatchPattern = "Co-authored-by: " + sedEscapedName + " <" + sedEscapedEmail + ">"
	} else {
		sedMatchPattern = "Co-authored-by: " + sedEscapedName + " <[^>]*>"
	}

	sedEscapedNewName := shell.EscapeSedReplacement(cfg.CoauthorNewName)
	sedEscapedNewEmail := shell.EscapeSedReplacement(cfg.CoauthorNewEmail)
	newTrailer := "Co-authored-by: " + sedEscapedNewName + " <" + sedEscapedNewEmail + ">"

	if modeType == "batch" {
		if cfg.CoauthorMode == "remove" {
			fmt.Fprintf(&script, "sed -E %s\n", shell.QuoteSingle(fmt.Sprintf("/^%s$/d", sedMatchPattern)))
		} else {
			fmt.Fprintf(&script, "sed -E %s\n", shell.QuoteSingle(fmt.Sprintf("s/^%s$/%s/", sedMatchPattern, newTrailer)))
		}
	} else {
		script.WriteString("case \"$GIT_COMMIT\" in\n")
		for hash, action := range cfg.State.CoauthorActions {
			switch action {
			case "remove":
				fmt.Fprintf(&script, "    %s)\n", hash)
				fmt.Fprintf(&script, "        sed -E %s\n", shell.QuoteSingle(fmt.Sprintf("/^%s$/d", sedMatchPattern)))
				script.WriteString("        ;;\n")
			case "rewrite":
				fmt.Fprintf(&script, "    %s)\n", hash)
				fmt.Fprintf(&script, "        sed -E %s\n", shell.QuoteSingle(fmt.Sprintf("s/^%s$/%s/", sedMatchPattern, newTrailer)))
				script.WriteString("        ;;\n")
			default:
				fmt.Fprintf(&script, "    %s)\n", hash)
				fmt.Fprintf(&script, "        cat\n")
				script.WriteString("        ;;\n")
			}
		}
		script.WriteString("    *)\n")
		script.WriteString("        cat\n")
		script.WriteString("        ;;\nesac\n")
	}

	if err := os.WriteFile(outputPath, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write coauthor filter script: %w", err)
	}

	return nil
}

func RunCoauthorPhase(cfg *config.Config) error {
	if cfg.CoauthorMode == "" {
		return nil
	}

	switch cfg.CoauthorMode {
	case "rewrite", "remove":
		return RewriteBatch(cfg)
	case "interactive":
		InteractiveCoauthorEdit(cfg)
		return RewriteInteractive(cfg)
	}

	return nil
}

func InteractiveCoauthorEdit(cfg *config.Config) {
	log := cfg.Logger
	log.Header("Interactive Co-authored-by Editing")

	log.Info("Scanning all commits for Co-authored-by trailers matching: " + cfg.CoauthorOldName)

	// Get all commits and filter
	cmd := exec.Command("git", "log", "--all", "--format=%H")
	cmd.Dir = cfg.WorkDir()
	output, err := cmd.Output()
	if err != nil {
		log.Warning("Failed to get commit list")
		return
	}

	var matchingCommits []string
	for commitHash := range strings.SplitSeq(string(output), "\n") {
		if commitHash == "" {
			continue
		}
		body := getCommitBody(cfg.TempDir, commitHash)
		if matchesCoauthor(body, cfg.CoauthorOldName, cfg.CoauthorOldEmail) {
			matchingCommits = append(matchingCommits, commitHash)
		}
	}

	if len(matchingCommits) == 0 {
		log.Warning("No commits found containing matching Co-authored-by trailers")
		return
	}

	log.Info(fmt.Sprintf("Found %d commit(s) with matching Co-authored-by trailer(s)", len(matchingCommits)))
	fmt.Println()

	prompts := ui.NewPromptSession()
	defer func() {
		if err := prompts.Close(); err != nil {
			log.Warning(err.Error())
		}
	}()

	current := 0
	for _, commitHash := range matchingCommits {
		current++
		authorName, authorDate, subject, body := getCommitDetails(cfg.TempDir, commitHash)
		matchingLines := extractMatchingLines(body, cfg.CoauthorOldName, cfg.CoauthorOldEmail)

		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", ui.Bold, ui.NC)
		fmt.Printf("%sCo-author commit %d of %d%s\n", ui.Bold, current, len(matchingCommits), ui.NC)
		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", ui.Bold, ui.NC)
		fmt.Printf("  %sHash:%s    %s\n", ui.Cyan, ui.NC, commitHash[:12])
		fmt.Printf("  %sAuthor:%s  %s\n", ui.Cyan, ui.NC, authorName)
		fmt.Printf("  %sDate:%s    %s\n", ui.Cyan, ui.NC, authorDate)
		fmt.Printf("  %sSubject:%s %s\n", ui.Cyan, ui.NC, subject)
		fmt.Printf("  %sMatching trailer(s):%s\n", ui.Cyan, ui.NC)
		for _, line := range matchingLines {
			fmt.Printf("    %s%s%s\n", ui.Yellow, line, ui.NC)
		}
		fmt.Println()

		if action, ok := cfg.State.CoauthorActions[commitHash]; ok {
			fmt.Printf("  %sCurrent decision:%s %s%s%s\n", ui.Cyan, ui.NC, ui.Bold, action, ui.NC)
			fmt.Println()
		}

		choice := promptLoop(commitHash, matchingCommits[current-1:], cfg, prompts)
		if choice == "quit" {
			return
		}
		fmt.Println()
	}
}

func getCommitBody(tmpDir string, hash string) string {
	cmd := exec.Command("git", "log", "-1", "--format=%B", hash)
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func getCommitDetails(tmpDir string, hash string) (authorName, authorDate, subject, body string) {
	cmd := exec.Command("git", "log", "-1", "--format=%an", hash)
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err == nil {
		authorName = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("git", "log", "-1", "--format=%ai", hash)
	cmd.Dir = tmpDir + "/working"
	output, err = cmd.Output()
	if err == nil {
		authorDate = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("git", "log", "-1", "--format=%s", hash)
	cmd.Dir = tmpDir + "/working"
	output, err = cmd.Output()
	if err == nil {
		subject = strings.TrimSpace(string(output))
	}

	cmd = exec.Command("git", "log", "-1", "--format=%B", hash)
	cmd.Dir = tmpDir + "/working"
	output, err = cmd.Output()
	if err == nil {
		body = string(output)
	}

	return
}

func matchesCoauthor(body string, name string, email string) bool {
	pattern := "Co-authored-by: " + name
	if email != "" {
		pattern += " <" + email + ">"
	} else {
		pattern += " "
	}
	return strings.Contains(body, pattern)
}

func extractMatchingLines(body string, name string, email string) []string {
	var matches []string
	for line := range strings.SplitSeq(body, "\n") {
		if email != "" {
			if strings.Contains(line, "Co-authored-by: "+name+" <"+email+">") {
				matches = append(matches, line)
			}
		} else {
			if strings.HasPrefix(line, "Co-authored-by: "+name+" ") {
				matches = append(matches, line)
			}
		}
	}
	return matches
}

func promptLoop(commitHash string, remainingCommits []string, cfg *config.Config, prompts *ui.PromptSession) string {
	for {
		fmt.Printf("  %s[R]ewrite%s - Replace trailer with new Co-authored-by\n", ui.Green, ui.NC)
		fmt.Printf("  %s[D]elete%s  - Remove the matching trailer line(s)\n", ui.Red, ui.NC)
		fmt.Printf("  %s[S]kip%s   - Leave this commit unchanged\n", ui.Blue, ui.NC)
		fmt.Printf("  %s[RA]%s     - Rewrite all remaining commits\n", ui.Cyan, ui.NC)
		fmt.Printf("  %s[DA]%s     - Delete trailers in all remaining commits\n", ui.Cyan, ui.NC)
		fmt.Printf("  %s[Q]uit%s   - Stop and apply decisions made so far\n", ui.Red, ui.NC)
		fmt.Println()

		choice, err := prompts.ReadLine("  Choice [R/D/S/RA/DA/Q]: ")
		if err != nil {
			cfg.Logger.Warning("Failed to read coauthor choice: " + err.Error())
			return "quit"
		}

		switch strings.ToUpper(choice) {
		case "R":
			cfg.State.CoauthorActions[commitHash] = "rewrite"
			cfg.Logger.Success("Will rewrite Co-authored-by trailer in commit " + commitHash[:12])
			return ""
		case "D":
			cfg.State.CoauthorActions[commitHash] = "remove"
			cfg.Logger.Success("Will remove Co-authored-by trailer from commit " + commitHash[:12])
			return ""
		case "S":
			cfg.State.CoauthorActions[commitHash] = "skip"
			cfg.Logger.Info("Commit " + commitHash[:12] + " will be left unchanged")
			return ""
		case "RA":
			applyCoauthorAction(remainingCommits, "rewrite", cfg)
			cfg.Logger.Info("Will rewrite Co-authored-by trailers in all remaining commits")
			return "quit"
		case "DA":
			applyCoauthorAction(remainingCommits, "remove", cfg)
			cfg.Logger.Info("Will remove Co-authored-by trailers from all remaining commits")
			return "quit"
		case "Q":
			cfg.Logger.Info("Stopping Co-authored-by interactive mode. Decisions made so far will be applied.")
			return "quit"
		default:
			cfg.Logger.Warning("Invalid choice. Please select R, D, S, RA, DA, or Q.")
		}
	}
}

func applyCoauthorAction(commitHashes []string, action string, cfg *config.Config) {
	for _, commitHash := range commitHashes {
		if commitHash == "" {
			continue
		}
		cfg.State.CoauthorActions[commitHash] = action
	}
}
