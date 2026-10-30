package coauthor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/git"
	"git-reforge/pkg/shell"
	"git-reforge/pkg/ui"
)

func RewriteBatch(cfg *config.Config) error {
	if cfg.CoauthorMode == "" || cfg.CoauthorMode == "interactive" {
		return nil
	}

	log := cfg.Logger
	log.Header("Rewriting Co-authored-by Trailers")
	return rewriteBatchFilterRepo(cfg)
}

func RewriteInteractive(cfg *config.Config) error {
	if cfg.CoauthorMode != "interactive" || len(cfg.State.CoauthorActions) == 0 {
		return nil
	}

	log := cfg.Logger
	log.Header("Applying Co-authored-by Interactive Decisions")
	return rewriteInteractiveFilterRepo(cfg)
}

type CoauthorTarget struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func ParseCoauthorTargets(cfg *config.Config) []CoauthorTarget {
	var targets []CoauthorTarget
	from := cfg.CoauthorFrom
	if len(from) == 0 {
		switch {
		case len(cfg.OldRawAuthors) > 0:
			from = cfg.OldRawAuthors
		case len(cfg.OldNames) > 0:
			from = cfg.OldNames
		default:
			from = cfg.OldEmails
		}
	}
	for _, raw := range from {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if name, email, err := config.ParseAuthor(raw); err == nil && email != "" {
			targets = append(targets, CoauthorTarget{Name: name, Email: email})
		} else {
			targets = append(targets, CoauthorTarget{Name: raw, Email: ""})
		}
	}
	return targets
}

func rewriteBatchFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	targets := ParseCoauthorTargets(cfg)
	if len(targets) == 0 {
		return nil
	}

	var lines []string
	switch cfg.CoauthorMode {
	case "remove":
		for _, t := range targets {
			pattern := CoauthorRegexPattern(t)
			lines = append(lines, "regex:(?m)"+pattern+"(\\r?\\n)?==>")
		}
	case "rewrite":
		replacement := "Co-authored-by: " + cfg.CoauthorNewName + " <" + cfg.CoauthorNewEmail + ">"
		for _, t := range targets {
			pattern := CoauthorRegexPattern(t)
			lines = append(lines, "regex:(?m)"+pattern+"==>"+replacement)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	replacePath := filepath.Join(cfg.TempDir, "replace_coauthor.txt")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(replacePath, []byte(content), 0644); err != nil {
		return err
	}

	cmd := exec.Command("git", "filter-repo", "--force", "--replace-message", replacePath)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo coauthor batch"); err != nil {
		return err
	}
	log.Success("Co-authored-by trailers updated")
	return nil
}

func CoauthorRegexPattern(t CoauthorTarget) string {
	switch {
	case t.Name != "" && t.Email != "":
		return `^Co-authored-by:\s*` + regexp.QuoteMeta(t.Name) + `\s*<` + regexp.QuoteMeta(t.Email) + `>`
	case t.Email != "":
		return `^Co-authored-by:[^<\r\n]*<` + regexp.QuoteMeta(t.Email) + `>`
	default:
		return `^Co-authored-by:\s*` + regexp.QuoteMeta(t.Name) + `(\s*<[^>\r\n]*>)?`
	}
}

func rewriteInteractiveFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	targets := ParseCoauthorTargets(cfg)
	dataFile := filepath.Join(cfg.TempDir, "filter_coauthor_interactive.json")
	data := map[string]any{
		"actions":   cfg.State.CoauthorActions,
		"targets":   targets,
		"new_name":  cfg.CoauthorNewName,
		"new_email": cfg.CoauthorNewEmail,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return err
	}

	pyCallback := fmt.Sprintf(`import re, json
with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

actions = cfg.get('actions', {})
targets = cfg.get('targets', [])
new_name = cfg.get('new_name', '')
new_email = cfg.get('new_email', '')

patterns = []
for t in targets:
    if t.get('name') and t.get('email'):
        patterns.append(r"(?m)^Co-authored-by: " + re.escape(t['name']) + r" <" + re.escape(t['email']) + r">\r?$\n?")
    elif t.get('email'):
        patterns.append(r"(?m)^Co-authored-by: [^<\r\n]*<" + re.escape(t['email']) + r">\r?$\n?")
    else:
        patterns.append(r"(?m)^Co-authored-by: " + re.escape(t['name']) + r" <[^>\r\n]*>\r?$\n?")

replacement = ("Co-authored-by: " + new_name + " <" + new_email + ">\n").encode('utf-8')

cid = commit.original_id.decode('ascii', errors='ignore')
if cid in actions:
    act = actions[cid]
    if act == "remove":
        for p in patterns:
            commit.message = re.sub(p.encode('utf-8'), b"", commit.message)
        if commit.message:
            commit.message = commit.message.rstrip(b'\n') + b'\n'
    elif act == "rewrite":
        for p in patterns:
            commit.message = re.sub(p.encode('utf-8'), replacement, commit.message)
`, dataFile)

	cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", pyCallback)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo coauthor interactive"); err != nil {
		return err
	}
	log.Success("Co-authored-by trailers updated (interactive)")
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

	targets := ParseCoauthorTargets(cfg)
	if len(targets) == 0 {
		log.Warning("No Co-authored-by targets configured")
		return
	}

	log.Info("Scanning all commits for matching Co-authored-by trailers...")

	var matchingCommits []string
	commitDetailsCache := make(map[string]git.CommitInfo)
	seen := make(map[string]bool)

	for _, t := range targets {
		var pattern string
		switch {
		case t.Name != "" && t.Email != "":
			pattern = "Co-authored-by: " + t.Name + " <" + t.Email + ">"
		case t.Email != "":
			pattern = "<" + t.Email + ">"
		default:
			pattern = "Co-authored-by: " + t.Name
		}

		cmd := exec.Command("git", "log", "--all", "-F", "--grep="+pattern, "--format=%H")
		cmd.Dir = cfg.WorkDir()
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}

		for commitHash := range strings.SplitSeq(string(output), "\n") {
			commitHash = strings.TrimSpace(commitHash)
			if commitHash == "" || seen[commitHash] {
				continue
			}
			info, err := git.CommitDetails(cfg.WorkDir(), commitHash)
			if err == nil && matchesAnyCoauthor(info.Body, targets) {
				matchingCommits = append(matchingCommits, commitHash)
				commitDetailsCache[commitHash] = info
				seen[commitHash] = true
			}
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
		info := commitDetailsCache[commitHash]
		matchingLines := extractMatchingLinesForTargets(info.Body, targets)

		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", ui.Bold, ui.NC)
		fmt.Printf("%sCo-author commit %d of %d%s\n", ui.Bold, current, len(matchingCommits), ui.NC)
		fmt.Printf("%s────────────────────────────────────────────────────────────────%s\n", ui.Bold, ui.NC)
		fmt.Printf("  %sHash:%s    %s\n", ui.Cyan, ui.NC, commitHash[:12])
		fmt.Printf("  %sAuthor:%s  %s\n", ui.Cyan, ui.NC, info.AuthorName)
		fmt.Printf("  %sDate:%s    %s\n", ui.Cyan, ui.NC, info.AuthorDate)
		fmt.Printf("  %sSubject:%s %s\n", ui.Cyan, ui.NC, info.Subject)
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

func matchesAnyCoauthor(body string, targets []CoauthorTarget) bool {
	return len(extractMatchingLinesForTargets(body, targets)) > 0
}

func extractMatchingLinesForTargets(body string, targets []CoauthorTarget) []string {
	var matches []string
	for line := range strings.SplitSeq(body, "\n") {
		if !strings.HasPrefix(line, "Co-authored-by:") {
			continue
		}
		for _, t := range targets {
			var matched bool
			switch {
			case t.Name != "" && t.Email != "":
				matched = strings.Contains(line, "Co-authored-by: "+t.Name+" <"+t.Email+">")
			case t.Email != "":
				matched = strings.Contains(line, "<"+t.Email+">")
			default:
				matched = strings.HasPrefix(line, "Co-authored-by: "+t.Name+" <") || line == "Co-authored-by: "+t.Name
			}
			if matched {
				matches = append(matches, line)
				break
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
