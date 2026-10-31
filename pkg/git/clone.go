package git

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"git-reforge/pkg/config"
	"git-reforge/pkg/shell"
)

func Clone(cfg *config.Config) error {
	log := cfg.Logger

	log.Header("Cloning Repository")
	log.Info("Source: " + cfg.SourceURL)

	tmpDir, err := os.MkdirTemp("", "git-reforge-migration.")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	cfg.TempDir = tmpDir
	cloneSucceeded := false
	defer func() {
		if !cloneSucceeded {
			if err := os.RemoveAll(tmpDir); err != nil && log != nil {
				log.Warning("Failed to clean up failed clone temp directory: " + tmpDir)
			}
			cfg.TempDir = ""
		}
	}()
	log.Info("Created temporary directory: " + tmpDir)

	workDir := cfg.WorkDir()
	log.Info("Cloning repository into working directory...")
	cmd := exec.Command("git", "clone", cfg.SourceURL, workDir)
	if err := shell.RunCommand(cmd, "git clone"); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if err := createLocalBranchesFromOrigin(workDir); err != nil {
		return err
	}
	if err := checkoutFirstAvailableBranch(workDir); err != nil {
		return err
	}

	log.Success("Repository cloned successfully")
	cloneSucceeded = true
	return nil
}

type CommitInfo struct {
	Hash           string
	AuthorName     string
	AuthorEmail    string
	CommitterName  string
	CommitterEmail string
	AuthorDate     string
	Subject        string
	Body           string
}

// CommitDetails extracts metadata for a single commit
func CommitDetails(workDir string, hash string) (CommitInfo, error) {
	cmd := exec.Command("git", "log", "-1", "--format=%an%x00%ae%x00%cn%x00%ce%x00%ai%x00%s%x00%B", hash)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommitInfo{Hash: hash}, fmt.Errorf("git log failed for commit %s: %s: %w", hash, strings.TrimSpace(string(output)), err)
	}
	parts := strings.SplitN(string(output), "\x00", 7)
	if len(parts) < 7 {
		return CommitInfo{Hash: hash}, fmt.Errorf("malformed commit metadata for %s", hash)
	}
	info := CommitInfo{
		Hash:           hash,
		AuthorName:     strings.TrimSpace(parts[0]),
		AuthorEmail:    strings.TrimSpace(parts[1]),
		CommitterName:  strings.TrimSpace(parts[2]),
		CommitterEmail: strings.TrimSpace(parts[3]),
		AuthorDate:     strings.TrimSpace(parts[4]),
		Subject:        strings.TrimSpace(parts[5]),
		Body:           parts[6],
	}
	return info, nil
}

func createLocalBranchesFromOrigin(workDir string) error {
	current := currentBranchName(workDir)
	refs, err := listRefs(workDir, "refs/remotes/origin")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if ref == "refs/remotes/origin/HEAD" {
			continue
		}
		branchName := strings.TrimPrefix(ref, "refs/remotes/origin/")
		if branchName == "" || branchName == ref {
			continue
		}
		if branchName == current {
			continue
		}
		cmd := exec.Command("git", "branch", "--force", branchName, ref)
		cmd.Dir = workDir
		if err := shell.RunCommand(cmd, "create branch "+branchName); err != nil {
			return fmt.Errorf("failed to create local branch %s from %s: %w", branchName, ref, err)
		}
	}
	return nil
}

func currentBranchName(workDir string) string {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func checkoutFirstAvailableBranch(workDir string) error {
	branchName, err := defaultBranchName(workDir)
	if err != nil {
		return err
	}
	if branchName == "" {
		return nil
	}
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = workDir
	if err := shell.RunCommand(cmd, "checkout branch "+branchName); err != nil {
		return fmt.Errorf("failed to check out local branch %s: %w", branchName, err)
	}
	return nil
}

func defaultBranchName(workDir string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "-q", "refs/remotes/origin/HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		branchName := strings.TrimPrefix(ref, "refs/remotes/origin/")
		if branchName != "" && branchName != ref && branchName != "HEAD" {
			return branchName, nil
		}
	}

	refs, err := listRefs(workDir, "refs/heads")
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return "", nil
	}
	slices.Sort(refs)
	branchName := strings.TrimPrefix(refs[0], "refs/heads/")
	if branchName == refs[0] {
		return "", nil
	}
	return branchName, nil
}

