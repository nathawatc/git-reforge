package rewrite

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"git-reforge/pkg/coauthor"
	"git-reforge/pkg/config"
	"git-reforge/pkg/shell"
)

func RewriteCommitsBatch(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History")

	if !slices.Contains([]string{"keep", "none", "gpg", "ssh"}, cfg.SignMode) {
		return fmt.Errorf("invalid sign mode: %s", cfg.SignMode)
	}

	displayedOld := cfg.DisplayedOldAuthors()

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

	return rewriteBatchUnified(cfg)
}

func rewriteBatchUnified(cfg *config.Config) error {
	log := cfg.Logger

	switch cfg.SignMode {
	case "ssh":
		cmd := exec.Command("git", "config", "gpg.format", "ssh")
		cmd.Dir = cfg.WorkDir()
		if err := shell.RunCommand(cmd, "git config gpg.format"); err != nil {
			return err
		}
		defer func() {
			unsetCmd := exec.Command("git", "config", "--unset", "gpg.format")
			unsetCmd.Dir = cfg.WorkDir()
			_ = unsetCmd.Run()
		}()

		cmd = exec.Command("git", "config", "user.signingkey", cfg.SSHKey)
		cmd.Dir = cfg.WorkDir()
		if err := shell.RunCommand(cmd, "git config user.signingkey"); err != nil {
			return err
		}
		defer func() {
			unsetCmd := exec.Command("git", "config", "--unset", "user.signingkey")
			unsetCmd.Dir = cfg.WorkDir()
			_ = unsetCmd.Run()
		}()
	case "gpg":
		cmd := exec.Command("git", "config", "gpg.format", "openpgp")
		cmd.Dir = cfg.WorkDir()
		if err := shell.RunCommand(cmd, "git config gpg.format"); err != nil {
			return err
		}
		defer func() {
			unsetCmd := exec.Command("git", "config", "--unset", "gpg.format")
			unsetCmd.Dir = cfg.WorkDir()
			_ = unsetCmd.Run()
		}()

		cmd = exec.Command("git", "config", "user.signingkey", cfg.GPGKey)
		cmd.Dir = cfg.WorkDir()
		if err := shell.RunCommand(cmd, "git config user.signingkey"); err != nil {
			return err
		}
		defer func() {
			unsetCmd := exec.Command("git", "config", "--unset", "user.signingkey")
			unsetCmd.Dir = cfg.WorkDir()
			_ = unsetCmd.Run()
		}()
	}

	filterScript, err := buildUnifiedFilterScript(cfg)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "filter-branch", "-f",
		"--commit-filter", fmt.Sprintf("sh %s \"$@\"", shell.QuoteSingle(filterScript)),
		"--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	log.Success("Commit history rewritten successfully")
	return nil
}

