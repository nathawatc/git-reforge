package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestPushFailsWhenTagPushFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	remoteDir := filepath.Join(tmpDir, "remote.git")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, tmpDir, "init", "--bare", remoteDir)
	writeFile(t, filepath.Join(remoteDir, "hooks", "pre-receive"), `#!/usr/bin/env bash
while read -r _old _new ref; do
  case "$ref" in
	refs/tags/*) exit 1 ;;
  esac
done
exit 0
`)
	if err := os.Chmod(filepath.Join(remoteDir, "hooks", "pre-receive"), 0755); err != nil {
		t.Fatal(err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"), "contents\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "initial")
	head := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))
	runGit(t, workDir, "update-ref", "refs/tags/v1.0.0", head)
	runGit(t, workDir, "remote", "add", "origin", remoteDir)

	cfg := config.New()
	cfg.TempDir = tmpDir
	cfg.DestURL = remoteDir
	cfg.SaveDir = filepath.Join(tmpDir, "saved")
	cfg.Logger = testLogger{}

	err := Push(cfg)
	if err == nil {
		t.Fatal("Push() error = nil want tag push failure")
	}
	if !strings.Contains(err.Error(), "failed to push tags") {
		t.Fatalf("Push() error = %v want failed to push tags", err)
	}

	branches := runGitOutput(t, remoteDir, "for-each-ref", "--format=%(refname)", "refs/heads")
	if branches == "" {
		t.Fatal("remote has no pushed branches")
	}

	tags := runGitOutput(t, remoteDir, "for-each-ref", "--format=%(refname)", "refs/tags")
	if tags != "" {
		t.Fatalf("remote tags = %q want none", tags)
	}
}

func TestCloneCreatesWorkingRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source.git")
	seedDir := filepath.Join(root, "seed")

	runGit(t, root, "init", "--bare", sourceDir)
	runGit(t, root, "clone", sourceDir, seedDir)
	runGit(t, seedDir, "config", "user.name", "Test User")
	runGit(t, seedDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(seedDir, "main.txt"), "main\n")
	runGit(t, seedDir, "add", "main.txt")
	runGit(t, seedDir, "commit", "-m", "main")
	runGit(t, seedDir, "branch", "-M", "main")
	head := strings.TrimSpace(runGitOutput(t, seedDir, "rev-parse", "HEAD"))
	runGit(t, seedDir, "update-ref", "refs/tags/v1.0.0", head)
	runGit(t, seedDir, "push", "origin", "main", "v1.0.0")

	runGit(t, seedDir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(seedDir, "feature.txt"), "feature\n")
	runGit(t, seedDir, "add", "feature.txt")
	runGit(t, seedDir, "commit", "-m", "feature")
	runGit(t, seedDir, "push", "origin", "feature")

	cfg := config.New()
	cfg.SourceURL = sourceDir
	cfg.Logger = testLogger{}

	if err := Clone(cfg); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	tempDir := cfg.TempDir
	t.Cleanup(func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp dir %s: %v", tempDir, err)
		}
	})

	if _, err := os.Stat(filepath.Join(cfg.TempDir, "working", ".git")); err != nil {
		t.Fatalf("working repo missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.TempDir, "repo.git")); !os.IsNotExist(err) {
		t.Fatalf("expected redundant repo.git to not exist, got err = %v", err)
	}

	refs := runGitOutput(t, filepath.Join(cfg.TempDir, "working"), "for-each-ref", "--format=%(refname)")
	for _, ref := range []string{
		"refs/heads/main",
		"refs/heads/feature",
		"refs/remotes/origin/main",
		"refs/remotes/origin/feature",
		"refs/tags/v1.0.0",
	} {
		if !strings.Contains(refs, ref+"\n") {
			t.Fatalf("working refs = %q missing %s", refs, ref)
		}
	}
}

func TestCloneCleansTempDirOnCloneFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	cfg := config.New()
	cfg.SourceURL = filepath.Join(t.TempDir(), "missing.git")
	cfg.Logger = testLogger{}

	err := Clone(cfg)
	if err == nil {
		t.Fatal("Clone() error = nil want clone failure")
	}
	if cfg.TempDir != "" {
		t.Fatalf("TempDir = %q want empty after clone failure", cfg.TempDir)
	}
}

func TestPushSkipsTagPushWhenRepositoryHasNoTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source.git")
	seedDir := filepath.Join(tmpDir, "seed")
	destDir := filepath.Join(tmpDir, "dest.git")

	runGit(t, tmpDir, "init", "--bare", sourceDir)
	runGit(t, tmpDir, "clone", sourceDir, seedDir)
	runGit(t, seedDir, "config", "user.name", "Test User")
	runGit(t, seedDir, "config", "user.email", "test@example.org")

	writeFile(t, filepath.Join(seedDir, "file.txt"), "main\n")
	runGit(t, seedDir, "add", "file.txt")
	runGit(t, seedDir, "commit", "-m", "main")
	runGit(t, seedDir, "branch", "-M", "main")
	runGit(t, seedDir, "push", "origin", "main")

	runGit(t, seedDir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(seedDir, "feature.txt"), "feature\n")
	runGit(t, seedDir, "add", "feature.txt")
	runGit(t, seedDir, "commit", "-m", "feature")
	runGit(t, seedDir, "push", "origin", "feature")

	cfg := config.New()
	cfg.SourceURL = sourceDir
	cfg.DestURL = destDir
	cfg.Logger = testLogger{}

	if err := Clone(cfg); err != nil {
		t.Fatalf("Clone() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(cfg.TempDir) })
	runGit(t, tmpDir, "init", "--bare", destDir)

	if err := Push(cfg); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	branches := runGitOutput(t, destDir, "for-each-ref", "--format=%(refname)", "refs/heads")
	for _, branch := range []string{"refs/heads/main", "refs/heads/feature"} {
		if !strings.Contains(branches, branch+"\n") {
			t.Fatalf("destination branches = %q missing %s", branches, branch)
		}
	}
}

func TestSavePreservesSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	saveDir := filepath.Join(tmpDir, "saved")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	privateDir := filepath.Join(workDir, "private")
	if err := os.Mkdir(privateDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(workDir, "target.txt"), "contents\n")
	writeFile(t, filepath.Join(privateDir, "secret.txt"), "secret\n")
	writeFile(t, filepath.Join(workDir, "..metadata"), "metadata\n")
	executablePath := filepath.Join(workDir, "run.sh")
	writeFile(t, executablePath, "#!/usr/bin/env sh\nexit 0\n")
	if err := os.Chmod(executablePath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", filepath.Join(workDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	if err := Save(tmpDir, saveDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	target, err := os.Readlink(filepath.Join(saveDir, "link.txt"))
	if err != nil {
		t.Fatalf("saved link missing or not symlink: %v", err)
	}
	if target != "target.txt" {
		t.Fatalf("saved link target = %q want target.txt", target)
	}

	info, err := os.Stat(filepath.Join(saveDir, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0755 {
		t.Fatalf("saved executable mode = %o want 755", got)
	}
	if _, err := os.Stat(filepath.Join(saveDir, "..metadata")); err != nil {
		t.Fatalf("valid leading-dot file was not saved: %v", err)
	}
	dirInfo, err := os.Stat(filepath.Join(saveDir, "private"))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0700 {
		t.Fatalf("saved directory mode = %o want 700", got)
	}
}

func TestSaveHandlesReadOnlyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "working")
	saveDir := filepath.Join(tmpDir, "saved")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	roDir := filepath.Join(workDir, "readonly")
	if err := os.Mkdir(roDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(roDir, "nested.txt"), "nested content\n")
	if err := os.Chmod(roDir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(roDir, 0755)
		_ = os.Chmod(filepath.Join(saveDir, "readonly"), 0755)
	})

	if err := Save(tmpDir, saveDir); err != nil {
		t.Fatalf("Save() error = %v want nil", err)
	}

	content, err := os.ReadFile(filepath.Join(saveDir, "readonly", "nested.txt"))
	if err != nil {
		t.Fatalf("failed to read copied file in readonly dir: %v", err)
	}
	if string(content) != "nested content\n" {
		t.Fatalf("file content = %q want 'nested content\\n'", string(content))
	}

	info, err := os.Stat(filepath.Join(saveDir, "readonly"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0555 {
		t.Fatalf("saved readonly directory mode = %o want 555", got)
	}
}

func TestSaveWhenRepoContainsWorkingSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "working")
	if err := os.Mkdir(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "init")
	subWorking := filepath.Join(repoDir, "working")
	if err := os.Mkdir(subWorking, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(subWorking, "inner.txt"), "inner\n")
	writeFile(t, filepath.Join(repoDir, "root.txt"), "root\n")

	saveDir := filepath.Join(tmpDir, "saved")
	if err := Save(repoDir, saveDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(saveDir, "root.txt")); err != nil {
		t.Errorf("Save() missed root.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(saveDir, ".git")); err != nil {
		t.Errorf("Save() missed .git: %v", err)
	}
}

func TestPruneOriginalRefsRemovesBackupRefs(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workDir := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"), "hello\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "init")
	head := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))
	runGit(t, workDir, "update-ref", "refs/original/refs/heads/main", head)

	refsBefore, err := listRefs(workDir, "refs/original")
	if err != nil || len(refsBefore) == 0 {
		t.Fatalf("expected refs/original to exist, got %v (err: %v)", refsBefore, err)
	}

	if err := PruneOriginalRefs(workDir); err != nil {
		t.Fatalf("PruneOriginalRefs() error = %v", err)
	}

	refsAfter, err := listRefs(workDir, "refs/original")
	if err != nil {
		t.Fatalf("listRefs() after prune error = %v", err)
	}
	if len(refsAfter) != 0 {
		t.Fatalf("refs/original still exists after prune: %v", refsAfter)
	}
}

func TestBranchRefspecsPrefersLocalBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workDir := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	writeFile(t, filepath.Join(workDir, "file.txt"), "hello\n")
	runGit(t, workDir, "add", "file.txt")
	runGit(t, workDir, "commit", "-m", "init")
	runGit(t, workDir, "branch", "-M", "main")
	head := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))
	runGit(t, workDir, "update-ref", "refs/remotes/origin/main", head)

	refspecs, err := branchRefspecs(workDir)
	if err != nil {
		t.Fatalf("branchRefspecs() error = %v", err)
	}
	for _, refspec := range refspecs {
		if strings.HasPrefix(refspec, "refs/remotes/origin/main:") {
			t.Fatalf("branchRefspecs returned remote refspec %q; want local refs/heads/main", refspec)
		}
	}
	if !slices.Contains(refspecs, "refs/heads/main:refs/heads/main") {
		t.Fatalf("branchRefspecs %v does not contain local refs/heads/main:refs/heads/main", refspecs)
	}
}

func TestPushBranchesPushesMultipleBranches(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	destDir := filepath.Join(root, "dest.git")
	runGit(t, root, "init", "--bare", destDir)

	workDir := filepath.Join(root, "working")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.org")
	runGit(t, workDir, "remote", "add", "origin", destDir)

	writeFile(t, filepath.Join(workDir, "f1.txt"), "main\n")
	runGit(t, workDir, "add", "f1.txt")
	runGit(t, workDir, "commit", "-m", "main commit")
	runGit(t, workDir, "branch", "-M", "main")

	runGit(t, workDir, "checkout", "-b", "feature-a")
	writeFile(t, filepath.Join(workDir, "f2.txt"), "a\n")
	runGit(t, workDir, "add", "f2.txt")
	runGit(t, workDir, "commit", "-m", "feature-a commit")

	runGit(t, workDir, "checkout", "-b", "feature-b")
	writeFile(t, filepath.Join(workDir, "f3.txt"), "b\n")
	runGit(t, workDir, "add", "f3.txt")
	runGit(t, workDir, "commit", "-m", "feature-b commit")

	if err := pushBranches(workDir); err != nil {
		t.Fatalf("pushBranches() error = %v", err)
	}

	remoteBranches := runGitOutput(t, destDir, "for-each-ref", "--format=%(refname)", "refs/heads")
	for _, want := range []string{"refs/heads/main", "refs/heads/feature-a", "refs/heads/feature-b"} {
		if !strings.Contains(remoteBranches, want) {
			t.Fatalf("remote branches %q missing %q", remoteBranches, want)
		}
	}
}