// Push pushes all branches and tags to the destination
func Push(cfg *config.Config) error {
	workDir := cfg.WorkDir()
	destURL := cfg.DestURL

	cmd := exec.Command("git", "remote", "set-url", "origin", destURL)
	cmd.Dir = workDir
	if err := shell.RunCommand(cmd, "set remote origin URL"); err != nil {
		cmd = exec.Command("git", "remote", "add", "origin", destURL)
		cmd.Dir = workDir
		if err := shell.RunCommand(cmd, "add remote origin"); err != nil {
			return fmt.Errorf("failed to set remote URL: %w", err)
		}
	}

	// Push all branches
	if err := pushBranches(workDir); err != nil {
		if IsHashAlgorithmError(err) {
			return FormatHashAlgorithmError(cfg.SourceURL, RepoObjectFormat(workDir), "sha1")
		}
		return fmt.Errorf("failed to push branches: %w", err)
	}

	// Push tags with explicit refspec
	if err := pushTags(workDir); err != nil {
		if IsHashAlgorithmError(err) {
			return FormatHashAlgorithmError(cfg.SourceURL, RepoObjectFormat(workDir), "sha1")
		}
		return fmt.Errorf("failed to push tags: %w", err)
	}

	return nil
}

// ConvertToSHA1 converts the repository in workDir from sha256 to sha1 in place.
func ConvertToSHA1(cfg *config.Config) error {
	log := cfg.Logger
	workDir := cfg.WorkDir()
	log.Info("Converting repository from sha256 to sha1...")

	sha1Temp := filepath.Join(cfg.TempDir, "working_sha1")
	cmd := exec.Command("git", "init", "--object-format=sha1", sha1Temp)
	if err := shell.RunCommand(cmd, "init sha1 repository"); err != nil {
		return fmt.Errorf("failed to initialize sha1 repository: %w", err)
	}

	exportCmd := exec.Command("git", "fast-export", "--all", "--signed-tags=strip", "--tag-of-filtered-object=rewrite")
	exportCmd.Dir = workDir
	importCmd := exec.Command("git", "fast-import")
	importCmd.Dir = sha1Temp

	reader, writer := io.Pipe()
	exportCmd.Stdout = writer
	importCmd.Stdin = reader

	var exportErr, importErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() {
			_ = writer.Close()
		}()
		exportErr = exportCmd.Run()
	}()

	go func() {
		defer wg.Done()
		importErr = importCmd.Run()
	}()

	wg.Wait()
	if exportErr != nil {
		return fmt.Errorf("fast-export failed during sha1 conversion: %w", exportErr)
	}
	if importErr != nil {
		return fmt.Errorf("fast-import failed during sha1 conversion: %w", importErr)
	}

	currBranch := currentBranchName(workDir)
	if currBranch == "" {
		currBranch = "main"
	}
	checkoutCmd := exec.Command("git", "checkout", currBranch)
	checkoutCmd.Dir = sha1Temp
	_ = checkoutCmd.Run()

	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to remove sha256 working directory: %w", err)
	}
	if err := os.Rename(sha1Temp, workDir); err != nil {
		return fmt.Errorf("failed to replace working directory with sha1: %w", err)
	}

	log.Success("Repository successfully converted from sha256 to sha1")
	return nil
}

// RepoObjectFormat returns the hash algorithm used by the repository ("sha1" or "sha256").
func RepoObjectFormat(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "--show-object-format")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "sha1"
	}
	return strings.TrimSpace(string(output))
}

func IsHashAlgorithmError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not support this repository's hash algorithm") ||
		strings.Contains(msg, "repository's hash algorithm")
}

