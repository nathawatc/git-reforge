package rewrite

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/shell"
)

func RewriteCommitsBatch(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Rewriting Commit History")

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

	return rewriteBatchFilterRepo(cfg)
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

const pyStripCommitMessage = `strip_title = [re.compile(p.encode('utf-8')) for p in (cfg.get('strip_title') or [])]
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
`

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
	data := map[string]any{
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

`+pyStripCommitMessage, dataFile)

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

`+pyStripCommitMessage, dataFile)

	cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", pyCallback)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo interactive"); err != nil {
		return err
	}
	log.Success("Commit history rewritten successfully")
	return nil
}
