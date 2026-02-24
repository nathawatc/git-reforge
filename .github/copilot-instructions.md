# GitHub Copilot Instructions — github-to-codeberg

## Project Overview

This is a pure Bash migration script that rewrites a GitHub repository's commit
history and pushes it to Codeberg. The tool supports author identity rewriting,
Co-authored-by trailer manipulation, GPG/SSH re-signing, and both batch and
interactive workflows.

## Repository Layout

```
src/
  main.sh              # Entry point — sources all libs, registers trap, calls main()
  lib/
    logging.sh         # Color constants (RED, GREEN, …, NC, BOLD) and log_* functions
    config.sh          # All global state variables and associative array declarations
    args.sh            # show_help(), parse_args(), validate_args()
    git_ops.sh         # cleanup(), check_dependencies(), clone_repository(), push_to_destination()
    rewrite.sh         # generate_interactive_filter(), generate_message_filter(),
                       # rewrite_commits_batch(), rewrite_commits_interactive()
    coauthor.sh        # interactive_coauthor_edit(), _build_coauthor_msg_filter(),
                       # rewrite_coauthors_batch(), rewrite_coauthors_interactive(),
                       # run_coauthor_phase()
    signing.sh         # resign_tags(), handle_signing()
    ui.sh              # display_config(), interactive_edit(), show_summary()
```

## Language & Shell Conventions

- **Shell**: Bash only (`#!/usr/bin/env bash`). Do not introduce Python, Ruby, or
  any other runtime dependency.
- **Strict mode**: `set -euo pipefail` is set in `main.sh`. Every function must
  be safe under these flags.
- **Variable safety**: Always quote variable expansions (`"$VAR"`). Never rely on
  word-splitting.
- **printf over echo**: Use `printf '%s' "$value"` instead of `echo "$value"` when
  piping or capturing output to avoid appended-newline side effects. `echo -e` is
  reserved exclusively for terminal display lines (color output).
- **No `echo -e` for data pipelines**: Only `echo -e` for colored user-facing
  output; all data piping uses `printf '%s'`.
- **Readonly guards**: Every lib file starts with a double-sourcing guard using a
  `readonly _LIB_*_LOADED` variable. New lib files must follow the same pattern.
- **Local variables**: Declare all function-scoped variables with `local` at the
  top of the function before use.
- **Error messages**: Use `log_error` for all failures. Fatal paths must call
  `exit 1` explicitly.

## Global State

All mutable state lives in `config.sh`. Do not declare new global variables
anywhere else.

| Variable                  | Type        | Purpose                                          |
| ------------------------- | ----------- | ------------------------------------------------ |
| `OLD_NAMES`               | array       | Author names to rewrite                          |
| `NEW_NAME`                | string      | Replacement author name                          |
| `NEW_EMAIL`               | string      | Replacement author email                         |
| `SOURCE_URL`              | string      | GitHub clone URL                                 |
| `DEST_URL`                | string      | Codeberg push URL                                |
| `INTERACTIVE`             | bool string | `true` / `false`                                 |
| `SIGN_MODE`               | string      | `keep` / `gpg` / `ssh` / `none`                  |
| `GPG_KEY`                 | string      | GPG key ID (gpg mode only)                       |
| `SSH_KEY`                 | string      | SSH key path (ssh mode only)                     |
| `TEMP_DIR`                | string      | mktemp directory; cleaned up by `cleanup()` trap |
| `COAUTHOR_MODE`           | string      | `rewrite` / `remove` / `interactive` / empty     |
| `COAUTHOR_OLD_NAME`       | string      | Name to match in trailers                        |
| `COAUTHOR_OLD_EMAIL`      | string      | Optional email to narrow trailer match           |
| `COAUTHOR_NEW_NAME`       | string      | Replacement trailer name                         |
| `COAUTHOR_NEW_EMAIL`      | string      | Replacement trailer email                        |
| `COMMIT_OVERRIDES`        | assoc array | hash → `"name\|email"` or `"SKIP"`               |
| `COMMIT_MESSAGES`         | assoc array | hash → custom commit message string              |
| `COMMIT_COAUTHOR_ACTIONS` | assoc array | hash → `rewrite` / `remove` / `skip`             |

## Logging

Use only the four logging functions defined in `logging.sh`. Never use raw `echo`
for status output.

