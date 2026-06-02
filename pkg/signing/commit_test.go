package signing

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github-to-codeberg/pkg/config"
)

type testLogger struct{}

func (testLogger) Header(string)  {}
func (testLogger) Info(string)    {}
func (testLogger) Success(string) {}
func (testLogger) Warning(string) {}
func (testLogger) Error(string)   {}

func TestStripTagPreservesSubjectAndBody(t *testing.T) {
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
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "tag", "-a", "v1.0.0", "-m", "Release v1.0.0", "-m", "Body line one\nBody line two")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.Logger = testLogger{}

	before, err := getTagRecreateMessage(tmpDir, "v1.0.0")
	if err != nil {
		t.Fatalf("getTagRecreateMessage() before error = %v", err)
	}
	want := "Release v1.0.0\n\nBody line one\nBody line two"
	if before != want {
		t.Fatalf("message before strip = %q want %q", before, want)
	}

	if err := stripTag(cfg, "v1.0.0"); err != nil {
		t.Fatalf("stripTag() error = %v", err)
	}

	after, err := getTagRecreateMessage(tmpDir, "v1.0.0")
	if err != nil {
		t.Fatalf("getTagRecreateMessage() after error = %v", err)
	}
	if after != want {
		t.Fatalf("message after strip = %q want %q", after, want)
	}
}

func TestGetTagRecreateMessagePreservesIndentedBody(t *testing.T) {
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
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "tag", "-a", "v1.0.1", "-m", "Release v1.0.1", "-m", "  indented body\nnext line")

	got, err := getTagRecreateMessage(tmpDir, "v1.0.1")
	if err != nil {
		t.Fatalf("getTagRecreateMessage() error = %v", err)
	}
	want := "Release v1.0.1\n\n  indented body\nnext line"
	if got != want {
		t.Fatalf("message = %q want %q", got, want)
	}
}

func TestTrimCommandTrailingNewlinesPreservesOtherWhitespace(t *testing.T) {
	got := trimCommandTrailingNewlines("  keep spaces  \n\n")
	want := "  keep spaces  "
	if got != want {
		t.Fatalf("trimCommandTrailingNewlines() = %q want %q", got, want)
	}
}

func TestHandleNonePreservesCommitDates(t *testing.T) {
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
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGitWithEnv(t, workDir, []string{
		"GIT_AUTHOR_DATE=2004-05-06T07:08:09+07:00",
		"GIT_COMMITTER_DATE=2004-05-07T08:09:10+07:00",
	}, "commit", "-m", "initial")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.SignMode = "none"
	cfg.Logger = testLogger{}

	if err := handleNone(cfg); err != nil {
		t.Fatalf("handleNone() error = %v", err)
	}

	dates := runGitOutput(t, workDir, "log", "-1", "--format=%aI|%cI")
	if strings.TrimSpace(dates) != "2004-05-06T07:08:09+07:00|2004-05-07T08:09:10+07:00" {
		t.Fatalf("handleNone did not preserve dates: %q", dates)
	}
}

func TestResignTagsReturnsFailedToErrorWhenTagSigningFails(t *testing.T) {
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
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "-c", "tag.gpgSign=false", "tag", "-a", "v1.0.0", "-m", "Release v1.0.0")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.SignMode = "ssh"
	cfg.SSHKey = filepath.Join(tmpDir, "missing-key")
	cfg.Logger = testLogger{}

	err := ResignTags(cfg)
	if err == nil {
		t.Fatal("ResignTags() error = nil want tag signing failure")
	}
	if !strings.Contains(err.Error(), "failed to re-sign SSH tag v1.0.0") {
		t.Fatalf("ResignTags() error = %v want tag failure context", err)
	}
}

func TestHandleSSHSignedCommitsAndAnnotatedTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(tmpDir, "signing_key")
	runCmd(t, tmpDir, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", keyPath)

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "-c", "tag.gpgSign=false", "tag", "-a", "v1.0.0", "-m", "Release v1.0.0")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.SignMode = "ssh"
	cfg.SSHKey = keyPath
	cfg.Logger = testLogger{}

	if err := handleSSH(cfg); err != nil {
		t.Fatalf("handleSSH() error = %v", err)
	}

	commitObject := runGitOutput(t, workDir, "cat-file", "commit", "HEAD")
	if !strings.Contains(commitObject, "-----BEGIN SSH SIGNATURE-----") {
		t.Fatalf("commit object missing SSH signature:\n%s", commitObject)
	}

	tagObject := runGitOutput(t, workDir, "cat-file", "tag", "refs/tags/v1.0.0")
	if !strings.Contains(tagObject, "-----BEGIN SSH SIGNATURE-----") {
		t.Fatalf("tag object missing SSH signature:\n%s", tagObject)
	}
}

func TestHandleGPGSignedCommitsAndAnnotatedTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not available")
	}

	tmpDir := t.TempDir()
	gnupgHome := filepath.Join(tmpDir, "gnupg")
	if err := os.Mkdir(gnupgHome, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GNUPGHOME", gnupgHome)
	runGPGKeygenOrSkip(t, tmpDir, "Test Signer <signer@example.org>")
	keyID := gpgFingerprint(t, tmpDir, "signer@example.org")

	workDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"))
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "-c", "tag.gpgSign=false", "tag", "-a", "v1.0.0", "-m", "Release v1.0.0")

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.SignMode = "gpg"
	cfg.GPGKey = keyID
	cfg.Logger = testLogger{}

	if err := handleGPG(cfg); err != nil {
		t.Fatalf("handleGPG() error = %v", err)
	}

	commitObject := runGitOutput(t, workDir, "cat-file", "commit", "HEAD")
	if !strings.Contains(commitObject, "-----BEGIN PGP SIGNATURE-----") {
		t.Fatalf("commit object missing PGP signature:\n%s", commitObject)
	}

	tagObject := runGitOutput(t, workDir, "cat-file", "tag", "refs/tags/v1.0.0")
	if !strings.Contains(tagObject, "-----BEGIN PGP SIGNATURE-----") {
		t.Fatalf("tag object missing PGP signature:\n%s", tagObject)
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("contents\n"), 0644); err != nil {
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
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return string(output)
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}

func runGPGKeygenOrSkip(t *testing.T, dir string, userID string) {
	t.Helper()
	cmd := exec.Command("gpg",
		"--batch", "--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-generate-key", userID, "ed25519", "sign", "1d")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("gpg key generation unavailable in this environment: %v\n%s", err, output)
	}
}

func gpgFingerprint(t *testing.T, dir string, userID string) string {
	t.Helper()
	cmd := exec.Command("gpg", "--batch", "--with-colons", "--list-secret-keys", userID)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("gpg list secret keys failed: %v", err)
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 9 && fields[0] == "fpr" && fields[9] != "" {
			return fields[9]
		}
	}
	t.Fatalf("no fingerprint found in gpg output:\n%s", output)
	return ""
}
