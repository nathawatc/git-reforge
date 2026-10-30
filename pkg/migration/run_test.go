package migration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"git-reforge/pkg/config"
)

type testLogger struct{}

func (testLogger) Header(string)  {}
func (testLogger) Info(string)    {}
func (testLogger) Success(string) {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

func TestCleanupTempDirRemovesExistingTempDir(t *testing.T) {
	cfg := config.New()
	cfg.TempDir = t.TempDir()

	cleanupTempDir(cfg)

	if _, err := os.Stat(cfg.TempDir); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) error = %v want not exist", cfg.TempDir, err)
	}
}

func TestRunKeepsTempDirOnPostCloneError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source.git")
	seedDir := filepath.Join(root, "seed")

	runGit(t, root, "init", "--bare", sourceDir)
	runGit(t, root, "clone", sourceDir, seedDir)
	runGit(t, seedDir, "config", "user.name", "Old User")
	runGit(t, seedDir, "config", "user.email", "old@example.org")
	if err := os.WriteFile(filepath.Join(seedDir, "file.txt"), []byte("contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "add", "file.txt")
	runGit(t, seedDir, "commit", "-m", "initial")
	runGit(t, seedDir, "push", "origin", "HEAD:main")

	cfg := config.New()
	cfg.SourceURL = sourceDir
	cfg.NoPush = true
	cfg.OldNames = []string{"Old User"}
	cfg.NewName = "New User"
	cfg.NewEmail = "new@example.org"
	cfg.SignMode = "invalid"
	cfg.Logger = testLogger{}

	err := Run(cfg)
	if err == nil {
		t.Fatal("Run() error = nil want invalid sign mode error")
	}
	if _, statErr := os.Stat(cfg.TempDir); statErr != nil {
		t.Fatalf("temp dir %q was not preserved after error: %v", cfg.TempDir, statErr)
	}
	if removeErr := os.RemoveAll(cfg.TempDir); removeErr != nil {
		t.Fatalf("failed to remove preserved temp dir: %v", removeErr)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
