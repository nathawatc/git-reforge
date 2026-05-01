# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Pure Bash migration script that rewrites GitHub repository commit history and pushes it to Codeberg. Supports author identity rewriting, Co-authored-by trailer manipulation, GPG/SSH re-signing, and batch/interactive workflows.

## Common Commands

```bash
# Run migration
./src/main.sh -s <github-url> -d <codeberg-url> -o "OldName" -e myuser@noreply.codeberg.org

# Interactive mode
./src/main.sh -s <url> -d <url> -o "OldName" -e <email> -i

# Local-only rewrite (no push)
./src/main.sh -s <url> -o "OldName" -e <email> --no-push --save-dir /path/to/output
```

## Architecture

```text
src/
  main.sh              # Entry point — sources libs, registers trap, calls main()
  lib/
    logging.sh         # Color constants + log_* functions (info/success/warning/error/header)
    config.sh          # All global state + associative arrays (COMMIT_OVERRIDES, COMMIT_MESSAGES, etc.) 
    args.sh            # show_help(), parse_args(), validate_args()
    git_ops.sh         # clone_repository(), push_to_destination(), save_repository(), cleanup()
    rewrite.sh         # generate_interactive_filter(), generate_message_filter(), rewrite_commits_batch/interactive()
    coauthor.sh        # interactive_coauthor_edit(), _build_coauthor_msg_filter(), rewrite_coauthors_*()
    signing.sh         # resign_tags(), handle_signing()
    ui.sh              # display_config(), interactive_edit(), show_summary()
```

### Execution Flow (main.sh)

1. Source libs in dependency order: logging → config → args → git_ops → rewrite → coauthor → signing → ui
2. parse_args → validate_args → check_dependencies → display_config
3. clone_repository (bare mirror → working clone)
4. Interactive: interactive_edit → rewrite_commits_interactive
5. Batch: rewrite_commits_batch
6. run_coauthor_phase
7. handle_signing
8. save_repository (if --save-dir) / push_to_destination (if not --no-push)

### Key Patterns

**Git filter-branch rewriting**: Rewrite functions generate temporary shell scripts consumed by `git filter-branch --env-filter` or `--msg-filter`. Scripts are written to `$TEMP_DIR/` and executed via `source` (env-filter) or `bash` (msg-filter).

**Per-commit overrides**: Interactive mode populates `COMMIT_OVERRIDES` (hash → `"name"$'\x1F'"email"`) and `COMMIT_MESSAGES` (hash → message). Rewrite functions emit case-statement filter scripts mapping each commit hash to its override.

**Generated filter script rules**:

- Commit message text: write to `$TEMP_DIR/msg_<hash>.txt` with `printf '%s'`, read back with `cat`
- User strings embedded in heredoc double-quotes: escape `\` → `"` → `$` before interpolation
- Every `case "$GIT_COMMIT" in` block needs a `*) ;;` default arm
- After filter-branch runs exporting helper vars, immediately `unset` them

**Global state**: All mutable state lives in `config.sh`. Interactive state uses associative arrays: `COMMIT_OVERRIDES`, `COMMIT_MESSAGES`, `COMMIT_COAUTHOR_ACTIONS`.

**No default for --old-name**: Fails loudly if omitted. Empty string entries in `OLD_NAMES` are rejected — they would match every commit via exact string comparison.

## Shell Conventions

- Strict mode: `set -euo pipefail` in main.sh
- `printf '%s' "$value"` for data pipelines; `echo -e` only for colored terminal output
- `read -r -p "..." var </dev/tty` for interactive prompts (required for piped stdin)
- `local` declared before use in every function
- Double-sourcing guard in every lib: `[[ -n "${_LIB_*_LOADED:-}" ]] && return 0`
- All failures: `log_error` + `exit 1`

## Signing Modes

| Mode   | Behavior                                                       |
| ------ | -------------------------------------------------------------- |
| `keep` | No filter-branch pass                                          |
| `none` | `--env-filter` + `gpg.sign false` to strip signatures          |
| `gpg`  | `--commit-filter` with `git commit-tree -S"$KEY"`              |
| `ssh`  | `gpg.format ssh` + `--commit-filter` with `git commit-tree -S` |

**Tag pushing**: Always use explicit refspec `git push --force origin "refs/tags/*:refs/tags/*"` — never `--tags`, as `--tags` silently skips annotated tags whose SHA changed after re-signing.

## Co-authored-by Filtering

Uses `sed -E` with pattern `Co-authored-by: <name> <email>` or `Co-authored-by: <name> <[^>]*>` for name-only matching. Rewrite/remove actions applied via `--msg-filter`. Interactive mode stores per-commit decisions in `COMMIT_COAUTHOR_ACTIONS`.
