package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetCommitsByIdentityIncludesCommitterMatches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "New Author")
	runGit(t, workDir, "config", "user.email", "new@example.org")
	if err := os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("contents\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=New Author",
		"GIT_AUTHOR_EMAIL=new@example.org",
		"GIT_COMMITTER_NAME=Old Committer",
		"GIT_COMMITTER_EMAIL=old@example.org",
	}, "commit", "-m", "committer match")

	commits := getCommitsByIdentity(tmpDir, "Old Committer")
	if len(commits) != 1 {
		t.Fatalf("getCommitsByIdentity() returned %d commits, want 1: %v", len(commits), commits)
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
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
