# AGENTS.md

Guidance for AI coding agents working in this repository.

## Project Overview

Go CLI migration tool that clones Git repositories from any forge (GitHub, Codeberg, GitLab,
Forgejo), rewrites commit history (authors, committers, messages, `Co-authored-by` trailers,
signatures), and pushes the rewritten branches and tags to the destination forge.

Replaces the legacy shell implementation (`src/main.sh`).

## Build & Test Commands

```bash
# Build binary
go build .

# Format code and optimize imports
goimports -w -local git-reforge .

# Tidy module dependencies (keeps only github.com/urfave/cli/v3)
go mod tidy

# Run all tests
go test -count=1 ./...

# Run a specific package test with verbose output
go test ./pkg/git/... -v

# Run linter
golangci-lint run

# Check Markdown formatting and line length
rumdl check .

# Snapshot release build
goreleaser build --snapshot --clean
```

## Architecture & Package Responsibilities

```text
main.go                 # CLI entry point (urfave/cli/v3 flags, preflight checks, hands off to migration.Run)
pkg/
	config/
		config.go       # Config struct, Logger interface, nopLogger default, InteractiveState
		validation.go   # ParseAuthor, ParseOldNames, IsLocalPath, Validate, CheckDependencies
	git/
		clone.go        # Clone directly to working, Push, Save, PruneOriginalRefs, CommitDetails
	coauthor/
		batch.go        # RunCoauthorPhase, RewriteBatch, RewriteInteractive, InteractiveCoauthorEdit
	migration/
		run.go          # Run lifecycle + cleanupTempDir (orchestrates clone → rewrite → coauthor → signing → save/push)
	rewrite/
		batch.go        # RewriteCommitsBatch, RewriteCommitsInteractive (git-filter-repo engine)
	shell/
		escape.go       # RunCommand (captures stderr), QuoteSingle
	signing/
		commit.go       # HandleSigning (keep/none/gpg/ssh), ResignTags, tagger metadata preservation
	logging/
		logger.go       # ANSI terminal logger implementing config.Logger (Blue/Green/Yellow/Red/Cyan)
	ui/
		colors.go       # Single Source of Truth for ANSI color constants
		display.go      # DisplayConfig (preflight summary + confirmation), ShowSummary, PromptSaveOrCleanup
		input.go        # PromptSession (TTY / stdin reader wrapper with Close)
		interactive.go  # InteractiveEdit (per-commit interactive review loop)
```

## Execution Flow

1. `main()` initializes `logging.Logger` and `config.New()`, parses positional arguments
   (`<source> [dest]`) and CLI flags via `urfave/cli/v3`.
2. `config.Validate(cfg)` verifies URLs, author requirements, signing mode, and directory existence.
3. `config.CheckDependencies(cfg)` confirms presence of `git`, `git-filter-repo`, and signing
   binaries (`gpg` / `ssh-keygen`).
4. `ui.DisplayConfig(cfg)` displays configuration; if `cfg.Yes` is set, skips confirmation prompt.
5. `migration.Run(cfg)` executes the migration pipeline:
   - `git.Clone(cfg)`: clones repository directly into `$TEMP_DIR/working` and checks out local branches.
   - Author rewrite: `ui.InteractiveEdit(cfg)` + `rewrite.RewriteCommitsInteractive(cfg)` in interactive
     mode; `rewrite.RewriteCommitsBatch(cfg)` in batch mode.
   - Co-author trailers: `coauthor.RunCoauthorPhase(cfg)` (batch or interactive via
     `InteractiveCoauthorEdit`).
   - Signing: `signing.HandleSigning(cfg)` applies commit signing rules, then calls
     `signing.ResignTags(cfg)`.
   - Backup pruning: `git.PruneOriginalRefs(cfg.WorkDir())` removes any `.git/refs/original/` references.
   - Local save: `git.Save(cfg.WorkDir(), cfg.SaveDir)` copies rewritten repo to target directory
     if requested.
   - Remote push: `git.Push(cfg)` pushes all local branches and tags to destination unless
     `--no-push` is set.
   - Summary: `ui.ShowSummary(cfg)` prints operation report.
6. Deferred cleanup: `cleanupTempDir(cfg)` removes `$TEMP_DIR` on success (`retErr == nil`).

## Technical Invariants & Key Decisions

