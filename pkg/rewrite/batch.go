package rewrite

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/shell"
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
	var script strings.Builder
	script.WriteString(`#!/usr/bin/env bash
`)
	for _, name := range cfg.OldNames {
		fmt.Fprintf(&script, "echo %q\n", name)
	}
	script.WriteString(` | while IFS= read -r old_name; do
    if [ "$GIT_AUTHOR_NAME" = "$old_name" ]; then
        export GIT_AUTHOR_NAME="` + shell.EscapeShell(cfg.NewName) + `"
        export GIT_AUTHOR_EMAIL="` + shell.EscapeShell(cfg.NewEmail) + `"
        break
    fi
done

echo "` + shell.EscapeShell(strings.Join(cfg.OldNames, "\n")) + `" | while IFS= read -r old_name; do
    if [ "$GIT_COMMITTER_NAME" = "$old_name" ]; then
        export GIT_COMMITTER_NAME="` + shell.EscapeShell(cfg.NewName) + `"
        export GIT_COMMITTER_EMAIL="` + shell.EscapeShell(cfg.NewEmail) + `"
        break
    fi
done
`)

	if err := os.WriteFile(filterScript, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write filter script: %w", err)
	}

	cmd := exec.Command("git", "filter-branch", "-f", "--env-filter", "source "+filterScript, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
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

	if len(cfg.State.CommitOverrides) == 0 && len(cfg.State.CommitMessages) == 0 {
		log.Warning("No commits selected for modification")
		return nil
	}

	total := len(cfg.State.CommitOverrides) + len(cfg.State.CommitMessages)
	log.Info(fmt.Sprintf("Applying %d commit modification(s)...", total))
	if len(cfg.State.CommitOverrides) > 0 {
		log.Info(fmt.Sprintf("  - %d author/email change(s)", len(cfg.State.CommitOverrides)))
	}
	if len(cfg.State.CommitMessages) > 0 {
		log.Info(fmt.Sprintf("  - %d message change(s)", len(cfg.State.CommitMessages)))
	}

	msgFilterScript, err := generateMessageFilter(cfg)
	if err != nil {
		return err
	}

	args := []string{"git", "filter-branch", "-f"}

	if len(cfg.State.CommitOverrides) > 0 {
		envFilterScript, err := generateInteractiveFilter(cfg)
		if err != nil {
			return err
		}
		args = append(args, "--env-filter", "source "+envFilterScript)
	}

	if len(cfg.State.CommitMessages) > 0 {
		args = append(args, "--msg-filter", "source "+msgFilterScript)
	}

	args = append(args, "--tag-name-filter", "cat", "--", "--all")

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	log.Success("Commit history rewritten successfully")
	return nil
}

func generateInteractiveFilter(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.TempDir, "filter.sh")

	var script strings.Builder
	script.WriteString("case \"$GIT_COMMIT\" in\n")
	for hash, override := range cfg.State.CommitOverrides {
		if override.Skip {
			fmt.Fprintf(&script, "    %s)\n        ;;\n", hash.String())
		} else {
			fmt.Fprintf(&script, "    %s)\n", hash.String())
			fmt.Fprintf(&script, "        export GIT_AUTHOR_NAME=\"%s\"\n", shell.EscapeShell(override.Name))
			fmt.Fprintf(&script, "        export GIT_AUTHOR_EMAIL=\"%s\"\n", shell.EscapeShell(override.Email))
			fmt.Fprintf(&script, "        export GIT_COMMITTER_NAME=\"%s\"\n", shell.EscapeShell(override.Name))
			fmt.Fprintf(&script, "        export GIT_COMMITTER_EMAIL=\"%s\"\n", shell.EscapeShell(override.Email))
			script.WriteString("        ;;\n")
		}
	}
	script.WriteString("    *)\n        ;;\nesac\n")

	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write filter script: %w", err)
	}

	return scriptPath, nil
}

func generateMessageFilter(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.TempDir, "msg_filter.sh")

	var script strings.Builder
	script.WriteString("#!/usr/bin/env bash\n")
	script.WriteString("original_msg=$(cat)\n")

	if len(cfg.State.CommitMessages) > 0 {
		script.WriteString("case \"$GIT_COMMIT\" in\n")
		for hash, msg := range cfg.State.CommitMessages {
			// Write message to sidecar file
			msgFile := filepath.Join(cfg.TempDir, "msg_"+hash.String()+".txt")
			if err := os.WriteFile(msgFile, []byte(msg), 0644); err != nil {
				return "", fmt.Errorf("failed to write message file: %w", err)
			}
			fmt.Fprintf(&script, "    %s)\n        cat \"%s\"\n        ;;\n", hash.String(), msgFile)
		}
		script.WriteString("    *)\n        printf '%s' \"$original_msg\"\n        ;;\nesac\n")
	} else {
		script.WriteString("printf '%s' \"$original_msg\"\n")
	}

	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write message filter script: %w", err)
	}

	return scriptPath, nil
}