func buildUnifiedFilterScript(cfg *config.Config) (string, error) {
	scriptPath := filepath.Join(cfg.TempDir, "unified_filter.sh")
	pyFilterPath, err := generateUnifiedPythonMessageFilter(cfg)
	if err != nil {
		return "", err
	}

	var script strings.Builder
	script.WriteString("#!/usr/bin/env sh\n")

	var matchCases []string
	for _, raw := range cfg.OldRawAuthors {
		_, email, _ := config.ParseAuthor(raw)
		if email != "" {
			matchCases = append(matchCases, shell.QuoteSingle(email))
		}
	}
	for _, e := range cfg.OldEmails {
		if e != "" {
			matchCases = append(matchCases, shell.QuoteSingle(e))
		}
	}

	var nameCases []string
	for _, raw := range cfg.OldRawAuthors {
		name, _, _ := config.ParseAuthor(raw)
		if name != "" {
			nameCases = append(nameCases, shell.QuoteSingle(name))
		}
	}
	for _, n := range cfg.OldNames {
		if n != "" {
			nameCases = append(nameCases, shell.QuoteSingle(n))
		}
	}

	if len(matchCases) > 0 || len(nameCases) > 0 {
		script.WriteString("match=0\n")
		if len(matchCases) > 0 {
			pattern := strings.Join(matchCases, "|")
			fmt.Fprintf(&script, "case \"$GIT_AUTHOR_EMAIL\" in %s) match=1 ;; esac\n", pattern)
			fmt.Fprintf(&script, "case \"$GIT_COMMITTER_EMAIL\" in %s) match=1 ;; esac\n", pattern)
		}
		if len(nameCases) > 0 {
			pattern := strings.Join(nameCases, "|")
			fmt.Fprintf(&script, "case \"$GIT_AUTHOR_NAME\" in %s) match=1 ;; esac\n", pattern)
			fmt.Fprintf(&script, "case \"$GIT_COMMITTER_NAME\" in %s) match=1 ;; esac\n", pattern)
		}
	} else {
		script.WriteString("match=1\n")
	}

	script.WriteString(`orig_tree="$(git rev-parse "$GIT_COMMIT^{tree}")"
expected="$orig_tree"
for p in $(git rev-list --parents -n 1 "$GIT_COMMIT" | cut -s -d' ' -f2-); do
    expected="$expected -p $p"
done
actual="$*"

if [ "$match" -eq 0 ] && [ "$expected" = "$actual" ]; then
    echo "$GIT_COMMIT"
    exit 0
fi
`)

	script.WriteString("if [ \"$match\" -eq 1 ]; then\n")
	if cfg.NewName != "" {
		fmt.Fprintf(&script, "    export GIT_AUTHOR_NAME=%s\n", shell.QuoteSingle(cfg.NewName))
		fmt.Fprintf(&script, "    export GIT_COMMITTER_NAME=%s\n", shell.QuoteSingle(cfg.NewName))
	}
	if cfg.NewEmail != "" {
		fmt.Fprintf(&script, "    export GIT_AUTHOR_EMAIL=%s\n", shell.QuoteSingle(cfg.NewEmail))
		fmt.Fprintf(&script, "    export GIT_COMMITTER_EMAIL=%s\n", shell.QuoteSingle(cfg.NewEmail))
	}

	if pyFilterPath != "" {
		fmt.Fprintf(&script, "    msg=$(python3 %s)\n", shell.QuoteSingle(pyFilterPath))
	} else {
		script.WriteString("    msg=$(cat)\n")
	}

	switch cfg.SignMode {
	case "ssh", "gpg":
		script.WriteString("    echo \"$msg\" | git commit-tree -S \"$@\"\n")
	default:
		script.WriteString("    echo \"$msg\" | git commit-tree \"$@\"\n")
	}

	script.WriteString("else\n")
	script.WriteString("    git commit-tree \"$@\"\n")
	script.WriteString("fi\n")

	if err := os.WriteFile(scriptPath, []byte(script.String()), 0755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func generateUnifiedPythonMessageFilter(cfg *config.Config) (string, error) {
	hasStrip := len(cfg.StripTitlePatterns) > 0 || len(cfg.StripBodyPatterns) > 0
	hasCoauthor := cfg.CoauthorMode == "rewrite" || cfg.CoauthorMode == "remove"
	if !hasStrip && !hasCoauthor {
		return "", nil
	}

	targets := coauthor.ParseCoauthorTargets(cfg)
	dataFile := filepath.Join(cfg.TempDir, "unified_msg.json")
	data := map[string]any{
		"strip_title":        cfg.StripTitlePatterns,
		"strip_body":         cfg.StripBodyPatterns,
		"coauthor_mode":      cfg.CoauthorMode,
		"coauthor_targets":   targets,
		"coauthor_new_name":  cfg.CoauthorNewName,
		"coauthor_new_email": cfg.CoauthorNewEmail,
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dataFile, b, 0644); err != nil {
		return "", err
	}

	pyScriptPath := filepath.Join(cfg.TempDir, "unified_msg.py")
	pyContent := fmt.Sprintf(`import sys, re, json

with open(%q, 'r', encoding='utf-8') as f:
    cfg = json.load(f)

raw = sys.stdin.read()
parts = raw.split("\n", 1)
title = parts[0]
body = parts[1] if len(parts) > 1 else ""

for p in (cfg.get('strip_title') or []):
    title = re.sub(p, "", title)
title = title.rstrip()

for p in (cfg.get('strip_body') or []):
    body = re.sub(p, "", body, flags=re.MULTILINE)

mode = cfg.get('coauthor_mode', '')
targets = cfg.get('coauthor_targets', [])
if mode == 'remove':
    for t in targets:
        name = t.get('name') or ''
        email = t.get('email') or ''
        if name and email:
            pat = r"(?m)^Co-authored-by:\s*" + re.escape(name) + r"\s*<" + re.escape(email) + r">(\r?\n)?"
        elif email:
            pat = r"(?m)^Co-authored-by:[^<\r\n]*<" + re.escape(email) + r">(\r?\n)?"
        else:
            pat = r"(?m)^Co-authored-by:\s*" + re.escape(name) + r"(\s*<[^>\r\n]*>)?(\r?\n)?"
        body = re.sub(pat, "", body)
elif mode == 'rewrite':
    new_tr = "Co-authored-by: " + cfg.get('coauthor_new_name', '') + " <" + cfg.get('coauthor_new_email', '') + ">"
    for t in targets:
        name = t.get('name') or ''
        email = t.get('email') or ''
        if name and email:
            pat = r"(?m)^Co-authored-by:\s*" + re.escape(name) + r"\s*<" + re.escape(email) + r">"
        elif email:
            pat = r"(?m)^Co-authored-by:[^<\r\n]*<" + re.escape(email) + r">"
        else:
            pat = r"(?m)^Co-authored-by:\s*" + re.escape(name) + r"(\s*<[^>\r\n]*>)?"
        body = re.sub(pat, new_tr, body)

body = re.sub(r"\n{3,}", "\n\n", body).strip("\r\n")
if body:
    sys.stdout.write(title + "\n\n" + body + "\n")
else:
    sys.stdout.write(title + "\n")
`, dataFile)

	if err := os.WriteFile(pyScriptPath, []byte(pyContent), 0755); err != nil {
		return "", err
	}
	return pyScriptPath, nil
}

func RewriteCommitsInteractive(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History (Interactive)")

	if len(cfg.State.CommitOverrides) == 0 && len(cfg.State.CommitMessages) == 0 &&
		len(cfg.StripTitlePatterns) == 0 && len(cfg.StripBodyPatterns) == 0 {
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

	return rewriteInteractiveFilterRepo(cfg)
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
	data := map[string]any{
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
