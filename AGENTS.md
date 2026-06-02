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

# Format code and optimize imports
goimports -w -local github-to-codeberg .

# Run tests
go test ./...

# Run a specific test
go test ./pkg/shell/... -v

# Lint
golangci-lint run
```

## Architecture

```text
main.go                 # CLI entry point — urfave/cli/v3 flags, validation/dependency preflight, user prompt, hands off to migration.Run
pkg/
	config/config.go      # Config struct, InteractiveState, WorkDir() method
	config/validation.go  # ParseOldNames, Validate, CheckDependencies
	migration/run.go      # Run lifecycle + cleanupTempDir; owns clone → rewrite → coauthor → signing → save/push → summary
	migration/run_test.go # cleanupTempDir unit test
	migration/run_integration_test.go # end-to-end pipeline against local repos
	git/clone.go          # Clone (bare mirror → working), push/save, failed-push recovery, temp cleanup
	logging/logger.go     # slog wrapper with ANSI colors (imports pkg/ui)
	rewrite/batch.go      # RewriteCommitsBatch, RewriteCommitsInteractive, filter generation
	coauthor/batch.go     # RunCoauthorPhase, batch/interactive coauthor filtering
	signing/commit.go     # HandleSigning (keep/none/gpg/ssh), ResignTags
	shell/escape.go       # EscapeSed, EscapeSedReplacement, EscapeDoubleQuoted, QuoteSingle — shared shell/sed escaping utilities
	ui/
		colors.go           # ANSI color constants (Red, Green, Yellow, Bold, NC, etc.)
		display.go          # DisplayConfig (preflight prompt + summary)
		input.go            # PromptSession — TTY / stdin reader wrapper
		interactive.go      # InteractiveEdit — per-commit TTY menu loop
```

### Execution Flow

1. `main()` → parse args via urfave/cli/v3 → populate cfg → `config.Validate` → `config.CheckDependencies` → `ui.DisplayConfig` <!-- rumdl-disable-line MD013 -->
2. `migration.Run(cfg)` → `git.Clone(cfg)` → Batch: `rewrite.RewriteCommitsBatch` | Interactive: `ui.InteractiveEdit` → `rewrite.RewriteCommitsInteractive` <!-- rumdl-disable-line MD013 -->
3. `coauthor.RunCoauthorPhase`
4. `signing.HandleSigning` → `ResignTags`
5. `git.Save` or `git.Push` → `ui.ShowSummary`
6. `cleanupTempDir(cfg)` deferred inside `migration.Run`

### Key Decisions

| Decision          | Choice                                                                                                                          |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| Git rewriting     | Uses `git filter-branch` via `exec.Command` (not pure go-git plumbing for rewrite)                                              |
| Config validation | `pkg/config/validation.go` owns `ParseOldNames`, `Validate`, and dependency checks                                              |
| Migration runner  | `pkg/migration/run.go` owns clone → rewrite → coauthor → signing → save/push → summary/cleanup                                  |
| Config state      | Context struct passed as first arg; interactive state in `cfg.State` field                                                      |
| Error handling    | Functions return `error`, CLI layer logs and exits with code 1                                                                  |
| Temp dir          | `os.MkdirTemp()` root; `cfg.WorkDir()` returns `$TEMP_DIR/working`; clone failures self-clean; runner cleans only after success |
| Logger            | `log/slog` stdlib with ANSI colors via `pkg/ui` constants                                                                       |
| CLI flags         | `urfave/cli/v3` with explicit flag types; `--old-name` uses `StringSliceFlag` and `config.ParseOldNames`                        |
| Shell escaping    | `pkg/shell/escape.go`; coauthor sed scripts must shell-quote the full sed program with `QuoteSingle` after sed escaping         |
| Colors            | Single source: `pkg/ui/colors.go` — all packages import `pkg/ui` for constants                                                  |

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

`git.Push` must first check for tag refs and skip the tag push when the repository has no tags.
If tags exist and the explicit tag push is rejected, return an error after offering the push-failure
save/cleanup recovery path.

`signing.ResignTags` processes every annotated tag before returning. Accumulate tag failures with
`errors.Join`, keep individual failure messages starting with `failed to ...`, and let
`HandleSigning` propagate the joined error.

## Common Commands

```bash
# Basic migration
./github-to-codeberg -s https://github.com/user/repo.git -d ssh://git@codeberg.org/user/repo.git -o "OldName" -e user@noreply.codeberg.org

# Interactive mode
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> -i

# Local-only rewrite
./github-to-codeberg -s <url> -o "OldName" -e <email> --no-push --save-dir /path/to/output

# With SSH signing
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> --sign-mode ssh --ssh-key /path/to/key

# With GPG signing
./github-to-codeberg -s <url> -d <url> -o "OldName" -e <email> --sign-mode gpg --gpg-key ABCD1234
```
