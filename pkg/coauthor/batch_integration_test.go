package coauthor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git-reforge/pkg/config"
)

type testLogger struct{}

func (testLogger) Header(string)  {}
func (testLogger) Info(string)    {}
func (testLogger) Success(string) {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

func TestRewriteBatchRewritesCoauthorTrailerWithSpecialCharacters(t *testing.T) {
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
	messageFile := filepath.Join(tmpDir, "message.txt")
	writeFile(t, messageFile, "Feature\n\nCo-authored-by: Alice <alice@example.org>\nCo-authored-by: Bob <bob@example.org>\n")
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_DATE=2003-04-05T06:07:08+07:00",
		"GIT_COMMITTER_DATE=2003-04-06T07:08:09+07:00",
	}, "commit", "-F", messageFile)

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorFrom = []string{"Alice"}
	cfg.CoauthorNewName = "New.Name & Co"
	cfg.CoauthorNewEmail = "new/name@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteBatch(cfg); err != nil {
		t.Fatalf("RewriteBatch() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if !strings.Contains(message, "Co-authored-by: New.Name & Co <new/name@example.org>") {
		t.Fatalf("rewritten message missing replacement trailer:\n%s", message)
	}
	if !strings.Contains(message, "Co-authored-by: Bob <bob@example.org>") {
		t.Fatalf("rewritten message removed unrelated trailer:\n%s", message)
	}
	if strings.Contains(message, "Alice <alice@example.org>") {
		t.Fatalf("rewritten message still contains old trailer:\n%s", message)
	}

	dates := runGitOutput(t, workDir, "log", "-1", "--format=%aI|%cI")
	if strings.TrimSpace(dates) != "2003-04-05T06:07:08+07:00|2003-04-06T07:08:09+07:00" {
		t.Fatalf("coauthor rewrite did not preserve dates: %q", dates)
	}
}

func TestRewriteBatchRemoveNarrowsByCoauthorEmail(t *testing.T) {
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
	messageFile := filepath.Join(tmpDir, "message.txt")
	writeFile(t, messageFile, strings.Join([]string{
		"Feature",
		"",
		"Co-authored-by: Alice <alice@example.org>",
		"Co-authored-by: Alice <alice@other.example.org>",
		"Co-authored-by: Bob <bob@example.org>",
		"",
	}, "\n"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-F", messageFile)

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.CoauthorMode = "remove"
	cfg.CoauthorFrom = []string{"Alice <alice@example.org>"}
	cfg.Logger = testLogger{}

	if err := RewriteBatch(cfg); err != nil {
		t.Fatalf("RewriteBatch() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if strings.Contains(message, "Co-authored-by: Alice <alice@example.org>") {
		t.Fatalf("rewritten message still contains removed trailer:\n%s", message)
	}
	for _, kept := range []string{
		"Co-authored-by: Alice <alice@other.example.org>",
		"Co-authored-by: Bob <bob@example.org>",
	} {
		if !strings.Contains(message, kept) {
			t.Fatalf("rewritten message removed non-matching trailer %q:\n%s", kept, message)
		}
	}
}

func TestRewriteBatchHandlesApostropheInCoauthorNamesAndEmails(t *testing.T) {
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
	messageFile := filepath.Join(tmpDir, "message.txt")
	writeFile(t, messageFile, "Feature\n\nCo-authored-by: O'Neil <o'neil@example.org>\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-F", messageFile)

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorFrom = []string{"O'Neil <o'neil@example.org>"}
	cfg.CoauthorNewName = "D'Angelo"
	cfg.CoauthorNewEmail = "d'angelo@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteBatch(cfg); err != nil {
		t.Fatalf("RewriteBatch() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if !strings.Contains(message, "Co-authored-by: D'Angelo <d'angelo@example.org>") {
		t.Fatalf("rewritten message missing apostrophe replacement:\n%s", message)
	}
	if strings.Contains(message, "O'Neil <o'neil@example.org>") {
		t.Fatalf("rewritten message still contains old apostrophe trailer:\n%s", message)
	}
}

func TestRewriteInteractiveAppliesPerCommitCoauthorDecision(t *testing.T) {
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
	messageFile := filepath.Join(tmpDir, "message.txt")
	writeFile(t, messageFile, "Feature\n\nCo-authored-by: Alice <alice@example.org>\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-F", messageFile)

	hash := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))
	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.CoauthorMode = "interactive"
	cfg.CoauthorFrom = []string{"Alice"}
	cfg.CoauthorNewName = "New Co"
	cfg.CoauthorNewEmail = "newco@example.org"
	cfg.Logger = testLogger{}
	cfg.State.CoauthorActions[hash] = "rewrite"

	if err := RewriteInteractive(cfg); err != nil {
		t.Fatalf("RewriteInteractive() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if !strings.Contains(message, "Co-authored-by: New Co <newco@example.org>") {
		t.Fatalf("message missing rewritten coauthor:\n%s", message)
	}
	if strings.Contains(message, "Alice <alice@example.org>") {
		t.Fatalf("message still contains old coauthor:\n%s", message)
	}
}

func TestRewriteBatchMultipleCoauthorFrom(t *testing.T) {
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
	messageFile := filepath.Join(tmpDir, "message.txt")
	writeFile(t, messageFile, "Feature\n\nCo-authored-by: Alice <alice@example.org>\nCo-authored-by: Charlie <charlie@example.org>\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-F", messageFile)

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorFrom = []string{"Alice", "Charlie"}
	cfg.CoauthorNewName = "Unified Co"
	cfg.CoauthorNewEmail = "unified@example.org"
	cfg.Logger = testLogger{}

	if err := RewriteBatch(cfg); err != nil {
		t.Fatalf("RewriteBatch() error = %v", err)
	}

	message := runGitOutput(t, workDir, "log", "-1", "--format=%B")
	if strings.Contains(message, "Alice <alice@example.org>") {
		t.Fatalf("Alice still present in message:\n%s", message)
	}
	if strings.Contains(message, "Charlie <charlie@example.org>") {
		t.Fatalf("Charlie still present in message:\n%s", message)
	}
	if !strings.Contains(message, "Co-authored-by: Unified Co <unified@example.org>") {
		t.Fatalf("rewritten trailer missing:\n%s", message)
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