func FormatHashAlgorithmError(sourceURL, srcAlgo, targetAlgo string) error {
	return fmt.Errorf(
		"destination repository does not support hash algorithm %s (e.g. GitHub requires %s)\n\n"+
			"To convert your repository from %s to %s before migrating:\n"+
			"  1. git clone %s repo-%s\n"+
			"  2. git init --object-format=%s repo-%s\n"+
			"  3. git -C repo-%s fast-export --all --signed-tags=strip --tag-of-filtered-object=rewrite | git -C repo-%s fast-import\n"+
			"  4. git -C repo-%s checkout main\n"+
			"  5. Run git-reforge on repo-%s",
		srcAlgo, targetAlgo, srcAlgo, targetAlgo,
		sourceURL, srcAlgo, targetAlgo, targetAlgo,
		srcAlgo, targetAlgo, targetAlgo, targetAlgo,
	)
}

func pushBranches(workDir string) error {
	refspecs, err := branchRefspecs(workDir)
	if err != nil {
		return err
	}
	if len(refspecs) == 0 {
		return nil
	}

	args := append([]string{"push", "--force", "origin"}, refspecs...)
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	if err := shell.RunCommand(cmd, "push branches"); err != nil {
		return fmt.Errorf("failed to push branches: %w", err)
	}

	return nil
}

func pushTags(workDir string) error {

	tagRefs, err := listRefs(workDir, "refs/tags")
	if err != nil {
		return err
	}
	if len(tagRefs) == 0 {
		return nil
	}

	cmd := exec.Command("git", "push", "--force", "origin", "refs/tags/*:refs/tags/*")
	cmd.Dir = workDir
	if err := shell.RunCommand(cmd, "push tags"); err != nil {
		return fmt.Errorf("failed to push tags: %w", err)
	}
	return nil
}

func branchRefspecs(workDir string) ([]string, error) {
	localRefs, err := listRefs(workDir, "refs/heads")
	if err != nil {
		return nil, err
	}
	slices.Sort(localRefs)

	var refspecs []string
	for _, ref := range localRefs {
		branchName := strings.TrimPrefix(ref, "refs/heads/")
		if branchName == "" || branchName == ref {
			continue
		}
		refspecs = append(refspecs, ref+":refs/heads/"+branchName)
	}
	return refspecs, nil
}

func listRefs(workDir string, namespace string) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname)", namespace)
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list %s refs: %w", namespace, err)
	}

	var refs []string
	for ref := range strings.SplitSeq(string(output), "\n") {
		if ref == "" {
			continue
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// Save copies the working repository to saveDir
func Save(sourceDir string, saveDir string) error {
	workDir := sourceDir
	if _, err := os.Stat(filepath.Join(sourceDir, ".git")); os.IsNotExist(err) {
		if fi, err := os.Stat(filepath.Join(sourceDir, "working")); err == nil && fi.IsDir() {
			workDir = filepath.Join(sourceDir, "working")
		}
	}
	if err := PruneOriginalRefs(workDir); err != nil {
		return err
	}
	return copyDirRecursive(workDir, saveDir)
}

// PruneOriginalRefs removes refs/original references left behind by filter-branch
func PruneOriginalRefs(workDir string) error {
	if _, err := os.Stat(filepath.Join(workDir, ".git")); os.IsNotExist(err) {
		return nil
	}
	refs, err := listRefs(workDir, "refs/original")
	if err != nil {
		return err
	}
	for _, ref := range refs {
		cmd := exec.Command("git", "update-ref", "-d", ref)
		cmd.Dir = workDir
		_ = cmd.Run()
	}
	_ = os.RemoveAll(filepath.Join(workDir, ".git", "refs", "original"))
	return nil
}

func copyDirRecursive(src, dst string) error {
	var dirModes []struct {
		path string
		mode os.FileMode
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Prevent writing outside dst tree
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return copySymlink(path, dstPath)
		}
		if info.IsDir() {
			// Create directory with write permissions so child files can be written
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			dirModes = append(dirModes, struct {
				path string
				mode os.FileMode
			}{path: dstPath, mode: info.Mode().Perm()})
		} else if info.Mode().IsRegular() {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return err
			}
			if err := copyFile(path, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Apply final directory permissions in reverse order (children before parents)
	for _, dm := range slices.Backward(dirModes) {
		if err := os.Chmod(dm.path, dm.mode); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sf.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := df.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err = io.Copy(df, sf); err != nil {
		return err
	}
	if err = df.Chmod(mode); err != nil {
		return err
	}
	return df.Sync()
}

func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("read symlink %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Symlink(target, dst)
}
