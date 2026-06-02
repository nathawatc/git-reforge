# AGENTS.md

Guidance for AI coding agents working in this repo.

## Project Overview

Go migration tool. Rewrites GitHub commit history → pushes to Codeberg. Replaces `src/main.sh`.
Supports author rewriting, Co-authored-by trailer manipulation, GPG/SSH re-signing,
batch/interactive workflows.

## Build Commands

```bash
# Build binary
go build .

# Run tests
go test ./...

# Run a specific test
go test ./pkg/shell/... -v

# Lint
golangci-lint run
```

## Architecture

```text
main.go                 # CLI entry point — urfave/cli/v3 flags, orchestrates flow
pkg/
  config/config.go      # Config struct, InteractiveState, WorkDir() method
  git/clone.go          # Clone (bare mirror → working), push, save (handles TTY & symlinks)
  logging/logger.go     # slog wrapper with ANSI colors (imports pkg/ui)
  rewrite/batch.go      # RewriteCommitsBatch, RewriteCommitsInteractive, filter generation
  coauthor/batch.go     # RunCoauthorPhase, batch/interactive coauthor filtering
  signing/commit.go     # HandleSigning (keep/none/gpg/ssh), ResignTags
  shell/escape.go       # EscapeShell, EscapeSed, EscapeDoubleQuoted, QuoteSingle — shared shell escaping utilities
  ui/
    colors.go           # ANSI color constants (Red, Green, Yellow, Bold, NC, etc.)
    display.go          # DisplayConfig, ShowSummary
    input.go            # PromptSession — TTY / stdin reader wrapper
    interactive.go      # InteractiveEdit — per-commit TTY menu loop
```

### Execution Flow

1. `main()` → parse args via urfave/cli/v3 → validateArgs → checkDependencies
2. `git.Clone(cfg)` → bare mirror at `$TEMP_DIR/repo.git`, working at `$TEMP_DIR/working`
3. Batch: `rewrite.RewriteCommitsBatch` | Interactive: `ui.InteractiveEdit` → `rewrite.RewriteCommitsInteractive`
4. `coauthor.RunCoauthorPhase`
5. `signing.HandleSigning` → ResignTags
6. `git.Save` or `git.Push` → `ui.ShowSummary`
7. `os.RemoveAll(cfg.TempDir)` on exit

### Key Decisions

| Decision       | Choice                                                                             |
| -------------- | ---------------------------------------------------------------------------------- |
| Git rewriting  | Uses `git filter-branch` via `exec.Command` (not pure go-git plumbing for rewrite) |
| Config state   | Context struct passed as first arg; interactive state in `cfg.State` field         |
| Error handling | Functions return `error`, CLI layer logs and exits with code 1                     |
| Temp dir       | `os.MkdirTemp()` root; `cfg.WorkDir()` returns `$TEMP_DIR/working`                 |
| Logger         | `log/slog` stdlib with ANSI colors via `pkg/ui` constants                          |
| CLI flags      | `urfave/cli/v3` with explicit flag types; `--old-name` uses `StringSliceFlag`      |
| Shell escaping | `pkg/shell/escape.go` — EscapeShell for bash, EscapeSed for sed, etc.              |
| Colors         | Single source: `pkg/ui/colors.go` — all packages import `pkg/ui` for constants     |

### Interactive State

Runtime choices from interactive mode live in `InteractiveState`, accessed via `cfg.State`:

- `cfg.State.CommitOverrides map[plumbing.Hash]CommitOverride` — per-commit name/email/skip
- `cfg.State.CommitMessages map[plumbing.Hash]string` — per-commit message overrides
- `cfg.State.CoauthorActions map[plumbing.Hash]string` — coauthor rewrite/remove/skip decisions

### Signing Modes

| Mode   | Implementation                                                                                      |
| ------ | --------------------------------------------------------------------------------------------------- |
| `keep` | No-op                                                                                               |
| `none` | `git filter-branch` env-filter touching `GIT_COMMITTER_DATE` to recreate objects without signatures |
| `gpg`  | `git filter-branch --commit-filter` with `git commit-tree -S"$KEY"`                                 |
| `ssh`  | `git config gpg.format ssh` + `git filter-branch --commit-filter` with `git commit-tree -S`         |

### Tag Handling

Always uses explicit refspec: `git push --force origin "refs/tags/*:refs/tags/*"` — never `--tags`,
which silently skips annotated tags whose SHA changed after re-signing.

## Common Commands

```bash
# Basic migration
./github-to-codeberg -s https://github.com/user/repo.git -d ssh://git@codeberg.org/user/repo.git -o
"OldName" -e user@noreply.codeberg.org

# Interactive mode
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> -i

# Local-only rewrite
./github-to-codeberg -s <url> -o "OldName" -e <email> --no-push --save-dir /path/to/output

# With SSH signing
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> --sign-mode ssh --ssh-key /path/to/key

# With GPG signing
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> --sign-mode gpg --gpg-key ABCD1234
```
