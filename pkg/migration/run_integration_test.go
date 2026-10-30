package migration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"git-reforge/pkg/config"
	"git-reforge/pkg/migration"
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
	cfg.CoauthorFrom = []string{"Alice <alice@example.org>"}
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

func TestForkMigrationPreservesUpstreamCommitsAndSignatures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source.git")
	destDir := filepath.Join(root, "dest.git")
	seedDir := filepath.Join(root, "seed")
	boltKey := filepath.Join(root, "bolt_key")
	scottKey := filepath.Join(root, "scott_key")

	runCmd(t, root, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", boltKey)
	runCmd(t, root, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", scottKey)

	runGit(t, root, "init", "--bare", sourceDir)
	runGit(t, root, "init", "--bare", destDir)
	runGit(t, root, "clone", sourceDir, seedDir)

	// Upstream commit by boltgolt (signed, with PR # and Reviewed-on)
	runGit(t, seedDir, "config", "gpg.format", "ssh")
	runGit(t, seedDir, "config", "user.name", "boltgolt")
	runGit(t, seedDir, "config", "user.email", "boltgolt@gmail.com")
	runGit(t, seedDir, "config", "user.signingkey", boltKey)
	runGit(t, seedDir, "config", "commit.gpgsign", "true")

	writeFile(t, filepath.Join(seedDir, "bolt.txt"), "upstream\n")
	runGit(t, seedDir, "add", "bolt.txt")
	runGit(t, seedDir, "commit", "-m", "upstream commit (#10)\n\nReviewed-on: https://github.com/boltgolt/howdy/pull/10\n")
	origBoltSHA := strings.TrimSpace(runGitOutput(t, seedDir, "rev-parse", "HEAD"))

	// Fork commit by nathawat
	runGit(t, seedDir, "config", "commit.gpgsign", "false")
	runGit(t, seedDir, "config", "user.name", "nathawat")
	runGit(t, seedDir, "config", "user.email", "nathawat@noreply.codeberg.org")
	writeFile(t, filepath.Join(seedDir, "nathawat.txt"), "fork\n")
	runGit(t, seedDir, "add", "nathawat.txt")
	runGit(t, seedDir, "commit", "-m", "fork commit (#100)\n\nRefs: #100\nReviewed-on: https://codeberg.org/nathawat/howdy-next/pull/100\nCo-authored-by: Old Friend <old@friend.com>\n")

	runGit(t, seedDir, "branch", "-M", "main")
	runGit(t, seedDir, "push", "origin", "main")

	cfg := config.New()
	cfg.SourceURL = sourceDir
	cfg.DestURL = destDir
	cfg.OldRawAuthors = []string{"nathawat <nathawat@noreply.codeberg.org>"}
	cfg.OldNames = []string{"nathawat"}
	cfg.OldEmails = []string{"nathawat@noreply.codeberg.org"}
	cfg.NewName = "Scott-Nx"
	cfg.NewEmail = "158021536+Scott-Nx@users.noreply.github.com"
	cfg.SignMode = "ssh"
	cfg.SSHKey = scottKey
	cfg.StripTitlePatterns = []string{`\s*\(#\d+\)`}
	cfg.StripBodyPatterns = []string{`^Refs:?.*$`, `^Reviewed-on:.*$`}
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorFrom = []string{"Old Friend <old@friend.com>"}
	cfg.CoauthorNewName = "New Friend"
	cfg.CoauthorNewEmail = "new@friend.com"
	cfg.Logger = pipelineTestLogger{}

	if err := migration.Run(cfg); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cloneDir := filepath.Join(root, "dest-clone")
	runGit(t, root, "clone", destDir, cloneDir)
	runGit(t, cloneDir, "checkout", "main")

	// 1. Assert upstream commit by boltgolt is 100% untouched
	boltAfter := strings.TrimSpace(runGitOutput(t, cloneDir, "rev-list", "--max-parents=0", "HEAD"))
	if boltAfter != origBoltSHA {
		t.Fatalf("Upstream boltgolt commit SHA changed from %s to %s", origBoltSHA, boltAfter)
	}
	boltCommit := runGitOutput(t, cloneDir, "cat-file", "commit", boltAfter)
	if !strings.Contains(boltCommit, "-----BEGIN SSH SIGNATURE-----") {
		t.Fatal("Upstream boltgolt commit lost signature")
	}
	if !strings.Contains(boltCommit, "upstream commit (#10)") {
		t.Fatalf("Upstream boltgolt commit title was stripped: %s", boltCommit)
	}
	if !strings.Contains(boltCommit, "Reviewed-on: https://github.com/boltgolt/howdy/pull/10") {
		t.Fatalf("Upstream boltgolt commit body was stripped: %s", boltCommit)
	}

	// 2. Assert fork commit by nathawat was rewritten and signed by Scott-Nx
	forkCommit := runGitOutput(t, cloneDir, "cat-file", "commit", "HEAD")
	if strings.Contains(forkCommit, "nathawat") {
		t.Fatalf("Fork commit still contains old author: %s", forkCommit)
	}
	if !strings.Contains(forkCommit, "Scott-Nx <158021536+Scott-Nx@users.noreply.github.com>") {
		t.Fatalf("Fork commit missing new identity: %s", forkCommit)
	}
	if !strings.Contains(forkCommit, "-----BEGIN SSH SIGNATURE-----") {
		t.Fatal("Fork commit was not signed with SSH key")
	}
	if strings.Contains(forkCommit, "(#100)") {
		t.Fatalf("Fork commit title (#100) was not stripped: %s", forkCommit)
	}
	if strings.Contains(forkCommit, "Refs:") || strings.Contains(forkCommit, "Reviewed-on:") {
		t.Fatalf("Fork commit body was not stripped: %s", forkCommit)
	}
	if !strings.Contains(forkCommit, "Co-authored-by: New Friend <new@friend.com>") {
		t.Fatalf("Fork commit coauthor trailer was not rewritten: %s", forkCommit)
	}
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
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
