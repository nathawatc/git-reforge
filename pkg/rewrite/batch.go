package rewrite

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github-to-codeberg/pkg/config"
)

func RewriteCommitsBatch(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History")

	if len(cfg.OldNames) == 1 {
		log.Info(fmt.Sprintf("Rewriting commits from '%s' to '%s' <%s>", cfg.OldNames[0], cfg.NewName, cfg.NewEmail))
	} else {
		log.Info(fmt.Sprintf("Rewriting commits from %d authors to '%s' <%s>", len(cfg.OldNames), cfg.NewName, cfg.NewEmail))
		for _, name := range cfg.OldNames {
			log.Info("  - " + name)
		}
	}

	filterScript := filepath.Join(cfg.TempDir, "batch_filter.sh")

	// Build the filter script
	script := `#!/usr/bin/env bash
OLD_NAMES_ARRAY=(
`
	for _, name := range cfg.OldNames {
		script += fmt.Sprintf("    %q\n", name)
	}
	script += `)

for old_name in "${OLD_NAMES_ARRAY[@]}"; do
    if [ "$GIT_AUTHOR_NAME" = "$old_name" ]; then
        export GIT_AUTHOR_NAME="` + EscapeShell(cfg.NewName) + `"
        export GIT_AUTHOR_EMAIL="` + EscapeShell(cfg.NewEmail) + `"
        break
    fi
done

for old_name in "${OLD_NAMES_ARRAY[@]}"; do
    if [ "$GIT_COMMITTER_NAME" = "$old_name" ]; then
        export GIT_COMMITTER_NAME="` + EscapeShell(cfg.NewName) + `"
        export GIT_COMMITTER_EMAIL="` + EscapeShell(cfg.NewEmail) + `"
        break
    fi
done
`

	if err := os.WriteFile(filterScript, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write filter script: %w", err)
	}

	cmd := exec.Command("git", "filter-branch", "-f", "--env-filter", "source "+filterScript, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.TempDir + "/working"
	cmd.Env = append(os.Environ(),
		"FILTER_NEW_NAME="+cfg.NewName,
		"FILTER_NEW_EMAIL="+cfg.NewEmail,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	log.Success("Commit history rewritten successfully")
	return nil
}

func RewriteCommitsInteractive(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History (Interactive)")

	if len(cfg.CommitOverrides) == 0 && len(cfg.CommitMessages) == 0 {
		log.Warning("No commits selected for modification")
		return nil
	}

	total := len(cfg.CommitOverrides) + len(cfg.CommitMessages)
	log.Info(fmt.Sprintf("Applying %d commit modification(s)...", total))
	if len(cfg.CommitOverrides) > 0 {
		log.Info(fmt.Sprintf("  - %d author/email change(s)", len(cfg.CommitOverrides)))
	}
	if len(cfg.CommitMessages) > 0 {
		log.Info(fmt.Sprintf("  - %d message change(s)", len(cfg.CommitMessages)))
	}

	msgFilterScript, err := generateMessageFilter(cfg)
	if err != nil {
		return err
	}

	args := []string{"git", "filter-branch", "-f"}

	if len(cfg.CommitOverrides) > 0 {
		envFilterScript, err := generateInteractiveFilter(cfg)
		if err != nil {
			return err
		}
		args = append(args, "--env-filter", "source "+envFilterScript)
	}

	if len(cfg.CommitMessages) > 0 {
		args = append(args, "--msg-filter", "source "+msgFilterScript)
	}

	args = append(args, "--tag-name-filter", "cat", "--", "--all")

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cfg.TempDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	log.Success("Commit history rewritten successfully")
	return nil
}

func generateInteractiveFilter(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.TempDir, "filter.sh")

	script := "case \"$GIT_COMMIT\" in\n"
	for hash, override := range cfg.CommitOverrides {
		if override.Skip {
			script += fmt.Sprintf("    %s)\n        ;;\n", hash.String())
		} else {
			script += fmt.Sprintf("    %s)\n", hash.String())
			script += fmt.Sprintf("        export GIT_AUTHOR_NAME=\"%s\"\n", EscapeShell(override.Name))
			script += fmt.Sprintf("        export GIT_AUTHOR_EMAIL=\"%s\"\n", EscapeShell(override.Email))
			script += fmt.Sprintf("        export GIT_COMMITTER_NAME=\"%s\"\n", EscapeShell(override.Name))
			script += fmt.Sprintf("        export GIT_COMMITTER_EMAIL=\"%s\"\n", EscapeShell(override.Email))
			script += "        ;;\n"
		}
	}
	script += "    *)\n        ;;\nesac\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", fmt.Errorf("failed to write filter script: %w", err)
	}

	return scriptPath, nil
}

func generateMessageFilter(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.TempDir, "msg_filter.sh")

	var script bytes.Buffer
	script.WriteString("#!/usr/bin/env bash\n")
	script.WriteString("original_msg=$(cat)\n")

	if len(cfg.CommitMessages) > 0 {
		script.WriteString("case \"$GIT_COMMIT\" in\n")
		for hash, msg := range cfg.CommitMessages {
			// Write message to sidecar file
			msgFile := filepath.Join(cfg.TempDir, "msg_"+hash.String()+".txt")
			if err := os.WriteFile(msgFile, []byte(msg), 0644); err != nil {
				return "", fmt.Errorf("failed to write message file: %w", err)
			}
			script.WriteString(fmt.Sprintf("    %s)\n        cat \"%s\"\n        ;;\n", hash.String(), msgFile))
		}
		script.WriteString("    *)\n        printf '%s' \"$original_msg\"\n        ;;\nesac\n")
	} else {
		script.WriteString("printf '%s' \"$original_msg\"\n")
	}

	if err := os.WriteFile(scriptPath, script.Bytes(), 0755); err != nil {
		return "", fmt.Errorf("failed to write message filter script: %w", err)
	}

	return scriptPath, nil
}

func EscapeShell(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "'", "'\\''")
	return s
}
