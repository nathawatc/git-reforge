package git

import (
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
	_, err = git.PlainClone(tmpDir+"/repo.git", true, &git.CloneOptions{
		URL: cfg.SourceURL,
	})
	if err != nil {
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

	return nil
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
func Push(tmpDir string, destURL string) error {
	cmd := exec.Command("git", "remote", "set-url", "origin", destURL)
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set remote URL: %w", err)
	}

	// Push all branches
	cmd = exec.Command("git", "push", "--all", "--force")
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push branches: %w", err)
	}

	// Push tags with explicit refspec
	cmd = exec.Command("git", "push", "--force", "origin", "refs/tags/*:refs/tags/*")
	cmd.Dir = tmpDir + "/working"
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tags: %w", err)
	}

	return nil
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
		// Skip symlinks to prevent traversal outside src tree
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Prevent writing outside dst tree
		if strings.HasPrefix(rel, "..") {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return err
			}
			if err := copyFile(path, dstPath); err != nil {
				return err
			}
		}
		return nil
	})
}

func copyFile(src, dst string) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sf.Close(); closeErr != nil {
			err = closeErr
		}
	}()

	df, err := os.Create(dst)
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
	return df.Sync()
}
