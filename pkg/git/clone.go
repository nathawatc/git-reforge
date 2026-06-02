package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github-to-codeberg/pkg/config"
)

func Clone(cfg *config.Config) error {
	log := cfg.Logger

	log.Header("Cloning Repository")
	log.Info("Source: " + cfg.SourceURL)

	tmpDir, err := os.MkdirTemp("", "github-codeberg-migration.")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	cfg.TempDir = tmpDir
	log.Info("Created temporary directory: " + tmpDir)

	// Clone as bare mirror
	log.Info("Cloning as bare mirror...")
	cmd := exec.Command("git", "clone", "--mirror", cfg.SourceURL, tmpDir+"/repo.git")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Convert to working repository
	log.Info("Converting to working repository...")
	err = convertBareToWorking(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to convert mirror to working repository: %w", err)
	}

	log.Success("Repository cloned successfully")
	return nil
}

func convertBareToWorking(tmpDir string) error {
	// Use git clone to convert bare to working
	cmd := exec.Command("git", "clone", tmpDir+"/repo.git", tmpDir+"/working")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Fetch all refs
	cmd = exec.Command("git", "fetch", "--all")
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	if err := createLocalBranchesFromOrigin(tmpDir + "/working"); err != nil {
		return err
	}
	if err := checkoutFirstAvailableBranch(tmpDir + "/working"); err != nil {
		return err
	}

	return nil
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
		if err := cmd.Run(); err != nil {
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
	if err := cmd.Run(); err != nil {
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

// ListCommitsByAuthor returns all commit hashes by the given authors
func ListCommitsByAuthor(tmpDir string, oldNames []string) ([]plumbing.Hash, error) {
	var allHashes []plumbing.Hash

	repo, err := git.PlainOpen(tmpDir + "/working")
	if err != nil {
		return nil, fmt.Errorf("failed to open working repository: %w", err)
	}

	iter, err := repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit iterator: %w", err)
	}

	for {
		commit, err := iter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("commit iteration error: %w", err)
		}

		if slices.Contains(oldNames, commit.Author.Name) {
			allHashes = append(allHashes, commit.Hash)
		}
	}

	return allHashes, nil
}

// ListCommitsWithCoauthor returns commits containing coauthor trailers
func ListCommitsWithCoauthor(tmpDir string, coauthorOldName string, coauthorOldEmail string) ([]plumbing.Hash, error) {
	var matchingHashes []plumbing.Hash

	repo, err := git.PlainOpen(tmpDir + "/working")
	if err != nil {
		return nil, fmt.Errorf("failed to open working repository: %w", err)
	}

	iter, err := repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit iterator: %w", err)
	}

	for {
		commit, err := iter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("commit iteration error: %w", err)
		}

		// Search for Co-authored-by trailers in message
		msg := commit.Message
		if coauthorOldEmail != "" {
			if containsCoauthorWithEmail(msg, coauthorOldName, coauthorOldEmail) {
				matchingHashes = append(matchingHashes, commit.Hash)
			}
		} else {
			if containsCoauthor(msg, coauthorOldName) {
				matchingHashes = append(matchingHashes, commit.Hash)
			}
		}
	}

	return matchingHashes, nil
}

func containsCoauthor(msg string, name string) bool {
	pattern := "Co-authored-by: " + name + " "
	return containsPattern(msg, pattern)
}

func containsCoauthorWithEmail(msg string, name string, email string) bool {
	pattern := "Co-authored-by: " + name + " <" + email + ">"
	return containsPattern(msg, pattern)
}

func containsPattern(msg string, pattern string) bool {
	for line := range strings.SplitSeq(msg, "\n") {
		if strings.HasPrefix(line, pattern) {
			return true
		}
	}
	return false
}

// GetCommitInfo returns author info for a commit
func GetCommitInfo(tmpDir string, hash plumbing.Hash) (authorName, authorEmail, authorDate, subject string, body string, err error) {
	repo, err := git.PlainOpen(tmpDir + "/working")
	if err != nil {
		err = fmt.Errorf("failed to open repository: %w", err)
		return
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		err = fmt.Errorf("failed to get commit: %w", err)
		return
	}

	authorName = commit.Author.Name
	authorEmail = commit.Author.Email
	authorDate = commit.Author.When.Format("2006-01-02 15:04:05 -0700")
	if i := strings.Index(commit.Message, "\n"); i >= 0 {
		subject = commit.Message[:i]
	} else {
		subject = commit.Message
	}
	body = commit.Message

	return
}

// ListAnnotatedTags returns all annotated tag names
func ListAnnotatedTags(tmpDir string) ([]string, error) {
	var tags []string

	cmd := exec.Command("git", "tag")
	cmd.Dir = tmpDir + "/working"
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git tag failed: %w", err)
	}

	for tag := range strings.SplitSeq(string(output), "\n") {
		if tag == "" {
			continue
		}
		// Check if annotated
		cmd = exec.Command("git", "cat-file", "-t", "refs/tags/"+tag)
		cmd.Dir = tmpDir + "/working"
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if string(out) == "tag\n" {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

// Push pushes all branches and tags to the destination
func Push(cfg *config.Config) error {
	tmpDir := cfg.TempDir
	destURL := cfg.DestURL

	cmd := exec.Command("git", "remote", "set-url", "origin", destURL)
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set remote URL: %w", err)
	}

	// Push all branches
	if err := pushBranches(tmpDir); err != nil {
		if saveErr := PromptSaveOrCleanup(cfg, "branch push"); saveErr != nil {
			return saveErr
		}
		return err
	}

	// Push tags with explicit refspec
	cmd = exec.Command("git", "push", "--force", "origin", "refs/tags/*:refs/tags/*")
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		cfg.Logger.Warning("Failed to push some tags (this may be expected if no tags exist)")
	}

	return nil
}

