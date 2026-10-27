package rewrite

import (
	"encoding/json"
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

	if cfg.NewName != "" {
		if len(cfg.OldNames) == 1 {
			log.Info(fmt.Sprintf("Rewriting commits from '%s' to '%s' <%s>", cfg.OldNames[0], cfg.NewName, cfg.NewEmail))
		} else {
			log.Info(fmt.Sprintf("Rewriting commits from %d authors to '%s' <%s>", len(cfg.OldNames), cfg.NewName, cfg.NewEmail))
			for _, name := range cfg.OldNames {
				log.Info("  - " + name)
			}
		}
	} else {
		log.Info(fmt.Sprintf("Rewriting commit emails to <%s> for author(s): %s", cfg.NewEmail, strings.Join(cfg.OldNames, ", ")))
	}

	if shell.HasFilterRepo() {
		return rewriteBatchFilterRepo(cfg)
	}

	filterScript := filepath.Join(cfg.TempDir, "batch_filter.sh")

	var patterns []string
	for _, name := range cfg.OldNames {
		patterns = append(patterns, shell.QuoteSingle(name))
	}
	patternList := strings.Join(patterns, "|")

	// Build the filter script using portable POSIX sh
	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\n")
	fmt.Fprintf(&script, `case "$GIT_AUTHOR_NAME" in
    %s)
        if [ -n "$FILTER_NEW_NAME" ]; then
            export GIT_AUTHOR_NAME="$FILTER_NEW_NAME"
        fi
        export GIT_AUTHOR_EMAIL="$FILTER_NEW_EMAIL"
        ;;
esac

case "$GIT_COMMITTER_NAME" in
    %s)
        if [ -n "$FILTER_NEW_NAME" ]; then
            export GIT_COMMITTER_NAME="$FILTER_NEW_NAME"
        fi
        export GIT_COMMITTER_EMAIL="$FILTER_NEW_EMAIL"
        ;;
esac
`, patternList, patternList)

	if err := os.WriteFile(filterScript, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write filter script: %w", err)
	}

	cmd := exec.Command("git", "filter-branch", "-f", "--env-filter", ". "+shell.QuoteSingle(filterScript), "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	cmd.Env = append(os.Environ(),
		"FILTER_NEW_NAME="+cfg.NewName,
		"FILTER_NEW_EMAIL="+cfg.NewEmail,
	)
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
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

	if shell.HasFilterRepo() {
		return rewriteInteractiveFilterRepo(cfg)
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
		args = append(args, "--env-filter", ". "+shell.QuoteSingle(envFilterScript))
	}

	if len(cfg.State.CommitMessages) > 0 {
		args = append(args, "--msg-filter", ". "+shell.QuoteSingle(msgFilterScript))
	}

	args = append(args, "--tag-name-filter", "cat", "--", "--all")

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
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
			fmt.Fprintf(&script, "    %s)\n        ;;\n", hash)
		} else {
			fmt.Fprintf(&script, "    %s)\n", hash)
			if override.Name != "" {
				fmt.Fprintf(&script, "        export GIT_AUTHOR_NAME=\"%s\"\n", shell.EscapeDoubleQuoted(override.Name))
				fmt.Fprintf(&script, "        export GIT_COMMITTER_NAME=\"%s\"\n", shell.EscapeDoubleQuoted(override.Name))
			}
			fmt.Fprintf(&script, "        export GIT_AUTHOR_EMAIL=\"%s\"\n", shell.EscapeDoubleQuoted(override.Email))
			fmt.Fprintf(&script, "        export GIT_COMMITTER_EMAIL=\"%s\"\n", shell.EscapeDoubleQuoted(override.Email))
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
	script.WriteString("#!/usr/bin/env sh\n")

	if len(cfg.State.CommitMessages) > 0 {
		script.WriteString("case \"$GIT_COMMIT\" in\n")
		for hash, msg := range cfg.State.CommitMessages {
			if !strings.HasSuffix(msg, "\n") {
				msg += "\n"
			}
			// Write message to sidecar file
			msgFile := filepath.Join(cfg.TempDir, "msg_"+hash+".txt")
			if err := os.WriteFile(msgFile, []byte(msg), 0644); err != nil {
				return "", fmt.Errorf("failed to write message file: %w", err)
			}
			fmt.Fprintf(&script, "    %s)\n        cat %s\n        ;;\n", hash, shell.QuoteSingle(msgFile))
		}
		script.WriteString("    *)\n        cat\n        ;;\nesac\n")
	} else {
		script.WriteString("cat\n")
	}

	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write message filter script: %w", err)
	}

	return scriptPath, nil
}

func rewriteBatchFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	dataFile := filepath.Join(cfg.TempDir, "filter_batch.json")
	data := map[string]interface{}{
		"old_names": cfg.OldNames,
		"new_name":  cfg.NewName,
		"new_email": cfg.NewEmail,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return err
	}

	pyCallback := fmt.Sprintf(`import json
with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

old_names = {n.encode('utf-8') for n in cfg['old_names']}
new_name = cfg['new_name'].encode('utf-8')
new_email = cfg['new_email'].encode('utf-8')

if commit.author_name in old_names:
    if new_name:
        commit.author_name = new_name
    commit.author_email = new_email

if commit.committer_name in old_names:
    if new_name:
        commit.committer_name = new_name
    commit.committer_email = new_email
`, dataFile)

	cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", pyCallback)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo batch"); err != nil {
		return err
	}
	log.Success("Commit history rewritten successfully")
	return nil
}

func rewriteInteractiveFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	dataFile := filepath.Join(cfg.TempDir, "filter_interactive.json")
	data := map[string]interface{}{
		"overrides": cfg.State.CommitOverrides,
		"messages":  cfg.State.CommitMessages,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return err
	}

	pyCallback := fmt.Sprintf(`import json
with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

overrides = cfg.get('overrides', {})
messages = cfg.get('messages', {})

cid = commit.original_id.decode('ascii', errors='ignore')
if cid in overrides:
    ov = overrides[cid]
    if not ov.get('Skip'):
        name = ov.get('Name')
        if name:
            commit.author_name = name.encode('utf-8')
            commit.committer_name = name.encode('utf-8')
        email = ov.get('Email')
        if email:
            commit.author_email = email.encode('utf-8')
            commit.committer_email = email.encode('utf-8')

if cid in messages:
    msg = messages[cid]
    if not msg.endswith('\n'):
        msg += '\n'
    commit.message = msg.encode('utf-8')
`, dataFile)

	cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", pyCallback)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo interactive"); err != nil {
		return err
	}
	log.Success("Commit history rewritten successfully")
	return nil
}
