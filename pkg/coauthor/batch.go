package coauthor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/util"
)

const (
	Red    = "\033[1;31m"
	Green  = "\033[1;32m"
	Yellow = "\033[1;33m"
	Blue   = "\033[0;34m"
	Cyan   = "\033[0;36m"
	Bold   = "\033[1m"
	NC     = "\033[0m"
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
		"--msg-filter", "bash "+coauthorMsgFilter,
		"--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.TempDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	log.Success("Co-authored-by trailers updated")
	return nil
}

func RewriteInteractive(cfg *config.Config) error {
	if cfg.CoauthorMode != "interactive" {
		return nil
	}
	if len(cfg.CoauthorActions) == 0 {
		return nil
	}

	log := cfg.Logger
	log.Header("Applying Co-authored-by Interactive Decisions")

	coauthorMsgFilter := filepath.Join(cfg.TempDir, "coauthor_msg_filter.sh")
	if err := buildCoauthorMsgFilter(cfg, coauthorMsgFilter, "interactive"); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Applying %d Co-authored-by decision(s)...", len(cfg.CoauthorActions)))
	cmd := exec.Command("git", "filter-branch", "-f",
		"--msg-filter", "bash "+coauthorMsgFilter,
		"--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.TempDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	log.Success("Co-authored-by trailers updated (interactive)")
	return nil
}

func buildCoauthorMsgFilter(cfg *config.Config, outputPath string, modeType string) error {
	var script strings.Builder

	script.WriteString("#!/usr/bin/env bash\n")
	script.WriteString("input=$(cat)\n")

	sedEscapedName := escapeSed(cfg.CoauthorOldName)
	var sedMatchPattern string
	if cfg.CoauthorOldEmail != "" {
		sedEscapedEmail := escapeSed(cfg.CoauthorOldEmail)
		sedMatchPattern = "Co-authored-by: " + sedEscapedName + " <" + sedEscapedEmail + ">"
	} else {
		sedMatchPattern = "Co-authored-by: " + sedEscapedName + " <[^>]*>"
	}

	sedEscapedNewName := escapeSed(cfg.CoauthorNewName)
	sedEscapedNewEmail := escapeSed(cfg.CoauthorNewEmail)
	newTrailer := "Co-authored-by: " + sedEscapedNewName + " <" + sedEscapedNewEmail + ">"

	if modeType == "batch" {
		if cfg.CoauthorMode == "remove" {
			fmt.Fprintf(&script, "printf '%%s' \"$input\" | sed -E '/^%s$/d'\n", sedMatchPattern)
		} else {
			fmt.Fprintf(&script, "printf '%%s' \"$input\" | sed -E 's/^%s$/%s/'\n", sedMatchPattern, newTrailer)
		}
	} else {
		script.WriteString("case \"$GIT_COMMIT\" in\n")
		for hash, action := range cfg.CoauthorActions {
			switch action {
			case "remove":
				fmt.Fprintf(&script, "    %s)\n", hash.String())
				fmt.Fprintf(&script, "        printf '%%s' \"$input\" | sed -E '/^%s$/d'\n", sedMatchPattern)
				script.WriteString("        ;;\n")
			case "rewrite":
				fmt.Fprintf(&script, "    %s)\n", hash.String())
				fmt.Fprintf(&script, "        printf '%%s' \"$input\" | sed -E 's/^%s$/%s/'\n", sedMatchPattern, newTrailer)
				script.WriteString("        ;;\n")
			default:
				fmt.Fprintf(&script, "    %s)\n", hash.String())
				script.WriteString("        printf '%s' \"$input\"\n")
				script.WriteString("        ;;\n")
			}
		}
		script.WriteString("    *)\n")
		script.WriteString("        printf '%s' \"$input\"\n")
		script.WriteString("        ;;\nesac\n")
	}

	if err := os.WriteFile(outputPath, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write coauthor filter script: %w", err)
	}

	return nil
}

func escapeSed(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "/", "\\/")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	s = strings.ReplaceAll(s, ".", "\\.")
	return s
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
	cmd.Dir = cfg.TempDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		log.Warning("Failed to get commit list")
		return
	}

	var matchingCommits []string
	lines := util.SplitLines(string(output))
	for _, commitHash := range lines {
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

	current := 0
	for _, commitHash := range matchingCommits {
		current++
		authorName, authorDate, subject, body := getCommitDetails(cfg.TempDir, commitHash)
		matchingLines := extractMatchingLines(body, cfg.CoauthorOldName, cfg.CoauthorOldEmail)

		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
		fmt.Printf("%sCo-author commit %d of %d%s\n", Bold, current, len(matchingCommits), NC)
		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", Bold, NC)
		fmt.Printf("  %sHash:%s    %s\n", Cyan, NC, commitHash[:12])
		fmt.Printf("  %sAuthor:%s  %s\n", Cyan, NC, authorName)
		fmt.Printf("  %sDate:%s    %s\n", Cyan, NC, authorDate)
		fmt.Printf("  %sSubject:%s %s\n", Cyan, NC, subject)
		fmt.Printf("  %sMatching trailer(s):%s\n", Cyan, NC)
		for _, line := range matchingLines {
			fmt.Printf("    %s%s%s\n", Yellow, line, NC)
		}
		fmt.Println()

		if action, ok := cfg.CoauthorActions[plumbing.NewHash(commitHash)]; ok {
			fmt.Printf("  %sCurrent decision:%s %s%s%s\n", Cyan, NC, Bold, action, NC)
			fmt.Println()
		}

		choice := promptLoop(commitHash, cfg)
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
	lines := util.SplitLines(body)
	for _, line := range lines {
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

func promptLoop(commitHash string, cfg *config.Config) string {
	for {
		fmt.Printf("  %s[R]ewrite%s - Replace trailer with new Co-authored-by\n", Green, NC)
		fmt.Printf("  %s[D]elete%s  - Remove the matching trailer line(s)\n", Red, NC)
		fmt.Printf("  %s[S]kip%s   - Leave this commit unchanged\n", Blue, NC)
		fmt.Printf("  %s[RA]%s     - Rewrite all remaining commits\n", Cyan, NC)
		fmt.Printf("  %s[DA]%s     - Delete trailers in all remaining commits\n", Cyan, NC)
		fmt.Printf("  %s[Q]uit%s   - Stop and apply decisions made so far\n", Red, NC)
		fmt.Println()

		var choice string
		fmt.Print("  Choice [R/D/S/RA/DA/Q]: ")
		if _, err := fmt.Scanln(&choice); err != nil {
			choice = ""
		}

		hash := plumbing.NewHash(commitHash)

		switch strings.ToUpper(choice) {
		case "R":
			cfg.CoauthorActions[hash] = "rewrite"
			cfg.Logger.Success("Will rewrite Co-authored-by trailer in commit " + commitHash[:12])
			return ""
		case "D":
			cfg.CoauthorActions[hash] = "remove"
			cfg.Logger.Success("Will remove Co-authored-by trailer from commit " + commitHash[:12])
			return ""
		case "S":
			cfg.CoauthorActions[hash] = "skip"
			cfg.Logger.Info("Commit " + commitHash[:12] + " will be left unchanged")
			return ""
		case "RA":
			cfg.CoauthorActions[hash] = "rewrite"
			cfg.Logger.Info("Will rewrite Co-authored-by trailers in all remaining commits")
			return "quit"
		case "DA":
			cfg.CoauthorActions[hash] = "remove"
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
