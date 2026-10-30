package signing

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"git-reforge/pkg/config"
	"git-reforge/pkg/shell"
)

func HandleSigning(cfg *config.Config) error {
	log := cfg.Logger

	switch cfg.SignMode {
	case "keep":
		log.Info("Keeping original signatures (note: they are now invalid due to content changes)")
	case "none":
		return handleNone(cfg)
	case "gpg":
		return handleGPG(cfg)
	case "ssh":
		return handleSSH(cfg)
	default:
		return fmt.Errorf("unknown sign mode: %q (valid: keep|gpg|ssh|none)", cfg.SignMode)
	}

	return nil
}

func handleNone(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Removing Signatures")
	log.Info("Stripping all commit signatures...")

	if shell.HasFilterRepo() {
		cmd := exec.Command("git", "filter-repo", "--force", "--commit-callback", "pass")
		cmd.Dir = cfg.WorkDir()
		if err := shell.RunCommand(cmd, "git-filter-repo strip signatures"); err != nil {
			return err
		}
		log.Success("Signatures removed")
		return ResignTags(cfg)
	}

	// Disable signing locally
	cmd := exec.Command("git", "config", "gpg.sign", "false")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git config gpg.sign"); err != nil {
		return err
	}

	// Run filter-branch to recreate commits without signatures.
	// The env-filter touches GIT_COMMITTER_DATE (even to its original value)
	// to force filter-branch to process every commit; gpg.sign=false ensures
	// the recreated objects are not signed.
	cmd = exec.Command("git", "filter-branch", "-f", "--env-filter", `
export GIT_COMMITTER_DATE="$GIT_COMMITTER_DATE"
`, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	// Unset config
	cmd = exec.Command("git", "config", "--unset", "gpg.sign")
	cmd.Dir = cfg.WorkDir()
	_ = cmd.Run()

	log.Success("Signatures removed")
	return ResignTags(cfg)
}

func handleGPG(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Re-signing with GPG")
	log.Info("Re-signing all commits with GPG key: " + cfg.GPGKey)

	cmd := exec.Command("git", "config", "user.signingkey", cfg.GPGKey)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git config user.signingkey"); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "gpg.format", "openpgp")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git config gpg.format"); err != nil {
		return err
	}

	commitFilter := fmt.Sprintf(`git -c gpg.format=openpgp commit-tree -S"%s" "$@"`, shell.EscapeDoubleQuoted(cfg.GPGKey))
	cmd = exec.Command("git", "filter-branch", "-f", "--commit-filter", commitFilter, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--unset", "user.signingkey")
	cmd.Dir = cfg.WorkDir()
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "--unset", "gpg.format")
	cmd.Dir = cfg.WorkDir()
	_ = cmd.Run()

	log.Success("Commits re-signed with GPG")
	return ResignTags(cfg)
}

func handleSSH(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Re-signing with SSH")
	log.Info("Re-signing all commits with SSH key: " + cfg.SSHKey)

	cmd := exec.Command("git", "config", "gpg.format", "ssh")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git config gpg.format"); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "user.signingkey", cfg.SSHKey)
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "git config user.signingkey"); err != nil {
		return err
	}

	cmd = exec.Command("git", "filter-branch", "-f", "--commit-filter", `git commit-tree -S "$@"`, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := shell.RunCommand(cmd, "filter-branch"); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--unset", "gpg.format")
	cmd.Dir = cfg.WorkDir()
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "--unset", "user.signingkey")
	cmd.Dir = cfg.WorkDir()
	_ = cmd.Run()

	log.Success("Commits re-signed with SSH")
	return ResignTags(cfg)
}

