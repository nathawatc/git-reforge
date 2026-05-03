package signing

import (
	"fmt"
	"os/exec"
	"strings"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/shell"
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
	}

	return nil
}

func handleNone(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Removing Signatures")
	log.Info("Stripping all commit signatures...")

	// Disable signing locally
	cmd := exec.Command("git", "config", "gpg.sign", "false")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config gpg.sign failed: %w", err)
	}

	// Run filter-branch to recreate commits without signatures.
	// The env-filter touches GIT_COMMITTER_DATE (even to its original value)
	// to force filter-branch to process every commit; gpg.sign=false ensures
	// the recreated objects are not signed.
	cmd = exec.Command("git", "filter-branch", "-f", "--env-filter", `
export GIT_COMMITTER_DATE="$GIT_COMMITTER_DATE"
`, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	// Unset config
	cmd = exec.Command("git", "config", "--unset", "gpg.sign")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config --unset failed: %w", err)
	}

	log.Success("Signatures removed")
	return ResignTags(cfg)
}

func handleGPG(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Re-signing with GPG")
	log.Info("Re-signing all commits with GPG key: " + cfg.GPGKey)

	cmd := exec.Command("git", "config", "user.signingkey", cfg.GPGKey)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config user.signingkey failed: %w", err)
	}

	commitFilter := fmt.Sprintf(`git commit-tree -S"%s" "$@"`, shell.EscapeShell(cfg.GPGKey))
	cmd = exec.Command("git", "filter-branch", "-f", "--commit-filter", commitFilter, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	cmd = exec.Command("git", "config", "--unset", "user.signingkey")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config --unset user.signingkey failed: %w", err)
	}

	log.Success("Commits re-signed with GPG")
	return ResignTags(cfg)
}

func handleSSH(cfg *config.Config) error {
	log := cfg.Logger
	log.Header("Re-signing with SSH")
	log.Info("Re-signing all commits with SSH key: " + cfg.SSHKey)

	cmd := exec.Command("git", "config", "gpg.format", "ssh")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config gpg.format failed: %w", err)
	}

	cmd = exec.Command("git", "config", "user.signingkey", cfg.SSHKey)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config user.signingkey failed: %w", err)
	}

	cmd = exec.Command("git", "filter-branch", "-f", "--commit-filter", `git commit-tree -S "$@"`, "--tag-name-filter", "cat", "--", "--all")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("filter-branch failed: %w", err)
	}

	cmd = exec.Command("git", "config", "--unset", "gpg.format")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config --unset gpg.format failed: %w", err)
	}

	cmd = exec.Command("git", "config", "--unset", "user.signingkey")
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config --unset user.signingkey failed: %w", err)
	}

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

	switch cfg.SignMode {
	case "none":
		log.Info("Stripping signatures from annotated tags...")
		for _, tag := range annotatedTags {
			if err := stripTag(cfg, tag); err != nil {
				log.Warning("Failed to strip tag: " + tag)
				continue
			}
			log.Info("  Stripped signature from tag: " + tag)
		}
		log.Success("Tag signatures removed")
	case "gpg":
		log.Info("Re-signing annotated tags with GPG key: " + cfg.GPGKey + "...")
		for _, tag := range annotatedTags {
			if err := resignTagGPG(cfg, tag); err != nil {
				log.Warning("Failed to re-sign tag: " + tag)
				continue
			}
			log.Info("  Re-signed tag: " + tag)
		}
		log.Success("Tags re-signed with GPG")
	case "ssh":
		log.Info("Re-signing annotated tags with SSH key: " + cfg.SSHKey + "...")
		for _, tag := range annotatedTags {
			if err := resignTagSSH(cfg, tag); err != nil {
				log.Warning("Failed to re-sign tag: " + tag)
				continue
			}
			log.Info("  Re-signed tag: " + tag)
		}
		log.Success("Tags re-signed with SSH")
	}

	return nil
}

func getTagMessage(tmpDir string, tagName string) (string, string, error) {
	cmd := exec.Command("git", "tag", "-l", "--format=%(contents:subject)", tagName)
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git tag subject failed: %w", err)
	}
	subject := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "tag", "-l", "--format=%(contents:body)", tagName)
	cmd.Dir = tmpDir + "/working"
	output, err = cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git tag body failed: %w", err)
	}
	body := strings.TrimSpace(string(output))

	return subject, body, nil
}

func getTagCommit(tmpDir string, tagName string) (string, error) {
	cmd := exec.Command("git", "rev-list", "-1", tagName)
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-list failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func stripTag(cfg *config.Config, tagName string) error {
	_, body, err := getTagMessage(cfg.TempDir, tagName)
	if err != nil {
		return err
	}
	tagCommit, err := getTagCommit(cfg.TempDir, tagName)
	if err != nil {
		return err
	}

	msg := body
	cmd := exec.Command("git", "tag", "-f", "-a", tagName, tagCommit, "-m", msg)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git tag strip failed: %w", err)
	}
	return nil
}

func resignTagGPG(cfg *config.Config, tagName string) error {
	_, body, err := getTagMessage(cfg.TempDir, tagName)
	if err != nil {
		return err
	}
	tagCommit, err := getTagCommit(cfg.TempDir, tagName)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "tag", "-f", "-s", tagName, "-u", cfg.GPGKey, tagCommit, "-m", body)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git tag resign GPG failed: %w", err)
	}
	return nil
}

func resignTagSSH(cfg *config.Config, tagName string) error {
	_, body, err := getTagMessage(cfg.TempDir, tagName)
	if err != nil {
		return err
	}
	tagCommit, err := getTagCommit(cfg.TempDir, tagName)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "tag", "-f", "-s", tagName, tagCommit, "-m", body)
	cmd.Dir = cfg.WorkDir()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git tag resign SSH failed: %w", err)
	}
	return nil
}
