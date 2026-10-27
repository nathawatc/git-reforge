package rewrite

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github-to-codeberg/pkg/config"
)

type testLogger struct{}

func (testLogger) Header(string)  {}
func (testLogger) Info(string)    {}
func (testLogger) Success(string) {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

func TestRewriteCommitsBatchRewritesMatchingAuthors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "old.txt"), "old\n")
	runGit(t, workDir, "add", "old.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old [bot]",
		"GIT_AUTHOR_EMAIL=old@example.org",
		"GIT_AUTHOR_DATE=2001-02-03T04:05:06+07:00",
		"GIT_COMMITTER_NAME=Old [bot]",
		"GIT_COMMITTER_EMAIL=old@example.org",
		"GIT_COMMITTER_DATE=2001-02-04T05:06:07+07:00",
	}, "commit", "-m", "old commit")

	writeFile(t, filepath.Join(workDir, "other.txt"), "other\n")
	runGit(t, workDir, "add", "other.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Other User",
		"GIT_AUTHOR_EMAIL=other@example.org",
		"GIT_COMMITTER_NAME=Other User",
		"GIT_COMMITTER_EMAIL=other@example.org",
	}, "commit", "-m", "other commit")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.OldNames = []string{"Old [bot]"}
	cfg.NewName = "New User"
	cfg.NewEmail = "new@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteCommitsBatch(cfg); err != nil {
		t.Fatalf("RewriteCommitsBatch() error = %v", err)
	}

	log := runGitOutput(t, workDir, "log", "--all", "--format=%an <%ae>|%cn <%ce>|%s")
	if !strings.Contains(log, "New User <new@example.org>|New User <new@example.org>|old commit") {
		t.Fatalf("rewritten log missing new identity:\n%s", log)
	}
	if !strings.Contains(log, "Other User <other@example.org>|Other User <other@example.org>|other commit") {
		t.Fatalf("rewritten log changed non-matching identity:\n%s", log)
	}

	dates := runGitOutput(t, workDir, "log", "--all", "--format=%s|%aI|%cI")
	if !strings.Contains(dates, "old commit|2001-02-03T04:05:06+07:00|2001-02-04T05:06:07+07:00") {
		t.Fatalf("rewritten log did not preserve dates:\n%s", dates)
	}
}

func TestRewriteCommitsBatchRewritesMultipleOldAuthors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "one.txt"), "one\n")
	runGit(t, workDir, "add", "one.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old One",
		"GIT_AUTHOR_EMAIL=one@example.org",
		"GIT_COMMITTER_NAME=Old One",
		"GIT_COMMITTER_EMAIL=one@example.org",
	}, "commit", "-m", "old one")

	writeFile(t, filepath.Join(workDir, "two.txt"), "two\n")
	runGit(t, workDir, "add", "two.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old Two",
		"GIT_AUTHOR_EMAIL=two@example.org",
		"GIT_COMMITTER_NAME=Old Two",
		"GIT_COMMITTER_EMAIL=two@example.org",
	}, "commit", "-m", "old two")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.OldNames = []string{"Old One", "Old Two"}
	cfg.NewName = "New User"
	cfg.NewEmail = "new@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteCommitsBatch(cfg); err != nil {
		t.Fatalf("RewriteCommitsBatch() error = %v", err)
	}

	log := runGitOutput(t, workDir, "log", "--all", "--format=%an <%ae>|%s")
	for _, subject := range []string{"old one", "old two"} {
		want := "New User <new@example.org>|" + subject
		if !strings.Contains(log, want) {
			t.Fatalf("rewritten log = %q missing %q", log, want)
		}
	}
}

func TestRewriteCommitsInteractivePreservesApostropheInAuthor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "file.txt"), "old\n")
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old User",
		"GIT_AUTHOR_EMAIL=old@example.org",
		"GIT_AUTHOR_DATE=2002-03-04T05:06:07+07:00",
		"GIT_COMMITTER_NAME=Old User",
		"GIT_COMMITTER_EMAIL=old@example.org",
		"GIT_COMMITTER_DATE=2002-03-05T06:07:08+07:00",
	}, "commit", "-m", "old commit")

	hashes := strings.Fields(runGitOutput(t, workDir, "log", "--all", "--reverse", "--format=%H"))
	if len(hashes) != 1 {
		t.Fatalf("got %d commits want 1", len(hashes))
	}

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.Logger = testLogger{}
	cfg.State.CommitOverrides[plumbing.NewHash(hashes[0])] = config.CommitOverride{
		Name:  "O'Connor",
		Email: "new@example.org",
	}

	if err := RewriteCommitsInteractive(cfg); err != nil {
		t.Fatalf("RewriteCommitsInteractive() error = %v", err)
	}

	log := runGitOutput(t, workDir, "log", "--all", "--format=%an <%ae>|%cn <%ce>|%s")
	want := "O'Connor <new@example.org>|O'Connor <new@example.org>|old commit"
	if !strings.Contains(log, want) {
		t.Fatalf("rewritten log = %q want to contain %q", log, want)
	}

	dates := runGitOutput(t, workDir, "log", "--all", "--format=%s|%aI|%cI")
	if !strings.Contains(dates, "old commit|2002-03-04T05:06:07+07:00|2002-03-05T06:07:08+07:00") {
		t.Fatalf("interactive rewrite did not preserve dates:\n%s", dates)
	}
}