### History Rewriting Engine (`pkg/rewrite`, `pkg/coauthor`, `pkg/signing`)

- **Unified Scoped Filter**: History rewriting executes via an author-scoped single-pass filter.
  - Commits matching migrating author identities have their author, committer, message, trailers,
    and signatures rewritten and signed.
  - Untouched ancestor commits from other authors emit `$GIT_COMMIT` directly, preserving original
    commit hashes, messages, and cryptographic signatures intact.
  - Interactive mode uses `git-filter-repo` commit callbacks with sidecar JSON files.

### Git Commit Metadata SSOT (`pkg/git/clone.go`)

- `git.CommitDetails(workDir, hash) (CommitInfo, error)` is the **Single Source of Truth**
  for extracting commit metadata (`AuthorName`, `AuthorEmail`, `CommitterName`, `CommitterEmail`,
  `AuthorDate`, `Subject`, `Body`).
- Uses a single formatted `git log -1 --format=%an%x00%ae%x00%cn%x00%ce%x00%ai%x00%s%x00%B`
  invocation. Other packages must call `git.CommitDetails` rather than spawning ad-hoc `git log`
  subcommands.

### Interactive State (`pkg/config/config.go`)

Runtime user decisions live in `cfg.State`:

- `CommitOverrides map[string]CommitOverride`: keyed by 40-character commit SHA string.
- `CommitMessages map[string]string`: keyed by 40-character commit SHA string.
- `CoauthorActions map[string]string`: keyed by 40-character commit SHA string
  (`rewrite`, `remove`, `skip`).

No external git library (such as `go-git`) is used; commit hashes are native Go strings.

### Git Remote & Push Semantics (`pkg/git/clone.go`)

- **Origin management**: `git-filter-repo` strips the `origin` remote by default. `git.Push` first tries
  `git remote set-url origin <url>`, and if `origin` does not exist, runs `git remote add origin <url>`.
- **Atomic branch push**: `pushBranches` constructs a single `git push --force origin <refspecs...>`
  command for all branches. It never loops running one push per branch.
- **Local branch precedence**: `branchRefspecs` maps local branch heads (`refs/heads/*`).
- **Tag push**: Uses explicit refspec `git push --force origin "refs/tags/*:refs/tags/*"`.
  - Skipped if the repository contains no tags.
  - Never uses `--tags`, which silently skips annotated tags whose SHA changed after re-signing.
- **Push recovery**: Handled in `migration.Run` via `ui.PromptSaveOrCleanup`: if pushing fails and
  no terminal is attached, returns an error; if a TTY is available, offers an interactive prompt to
  save the rewritten repository locally before exiting.

### Annotated Tag Recreation & Commit Signing (`pkg/signing/commit.go`)

- Re-signing commits (`handleGPG`, `handleSSH`) only signs commits matching migrating author
  identities. Untouched ancestor commits from other authors keep their original SHA and signatures.
- Re-signing tags (`stripTag`, `resignTagGPG`, `resignTagSSH` via `recreateTag`) must preserve the original
  release date and author. Non-matching tags are left untouched.
- Reads tag metadata via `git for-each-ref --format="%(taggername)%00%(taggeremail:trim)%00%(taggerdate:raw)"`.
- Passes `GIT_COMMITTER_NAME`, `GIT_COMMITTER_EMAIL`, and `GIT_COMMITTER_DATE` when executing `git tag`.
- If the original tagger matches an old author name being migrated, applies `cfg.NewName` / `cfg.NewEmail`.

### Directory & File Safety (`pkg/git/clone.go`)

- `copyDirRecursive` creates destination directories with `0755` permissions so child files can always
  be created, then applies original directory permissions in reverse order (deepest child folders first,
  parent folders last). This prevents read-only directories (`0555`) from causing `permission denied`.
- `copyFile` guards deferred close checks so close errors do not mask underlying I/O errors.
- `PruneOriginalRefs` removes `.git/refs/original/` before local export or push to prevent
  backup history leaks.

### Subprocess Execution & Error Handling (`pkg/shell/escape.go`)

- All external commands must use `shell.RunCommand(cmd, description)`.
- `shell.RunCommand` captures `cmd.CombinedOutput()` and formats errors with command stderr output.
- Paths and shell script arguments must be escaped using `shell.QuoteSingle(path)`.