```bash
log_info    "..."   # blue  [INFO]
log_success "..."   # green [SUCCESS]
log_warning "..."   # yellow [WARNING]
log_error   "..."   # red   [ERROR]  — writes to stderr
log_header  "..."   # bold cyan section banner
```

## Interactive Input

All `read` calls that accept user input **must** redirect from `/dev/tty` so they
work correctly when stdin is piped:

```bash
read -r -p "Prompt: " variable </dev/tty
```

The confirmation prompt in `display_config` also follows this rule.

## Generated Filter Scripts

`rewrite.sh` and `coauthor.sh` generate temporary shell scripts consumed by
`git filter-branch`. Follow these rules when writing or modifying them:

1. **Sidecar files for message content**: Never inline commit message text into a
   heredoc. Write it to `$TEMP_DIR/msg_<hash>.txt` with `printf '%s'` and read it
   back with `cat` inside the generated script.
2. **Escape before embedding**: Any user-supplied string (name, email) embedded
   inside a double-quoted export line in a heredoc must have `\`, `"`, and `$`
   escaped in that order before interpolation.
3. **Always include a default case arm**: Every `case "$GIT_COMMIT" in` block in a
   generated script must end with a `*) ;;` arm.
4. **sed -E for coauthor patterns**: The coauthor filter uses `[^>]*` which
   requires ERE. Always emit `sed -E` in generated coauthor filter scripts.
5. **Consistent invoker**: `--env-filter` scripts are invoked with `source`;
   `--msg-filter` scripts are invoked with `bash`.

## Signing Rules

- `keep` mode: no filter-branch pass at all.
- `none` mode: use `--env-filter` with `export GIT_COMMITTER_DATE="$GIT_COMMITTER_DATE"`
  after setting `gpg.sign false`. Clean up with `git config --unset` afterwards.
- `gpg` mode: set `user.signingkey`, run `--commit-filter` with `git commit-tree -S"$KEY"`,
  then `--unset user.signingkey`.
- `ssh` mode: set `gpg.format ssh` and `user.signingkey`, run `--commit-filter`
  with `git commit-tree -S`, then `--unset` both.
- `resign_tags`: use `%(contents:subject)` and `%(contents:body)` — never
  `%(contents)` — to avoid embedding stale signature blocks in the new tag message.

## Tag Pushing

Always push tags with the explicit refspec to correctly handle re-signed annotated
tags whose object SHA has changed:

```bash
git push --force origin "refs/tags/*:refs/tags/*"
```

Never use `git push --tags` for this project.

## Argument Validation Rules

- `--source`, `--dest`, and `--new-email` are required. Validation is in
  `validate_args()`.
- `--old-name` (`-o`) is required. There is no default. Omitting it must produce
  a `log_error` and `exit 1`.
- `--new-name` (`-n`) has no default. Omitting it results in an empty `NEW_NAME`.
  This is intentional for users who only need email rewriting.
- `--sign-mode` values are validated inline during `parse_args()`.
- `--coauthor-mode` values are validated inline during `parse_args()`.
- SSH key path existence is validated in `validate_args()`.
- GPG key availability is validated in `check_dependencies()`.

## Commit Message Style

Use conventional commits:

```
<type>(<scope>): <short summary>

<body — wrap at 72 chars>
```

Types: `feat`, `fix`, `refactor`, `docs`, `chore`  
Scopes map to file names without extension: `main`, `args`, `config`, `git_ops`,
`rewrite`, `coauthor`, `signing`, `ui`, `logging`

## What NOT to Do

- Do not add a default value for `--old-name`. Failing loudly is intentional.
- Do not use `git push --tags`. Use the explicit refspec.
- Do not use `%(contents)` for tag messages in `resign_tags`.
- Do not use `echo "$var" | cmd` for data pipelines. Use `printf '%s' "$var" | cmd`.
- Do not add new global variables outside `config.sh`.
- Do not use `echo -e` for anything other than colored terminal output.
- Do not use `cd` inside a function without scoping it to a subshell.
- Do not hardcode any personal names, email addresses, or GitHub user IDs anywhere
  in source code, help text, or examples.
- Do not leave `git config` entries (signingkey, gpg.format, gpg.sign) set after
  a filter-branch run. Always clean up with `git config --unset`.