func TestRewriteCommitsInteractiveRewritesCommitMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"), "contents\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "old subject")

	hashes := strings.Fields(runGitOutput(t, workDir, "log", "--all", "--reverse", "--format=%H"))
	if len(hashes) != 1 {
		t.Fatalf("got %d commits want 1", len(hashes))
	}

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.Logger = testLogger{}
	cfg.State.CommitMessages[plumbing.NewHash(hashes[0])] = "new subject\n\nnew body"

	if err := RewriteCommitsInteractive(cfg); err != nil {
		t.Fatalf("RewriteCommitsInteractive() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if strings.TrimSpace(message) != "new subject\n\nnew body" {
		t.Fatalf("message = %q want rewritten message", message)
	}
}

func TestRewriteCommitsInteractivePreservesUntouchedCommitMessageTrailingNewline(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "f1.txt"), "1\n")
	runGit(t, workDir, "add", "f1.txt")
	runGit(t, workDir, "commit", "-m", "First commit\n\nSome details\n")
	firstHash := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))

	writeFile(t, filepath.Join(workDir, "f2.txt"), "2\n")
	runGit(t, workDir, "add", "f2.txt")
	runGit(t, workDir, "commit", "-m", "Second commit")
	secondHash := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.Logger = testLogger{}
	cfg.State.CommitMessages[plumbing.NewHash(secondHash)] = "Updated second"

	if err := RewriteCommitsInteractive(cfg); err != nil {
		t.Fatalf("RewriteCommitsInteractive() error = %v", err)
	}

	// Verify the untouched first commit preserved its exact body and trailing newline
	firstMsg := runGitOutput(t, workDir, "log", firstHash, "-1", "--format=%B")
	wantFirst := "First commit\n\nSome details\n\n"
	if firstMsg != wantFirst {
		t.Fatalf("untouched commit message = %q want %q", firstMsg, wantFirst)
	}
}

func TestRewriteCommitsBatchPreservesAuthorNameWhenNewNameEmpty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "file.txt"), "contents\n")
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old User",
		"GIT_AUTHOR_EMAIL=old@example.org",
		"GIT_COMMITTER_NAME=Old User",
		"GIT_COMMITTER_EMAIL=old@example.org",
	}, "commit", "-m", "email only change")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.OldNames = []string{"Old User"}
	cfg.NewName = ""
	cfg.NewEmail = "new@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteCommitsBatch(cfg); err != nil {
		t.Fatalf("RewriteCommitsBatch() error = %v", err)
	}

	log := runGitOutput(t, workDir, "log", "-1", "--format=%an <%ae>|%cn <%ce>")
	want := "Old User <new@example.org>|Old User <new@example.org>\n"
	if log != want {
		t.Fatalf("rewritten log = %q want %q", log, want)
	}
}

func TestRewriteCommitsBatchHandlesSpacesInTempDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := filepath.Join(t.TempDir(), "path with spaces")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(workDir, "file.txt"), "contents\n")
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_NAME=Old User",
		"GIT_AUTHOR_EMAIL=old@example.org",
		"GIT_COMMITTER_NAME=Old User",
		"GIT_COMMITTER_EMAIL=old@example.org",
	}, "commit", "-m", "spaced tempdir test")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.OldNames = []string{"Old User"}
	cfg.NewName = "New User"
	cfg.NewEmail = "new@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteCommitsBatch(cfg); err != nil {
		t.Fatalf("RewriteCommitsBatch() error = %v", err)
	}

	log := runGitOutput(t, workDir, "log", "-1", "--format=%an <%ae>")
	want := "New User <new@example.org>\n"
	if log != want {
		t.Fatalf("rewritten log = %q want %q", log, want)
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