func ResignTags(cfg *config.Config) error {
	log := cfg.Logger

	// Get all annotated tags
	cmd := exec.Command("git", "tag")
	cmd.Dir = cfg.WorkDir()
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git tag failed: %w", err)
	}

	var annotatedTags []string
	for tag := range strings.SplitSeq(string(output), "\n") {
		if tag == "" {
			continue
		}
		cmd = exec.Command("git", "cat-file", "-t", "refs/tags/"+tag)
		cmd.Dir = cfg.WorkDir()
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(out)) == "tag" {
			annotatedTags = append(annotatedTags, tag)
		}
	}

	if len(annotatedTags) == 0 {
		log.Info("No annotated tags found to process")
		return nil
	}

	log.Info(fmt.Sprintf("Processing %d annotated tag(s)...", len(annotatedTags)))

	type tagOp struct {
		startMsg   string
		action     string
		failFormat string
		doneMsg    string
		successMsg string
		fn         func(*config.Config, string) error
	}

	ops := map[string]tagOp{
		"none": {
			startMsg:   "Stripping signatures from annotated tags...",
			action:     "strip tag",
			failFormat: "failed to strip tag %s: %w",
			doneMsg:    "  Stripped signature from tag: ",
			successMsg: "Tag signatures removed",
			fn:         stripTag,
		},
		"gpg": {
			startMsg:   "Re-signing annotated tags with GPG key: " + cfg.GPGKey + "...",
			action:     "re-sign tag",
			failFormat: "failed to re-sign GPG tag %s: %w",
			doneMsg:    "  Re-signed tag: ",
			successMsg: "Tags re-signed with GPG",
			fn:         resignTagGPG,
		},
		"ssh": {
			startMsg:   "Re-signing annotated tags with SSH key: " + cfg.SSHKey + "...",
			action:     "re-sign tag",
			failFormat: "failed to re-sign SSH tag %s: %w",
			doneMsg:    "  Re-signed tag: ",
			successMsg: "Tags re-signed with SSH",
			fn:         resignTagSSH,
		},
	}

	op, ok := ops[cfg.SignMode]
	if !ok {
		return nil
	}

	log.Info(op.startMsg)
	var failures []error
	for _, tag := range annotatedTags {
		if err := op.fn(cfg, tag); err != nil {
			log.Warning("Failed to " + op.action + ": " + tag)
			failures = append(failures, fmt.Errorf(op.failFormat, tag, err))
			continue
		}
		log.Info(op.doneMsg + tag)
	}
	if err := errors.Join(failures...); err != nil {
		return err
	}
	log.Success(op.successMsg)

	return nil
}

func getTagRecreateMessage(workDir string, tagName string) (string, error) {
	cmd := exec.Command("git", "tag", "-l", "--format=%(contents:subject)%00%(contents:body)", tagName)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git tag contents failed: %w", err)
	}
	parts := strings.SplitN(string(output), "\x00", 2)
	subject := strings.TrimRight(parts[0], "\n")
	if len(parts) < 2 {
		return subject, nil
	}
	body := strings.TrimRight(parts[1], "\n")
	if body == "" {
		return subject, nil
	}
	if subject == "" {
		return body, nil
	}
	return subject + "\n\n" + body, nil
}

func getTagCommit(workDir string, tagName string) (string, error) {
	cmd := exec.Command("git", "rev-list", "-1", tagName)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-list failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

type taggerInfo struct {
	name  string
	email string
	date  string
}

func getTagTagger(workDir string, tagName string) (taggerInfo, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(taggername)%00%(taggeremail:trim)%00%(taggerdate:raw)", "refs/tags/"+tagName)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return taggerInfo{}, fmt.Errorf("git tagger info failed: %w", err)
	}
	parts := strings.Split(strings.TrimRight(string(output), "\n"), "\x00")
	if len(parts) < 3 {
		return taggerInfo{}, nil
	}
	return taggerInfo{
		name:  parts[0],
		email: strings.Trim(parts[1], "<>"),
		date:  parts[2],
	}, nil
}

func taggerEnv(cfg *config.Config, info taggerInfo) []string {
	env := os.Environ()
	name := info.name
	email := info.email
	if slices.Contains(cfg.OldNames, name) {
		if cfg.NewName != "" {
			name = cfg.NewName
		}
		if cfg.NewEmail != "" {
			email = cfg.NewEmail
		}
	}
	if name != "" {
		env = append(env, "GIT_COMMITTER_NAME="+name)
	}
	if email != "" {
		env = append(env, "GIT_COMMITTER_EMAIL="+email)
	}
	if info.date != "" {
		env = append(env, "GIT_COMMITTER_DATE="+info.date)
	}
	return env
}

func recreateTag(cfg *config.Config, tagName string, desc string, prefixArgs []string, tagFlags ...string) error {
	workDir := cfg.WorkDir()
	msg, err := getTagRecreateMessage(workDir, tagName)
	if err != nil {
		return err
	}
	tagCommit, err := getTagCommit(workDir, tagName)
	if err != nil {
		return err
	}
	tagger, err := getTagTagger(workDir, tagName)
	if err != nil {
		return err
	}

	args := append([]string{}, prefixArgs...)
	args = append(args, "tag", "-f")
	args = append(args, tagFlags...)
	args = append(args, tagName, tagCommit, "-m", msg)

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	cmd.Env = taggerEnv(cfg, tagger)
	return shell.RunCommand(cmd, desc)
}

func stripTag(cfg *config.Config, tagName string) error {
	return recreateTag(cfg, tagName, "git tag strip", nil, "-a")
}

func resignTagGPG(cfg *config.Config, tagName string) error {
	return recreateTag(cfg, tagName, "git tag resign GPG",
		[]string{"-c", "gpg.format=openpgp"},
		"-s", "-u", cfg.GPGKey)
}

func resignTagSSH(cfg *config.Config, tagName string) error {
	return recreateTag(cfg, tagName, "git tag resign SSH",
		[]string{"-c", "gpg.format=ssh", "-c", "user.signingkey=" + cfg.SSHKey},
		"-s")
}
