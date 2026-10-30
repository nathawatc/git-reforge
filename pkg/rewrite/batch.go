package rewrite

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/shell"
)

func RewriteCommitsBatch(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History")

	displayedOld := cfg.OldRawAuthors
	if len(displayedOld) == 0 {
		displayedOld = append([]string{}, cfg.OldNames...)
		for _, e := range cfg.OldEmails {
			if !slices.Contains(displayedOld, e) {
				displayedOld = append(displayedOld, e)
			}
		}
	}

	if cfg.NewName != "" {
		if len(displayedOld) == 1 {
			log.Info(fmt.Sprintf("Rewriting commits from '%s' to '%s' <%s>", displayedOld[0], cfg.NewName, cfg.NewEmail))
		} else {
			log.Info(fmt.Sprintf("Rewriting commits from %d authors to '%s' <%s>", len(displayedOld), cfg.NewName, cfg.NewEmail))
			for _, name := range displayedOld {
				log.Info("  - " + name)
			}
		}
	} else {
		log.Info(fmt.Sprintf("Rewriting commit emails to <%s> for author(s): %s", cfg.NewEmail, strings.Join(displayedOld, ", ")))
	}

	if shell.HasFilterRepo() {
		return rewriteBatchFilterRepo(cfg)
	}

	filterScript := filepath.Join(cfg.TempDir, "batch_filter.sh")

	var namePatterns []string
	for _, name := range cfg.OldNames {
		if name != "" {
			namePatterns = append(namePatterns, shell.QuoteSingle(name))
		}
	}
	namePatternList := strings.Join(namePatterns, "|")

	var emailPatterns []string
	for _, email := range cfg.OldEmails {
		if email != "" {
			emailPatterns = append(emailPatterns, shell.QuoteSingle(email))
		}
	}
	emailPatternList := strings.Join(emailPatterns, "|")

	// Build the filter script using portable POSIX sh
	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\nmatch_author=0\n")
	if namePatternList != "" {
		fmt.Fprintf(&script, `case "$GIT_AUTHOR_NAME" in
    %s) match_author=1 ;;
esac
`, namePatternList)
	}
	if emailPatternList != "" {
		fmt.Fprintf(&script, `case "$GIT_AUTHOR_EMAIL" in
    %s) match_author=1 ;;
esac
`, emailPatternList)
	}
	script.WriteString(`if [ "$match_author" -eq 1 ]; then
    if [ -n "$FILTER_NEW_NAME" ]; then
        export GIT_AUTHOR_NAME="$FILTER_NEW_NAME"
    fi
    export GIT_AUTHOR_EMAIL="$FILTER_NEW_EMAIL"
fi

match_committer=0
`)
	if namePatternList != "" {
		fmt.Fprintf(&script, `case "$GIT_COMMITTER_NAME" in
    %s) match_committer=1 ;;
esac
`, namePatternList)
	}
	if emailPatternList != "" {
		fmt.Fprintf(&script, `case "$GIT_COMMITTER_EMAIL" in
    %s) match_committer=1 ;;
esac
`, emailPatternList)
	}
	script.WriteString(`if [ "$match_committer" -eq 1 ]; then
    if [ -n "$FILTER_NEW_NAME" ]; then
        export GIT_COMMITTER_NAME="$FILTER_NEW_NAME"
    fi
    export GIT_COMMITTER_EMAIL="$FILTER_NEW_EMAIL"
fi
`)

	if err := os.WriteFile(filterScript, []byte(script.String()), 0755); err != nil {
		return fmt.Errorf("failed to write filter script: %w", err)
	}

	args := []string{"git", "filter-branch", "-f", "--env-filter", ". " + shell.QuoteSingle(filterScript)}
	if len(cfg.StripTitlePatterns) > 0 || len(cfg.StripBodyPatterns) > 0 {
		msgFilterScript, err := generateBatchMessageFilter(cfg)
		if err != nil {
			return err
		}
		args = append(args, "--msg-filter", ". "+shell.QuoteSingle(msgFilterScript))
	}
	args = append(args, "--tag-name-filter", "cat", "--", "--all")

	cmd := exec.Command(args[0], args[1:]...)
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

	args := []string{"git", "filter-branch", "-f"}

	if len(cfg.State.CommitOverrides) > 0 {
		envFilterScript, err := generateInteractiveFilter(cfg)
		if err != nil {
			return err
		}
		args = append(args, "--env-filter", ". "+shell.QuoteSingle(envFilterScript))
	}

	if len(cfg.State.CommitMessages) > 0 || len(cfg.StripTitlePatterns) > 0 || len(cfg.StripBodyPatterns) > 0 {
		msgFilterScript, err := generateMessageFilter(cfg)
		if err != nil {
			return err
		}
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
	hasStrip := len(cfg.StripTitlePatterns) > 0 || len(cfg.StripBodyPatterns) > 0
	var filterPyScript string
	if hasStrip {
		filterScript, err := generateBatchMessageFilter(cfg)
		if err != nil {
			return "", err
		}
		filterPyScript = filterScript
	}

	scriptPath := filepath.Join(cfg.TempDir, "msg_filter.sh")

	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\ncase \"$GIT_COMMIT\" in\n")
	for hash, msg := range cfg.State.CommitMessages {
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		// Write message to sidecar file
		msgFile := filepath.Join(cfg.TempDir, "msg_"+hash+".txt")
		if err := os.WriteFile(msgFile, []byte(msg), 0644); err != nil {
			return "", fmt.Errorf("failed to write message file: %w", err)
		}
		if hasStrip {
			fmt.Fprintf(&script, "    %s)\n        cat %s | . %s\n        ;;\n", hash, shell.QuoteSingle(msgFile), shell.QuoteSingle(filterPyScript))
		} else {
			fmt.Fprintf(&script, "    %s)\n        cat %s\n        ;;\n", hash, shell.QuoteSingle(msgFile))
		}
	}
	if hasStrip {
		fmt.Fprintf(&script, "    *)\n        . %s\n        ;;\nesac\n", shell.QuoteSingle(filterPyScript))
	} else {
		script.WriteString("    *)\n        cat\n        ;;\nesac\n")
	}

	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write message filter script: %w", err)
	}

	return scriptPath, nil
}