func pushBranches(tmpDir string) error {
	workDir := tmpDir + "/working"

	refspecs, err := branchRefspecs(workDir)
	if err != nil {
		return err
	}
	for _, refspec := range refspecs {
		cmd := exec.Command("git", "push", "--force", "origin", refspec)
		cmd.Dir = workDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to push branch %s: %w", refspec, err)
		}
	}

	return nil
}

func branchRefspecs(workDir string) ([]string, error) {
	refspecsByBranch := make(map[string]string)

	localRefs, err := listRefs(workDir, "refs/heads")
	if err != nil {
		return nil, err
	}
	for _, ref := range localRefs {
		branchName := strings.TrimPrefix(ref, "refs/heads/")
		if branchName == "" || branchName == ref {
			continue
		}
		refspecsByBranch[branchName] = ref + ":refs/heads/" + branchName
	}

	remoteRefs, err := listRefs(workDir, "refs/remotes/origin")
	if err != nil {
		return nil, err
	}
	for _, ref := range remoteRefs {
		if ref == "refs/remotes/origin/HEAD" {
			continue
		}
		branchName := strings.TrimPrefix(ref, "refs/remotes/origin/")
		if branchName == "" || branchName == ref {
			continue
		}
		refspecsByBranch[branchName] = ref + ":refs/heads/" + branchName
	}

	var branchNames []string
	for branchName := range refspecsByBranch {
		branchNames = append(branchNames, branchName)
	}
	slices.Sort(branchNames)

	var refspecs []string
	for _, branchName := range branchNames {
		refspecs = append(refspecs, refspecsByBranch[branchName])
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

func PromptSaveOrCleanup(cfg *config.Config, failedStep string) error {
	if cfg.SaveDir != "" {
		cfg.Logger.Warning("Push failed during " + failedStep + ".")
		cfg.Logger.Info("The rewritten repository will still be saved to: " + cfg.SaveDir)
		return nil
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to push branches and no terminal is available to choose save or cleanup: %w", err)
	}
	defer tty.Close()

	fmt.Fprintln(tty)
	fmt.Fprintf(tty, "The %s to Codeberg failed.\n", failedStep)
	fmt.Fprintln(tty, "The rewritten repository is still available in the temporary directory.")
	fmt.Fprintln(tty)
	fmt.Fprintln(tty, "  [S]ave    - copy the rewritten repository to a local path")
	fmt.Fprintln(tty, "  [C]leanup - discard the temporary directory and exit")
	fmt.Fprintln(tty)

	reader := bufio.NewReader(tty)
	for {
		fmt.Fprint(tty, "  Choice [S/C]: ")
		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read push recovery choice: %w", err)
		}

		switch strings.ToUpper(strings.TrimSpace(choice)) {
		case "S":
			return promptSavePath(cfg, tty, reader)
		case "C":
			cfg.Logger.Info("Cleanup selected. The temporary directory will be removed on exit.")
			return nil
		default:
			cfg.Logger.Warning("Invalid choice. Please enter S to save or C to clean up.")
		}
	}
}

func promptSavePath(cfg *config.Config, tty *os.File, reader *bufio.Reader) error {
	for {
		fmt.Fprint(tty, "  Enter destination path: ")
		savePath, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read save path: %w", err)
		}

		savePath = strings.TrimSpace(savePath)
		if savePath == "" {
			cfg.Logger.Warning("Path cannot be empty. Please enter a valid directory path.")
			continue
		}
		if _, err := os.Stat(savePath); err == nil {
			cfg.Logger.Warning("Path already exists: " + savePath + " - choose a path that does not exist.")
			continue
		}

		cfg.Logger.Info("Saving rewritten repository to: " + savePath)
		if err := Save(cfg.TempDir, savePath); err != nil {
			return fmt.Errorf("failed to save repository to %s: %w", savePath, err)
		}

		cfg.Logger.Success("Repository saved to: " + savePath)
		cfg.Logger.Info("You can push it later with:")
		fmt.Fprintf(tty, "    cd %s\n", savePath)
		fmt.Fprintf(tty, "    git remote set-url origin %s\n", cfg.DestURL)
		fmt.Fprintln(tty, "    git push --all --force")
		fmt.Fprintln(tty, "    git push --force origin \"refs/tags/*:refs/tags/*\"")
		return nil
	}
}

// Save copies the working repository to saveDir
func Save(tmpDir string, saveDir string) error {
	return copyDirRecursive(tmpDir+"/working", saveDir)
}

func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
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
			if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
				return err
			}
			if err := os.Chmod(dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return err
			}
			if err := copyFile(path, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
		return nil
	})
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sf.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := df.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(df, sf); err != nil {
		return err
	}
	if err := df.Chmod(mode); err != nil {
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
