package migration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github-to-codeberg/pkg/config"
	"github-to-codeberg/pkg/migration"
)

type pipelineTestLogger struct{}

func (pipelineTestLogger) Header(string)  {}
func (pipelineTestLogger) Info(string)    {}
func (pipelineTestLogger) Success(string) {}
func (pipelineTestLogger) Warning(string) {}
func (pipelineTestLogger) Error(string)   {}

func TestNonInteractiveMigrationPipelineLocalRepos(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source.git")
	destDir := filepath.Join(root, "dest.git")
	saveDir := filepath.Join(root, "saved")
	seedDir := filepath.Join(root, "seed")

	runGit(t, root, "init", "--bare", sourceDir)
	runGit(t, root, "init", "--bare", destDir)
	runGit(t, root, "clone", sourceDir, seedDir)
	runGit(t, seedDir, "config", "user.name", "Old User")
	runGit(t, seedDir, "config", "user.email", "old@example.org")

	writeFile(t, filepath.Join(seedDir, "file.txt"), "contents\n")
	messageFile := filepath.Join(root, "message.txt")
	writeFile(t, messageFile, "Feature\n\nCo-authored-by: Alice <alice@example.org>\n")
	runGit(t, seedDir, "add", "file.txt")
	runGitWithEnv(t, seedDir, []string{
		"GIT_AUTHOR_NAME=Old User",
		"GIT_AUTHOR_EMAIL=old@example.org",
		"GIT_AUTHOR_DATE=2005-06-07T08:09:10+07:00",
		"GIT_COMMITTER_NAME=Old User",
		"GIT_COMMITTER_EMAIL=old@example.org",
		"GIT_COMMITTER_DATE=2005-06-08T09:10:11+07:00",
	}, "commit", "-F", messageFile)
	runGit(t, seedDir, "branch", "-M", "main")
	head := strings.TrimSpace(runGitOutput(t, seedDir, "rev-parse", "HEAD"))
	runGit(t, seedDir, "update-ref", "refs/tags/v1.0.0", head)
	runGit(t, seedDir, "push", "origin", "main", "v1.0.0")

	cfg := config.New()
	cfg.SourceURL = sourceDir
	cfg.DestURL = destDir
	cfg.OldNames = []string{"Old User"}
	cfg.NewName = "New User"
	cfg.NewEmail = "new@example.org"
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorOldName = "Alice"
	cfg.CoauthorOldEmail = "alice@example.org"
	cfg.CoauthorNewName = "New Co"
	cfg.CoauthorNewEmail = "newco@example.org"
	cfg.SignMode = "keep"
	cfg.SaveDir = saveDir
	cfg.Logger = pipelineTestLogger{}

	if err := migration.Run(cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := os.Stat(cfg.TempDir); !os.IsNotExist(err) {
		t.Fatalf("temp dir %q still exists: %v", cfg.TempDir, err)
	}

	cloneDir := filepath.Join(root, "dest-clone")
	runGit(t, root, "clone", destDir, cloneDir)
	runGit(t, cloneDir, "checkout", "main")

	log := runGitOutput(t, cloneDir, "log", "--format=%an <%ae>|%cn <%ce>|%aI|%cI|%B", "-1")
	for _, want := range []string{
		"New User <new@example.org>|New User <new@example.org>|2005-06-07T08:09:10+07:00|2005-06-08T09:10:11+07:00",
		"Co-authored-by: New Co <newco@example.org>",
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("destination log missing %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "Old User") || strings.Contains(log, "Alice <alice@example.org>") {
		t.Fatalf("destination log still contains old identity/trailer:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(saveDir, ".git")); err != nil {
		t.Fatalf("saved repository missing .git: %v", err)
	}
	savedLog := runGitOutput(t, saveDir, "log", "--all", "--format=%an <%ae>")
	if strings.Contains(savedLog, "Old User") {
		t.Fatalf("saved repository still contains old author in git log --all:\n%s", savedLog)
	}
	tags := runGitOutput(t, destDir, "for-each-ref", "--format=%(refname)", "refs/tags")
	if !strings.Contains(tags, "refs/tags/v1.0.0\n") {
		t.Fatalf("destination tags = %q missing v1.0.0", tags)
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	runGitWithEnv(t, dir, nil, args...)
}

func runGitWithEnv(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s failed: %v", strings.Join(args, " "), err)
	}
	return string(output)
}