func generateBatchMessageFilter(cfg *config.Config) (string, error) {
	stripTitle := cfg.StripTitlePatterns
	if stripTitle == nil {
		stripTitle = []string{}
	}
	stripBody := cfg.StripBodyPatterns
	if stripBody == nil {
		stripBody = []string{}
	}

	dataFile := filepath.Join(cfg.TempDir, "filter_strip_batch.json")
	data := map[string]interface{}{
		"strip_title": stripTitle,
		"strip_body":  stripBody,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return "", err
	}

	pyScript := filepath.Join(cfg.TempDir, "batch_msg_filter.py")
	scriptContent := fmt.Sprintf(`import sys, re, json
with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

strip_title = [re.compile(p.encode('utf-8')) for p in (cfg.get('strip_title') or [])]
strip_body = []
for p in (cfg.get('strip_body') or []):
    pat = p.encode('utf-8')
    if pat.endswith(b'$'):
        pat += rb'(?:\r?\n)?'
    strip_body.append(re.compile(pat, re.MULTILINE))

raw = sys.stdin.buffer.read()
parts = raw.split(b'\n', 1)
title = parts[0]
body = parts[1] if len(parts) > 1 else b''

for p in strip_title:
    title = p.sub(b'', title)
title = title.rstrip()

for p in strip_body:
    body = p.sub(b'', body)
body = re.sub(rb'\n{3,}', rb'\n\n', body).strip(b'\r\n')

if body:
    sys.stdout.buffer.write(title + b'\n\n' + body + b'\n')
else:
    sys.stdout.buffer.write(title + b'\n')
`, dataFile)
	if err := os.WriteFile(pyScript, []byte(scriptContent), 0755); err != nil {
		return "", err
	}

	shScript := filepath.Join(cfg.TempDir, "batch_msg_filter.sh")
	shContent := fmt.Sprintf("#!/usr/bin/env sh\npython3 %s\n", shell.QuoteSingle(pyScript))
	if err := os.WriteFile(shScript, []byte(shContent), 0755); err != nil {
		return "", err
	}

	return shScript, nil
}

func rewriteBatchFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	stripTitle := cfg.StripTitlePatterns
	if stripTitle == nil {
		stripTitle = []string{}
	}
	stripBody := cfg.StripBodyPatterns
	if stripBody == nil {
		stripBody = []string{}
	}

	dataFile := filepath.Join(cfg.TempDir, "filter_batch.json")
	data := map[string]interface{}{
		"old_names":   cfg.OldNames,
		"old_emails":  cfg.OldEmails,
		"new_name":    cfg.NewName,
		"new_email":   cfg.NewEmail,
		"strip_title": stripTitle,
		"strip_body":  stripBody,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return err
	}

	pyCallback := fmt.Sprintf(`import json, re
with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

old_names = {n.encode('utf-8') for n in (cfg.get('old_names') or []) if n}
old_emails = {e.encode('utf-8') for e in (cfg.get('old_emails') or []) if e}
new_name = cfg['new_name'].encode('utf-8')
new_email = cfg['new_email'].encode('utf-8')

match_author = (commit.author_name in old_names) or (commit.author_email in old_emails)
if match_author:
    if new_name:
        commit.author_name = new_name
    commit.author_email = new_email

match_committer = (commit.committer_name in old_names) or (commit.committer_email in old_emails)
if match_committer:
    if new_name:
        commit.committer_name = new_name
    commit.committer_email = new_email

strip_title = [re.compile(p.encode('utf-8')) for p in (cfg.get('strip_title') or [])]
strip_body = []
for p in (cfg.get('strip_body') or []):
    pat = p.encode('utf-8')
    if pat.endswith(b'$'):
        pat += rb'(?:\r?\n)?'
    strip_body.append(re.compile(pat, re.MULTILINE))

if strip_title or strip_body:
    parts = commit.message.split(b'\n', 1)
    title = parts[0]
    body = parts[1] if len(parts) > 1 else b''

    for p in strip_title:
        title = p.sub(b'', title)
    title = title.rstrip()

    for p in strip_body:
        body = p.sub(b'', body)
    body = re.sub(rb'\n{3,}', rb'\n\n', body).strip(b'\r\n')

    if body:
        commit.message = title + b'\n\n' + body + b'\n'
    else:
        commit.message = title + b'\n'
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
	stripTitle := cfg.StripTitlePatterns
	if stripTitle == nil {
		stripTitle = []string{}
	}
	stripBody := cfg.StripBodyPatterns
	if stripBody == nil {
		stripBody = []string{}
	}

	dataFile := filepath.Join(cfg.TempDir, "filter_interactive.json")
	data := map[string]interface{}{
		"overrides":   cfg.State.CommitOverrides,
		"messages":    cfg.State.CommitMessages,
		"strip_title": stripTitle,
		"strip_body":  stripBody,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return err
	}

	pyCallback := fmt.Sprintf(`import json, re
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

strip_title = [re.compile(p.encode('utf-8')) for p in (cfg.get('strip_title') or [])]
strip_body = []
for p in (cfg.get('strip_body') or []):
    pat = p.encode('utf-8')
    if pat.endswith(b'$'):
        pat += rb'(?:\r?\n)?'
    strip_body.append(re.compile(pat, re.MULTILINE))

if strip_title or strip_body:
    parts = commit.message.split(b'\n', 1)
    title = parts[0]
    body = parts[1] if len(parts) > 1 else b''

    for p in strip_title:
        title = p.sub(b'', title)
    title = title.rstrip()

    for p in strip_body:
        body = p.sub(b'', body)
    body = re.sub(rb'\n{3,}', rb'\n\n', body).strip(b'\r\n')

    if body:
        commit.message = title + b'\n\n' + body + b'\n'
    else:
        commit.message = title + b'\n'
`, dataFile)

	cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", pyCallback)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo interactive"); err != nil {
		return err
	}
	log.Success("Commit history rewritten successfully")
	return nil
}
