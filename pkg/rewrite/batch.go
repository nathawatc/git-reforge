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

func rewriteBatchFilterRepo(cfg *config.Config) error {
	log := cfg.Logger
	args := []string{"git", "filter-repo", "--force"}

	mailmapPath, err := generateMailmap(cfg)
	if err != nil {
		return err
	}
	if mailmapPath != "" {
		args = append(args, "--mailmap", mailmapPath)
	}

	replaceMsgPath, err := generateReplaceMessage(cfg)
	if err != nil {
		return err
	}
	if replaceMsgPath != "" {
		args = append(args, "--replace-message", replaceMsgPath)
	}

	if mailmapPath == "" && replaceMsgPath == "" {
		args = append(args, "--proceed")
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git-filter-repo batch"); err != nil {
		return err
	}
	log.Success("Commit history rewritten successfully")
	return nil
}

func generateMailmap(cfg *config.Config) (string, error) {
	if cfg.NewEmail == "" && cfg.NewName == "" {
		return "", nil
	}

	var lines []string
	targetName := cfg.NewName
	targetEmail := cfg.NewEmail

	for _, raw := range cfg.OldRawAuthors {
		name, email, _ := config.ParseAuthor(raw)
		if name != "" && email != "" {
			if targetName != "" {
				lines = append(lines, fmt.Sprintf("%s <%s> %s <%s>", targetName, targetEmail, name, email))
			} else {
				lines = append(lines, fmt.Sprintf("<%s> %s <%s>", targetEmail, name, email))
			}
		}
	}

	for _, name := range cfg.OldNames {
		if name == "" {
			continue
		}
		if targetName != "" {
			lines = append(lines, fmt.Sprintf("%s <%s> %s", targetName, targetEmail, name))
		} else {
			lines = append(lines, fmt.Sprintf("<%s> %s", targetEmail, name))
		}
	}

	for _, email := range cfg.OldEmails {
		if email == "" {
			continue
		}
		if targetName != "" {
			lines = append(lines, fmt.Sprintf("%s <%s> <%s>", targetName, targetEmail, email))
		} else {
			lines = append(lines, fmt.Sprintf("<%s> <%s>", targetEmail, email))
		}
	}

	if len(lines) == 0 {
		return "", nil
	}

	mailmapPath := filepath.Join(cfg.TempDir, "mailmap.txt")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(mailmapPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return mailmapPath, nil
}

func generateReplaceMessage(cfg *config.Config) (string, error) {
	var lines []string
	for _, p := range cfg.StripTitlePatterns {
		if strings.TrimSpace(p) != "" {
			lines = append(lines, "regex:"+p+"==>")
		}
	}
	for _, p := range cfg.StripBodyPatterns {
		if strings.TrimSpace(p) != "" {
			lines = append(lines, "regex:(?m)"+p+"(\\r?\\n)?==>")
		}
	}
	if len(lines) == 0 {
		return "", nil
	}

	path := filepath.Join(cfg.TempDir, "replace_message.txt")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
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