func TestCommitDetailsExtractsAllFields(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workDir := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(workDir, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Commit Author")
	runGit(t, workDir, "config", "user.email", "author@example.org")
	writeFile(t, filepath.Join(workDir, "f.txt"), "hello\n")
	runGit(t, workDir, "add", "f.txt")
	runGit(t, workDir, "commit", "-m", "Subject Line\n\nBody line 1\nBody line 2\n")
	head := strings.TrimSpace(runGitOutput(t, workDir, "rev-parse", "HEAD"))

	info, err := CommitDetails(workDir, head)
	if err != nil {
		t.Fatalf("CommitDetails() error = %v", err)
	}
	if info.AuthorName != "Commit Author" {
		t.Errorf("AuthorName = %q want 'Commit Author'", info.AuthorName)
	}
	if info.AuthorEmail != "author@example.org" {
		t.Errorf("AuthorEmail = %q want 'author@example.org'", info.AuthorEmail)
	}
	if info.CommitterName != "Commit Author" {
		t.Errorf("CommitterName = %q want 'Commit Author'", info.CommitterName)
	}
	if info.CommitterEmail != "author@example.org" {
		t.Errorf("CommitterEmail = %q want 'author@example.org'", info.CommitterEmail)
	}
	if info.Subject != "Subject Line" {
		t.Errorf("Subject = %q want 'Subject Line'", info.Subject)
	}
	if !strings.Contains(info.Body, "Body line 1") {
		t.Errorf("Body = %q want to contain 'Body line 1'", info.Body)
	}
}

func TestRepoObjectFormatAndHashAlgorithmDetection(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	sha1Dir := filepath.Join(root, "sha1_repo")
	runGit(t, root, "init", "--object-format=sha1", sha1Dir)
	if got := RepoObjectFormat(sha1Dir); got != "sha1" {
		t.Errorf("RepoObjectFormat(sha1Dir) = %q want sha1", got)
	}

	sha256Dir := filepath.Join(root, "sha256_repo")
	runGit(t, root, "init", "--object-format=sha256", sha256Dir)
	if got := RepoObjectFormat(sha256Dir); got != "sha256" {
		t.Errorf("RepoObjectFormat(sha256Dir) = %q want sha256", got)
	}

	err := fmt.Errorf("fatal: the receiving end does not support this repository's hash algorithm")
	if !isHashAlgorithmError(err) {
		t.Error("isHashAlgorithmError(err) = false want true")
	}

	formattedErr := formatHashAlgorithmError("https://codeberg.org/user/repo.git", "sha256", "sha1")
	if !strings.Contains(formattedErr.Error(), "does not support hash algorithm sha256") {
		t.Errorf("formatHashAlgorithmError() missing error message: %v", formattedErr)
	}
	if !strings.Contains(formattedErr.Error(), "fast-export --all") {
		t.Errorf("formatHashAlgorithmError() missing fast-export command: %v", formattedErr)
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
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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
