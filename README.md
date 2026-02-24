# GitHub To Codeberg

A powerful tool for migrating repositories from GitHub to Codeberg with commit history rewriting capabilities.

## Features

- **Repository Migration** - Clone and push repositories from GitHub to Codeberg
- **Author Rewriting** - Change author name and email across all commits
- **Multiple Author Support** - Rewrite commits from multiple old authors
- **Interactive Mode** - Review and customize changes per-commit:
  - Edit author name and email individually
  - Edit commit messages
  - Skip specific commits
  - Apply defaults to remaining commits
- **Co-authored-by Trailer Rewriting** - Rewrite, remove, or interactively review `Co-authored-by` trailers
- **Commit Signing** - GPG and SSH signing options:
  - Keep original signatures
  - Re-sign with GPG key
  - Re-sign with SSH key
  - Strip all signatures
- **Local Save** - Save the rewritten repository locally without pushing
- **Preserve History** - Original commit dates are preserved

## Requirements

- Git
- Bash 4.0+ (for associative arrays)
- GPG (optional, for GPG signing)
- SSH (optional, for SSH signing)
- A GitHub account with repositories to migrate
- A Codeberg account

## Installation

```bash
git clone https://codeberg.org/nathawat/gitHub-to-codeberg.git
cd gitHub-to-codeberg
chmod +x src/main.sh
```

## Usage

```bash
./src/main.sh [OPTIONS]
```

### Required Options

| Option                    | Description                                                                      |
| ------------------------- | -------------------------------------------------------------------------------- |
| `-s, --source <url>`      | GitHub repository URL                                                            |
| `-d, --dest <url>`        | Codeberg repository URL (not required when `--no-push` or `--save-dir` is given) |
| `-e, --new-email <email>` | New author email                                                                 |

### Optional Options

| Option                  | Description                                                                                            |
| ----------------------- | ------------------------------------------------------------------------------------------------------ |
| `-n, --new-name <name>` | New author name                                                                                        |
| `-o, --old-name <name>` | Old author name(s) to replace (required; can be specified multiple times or as comma-separated values) |
| `-i, --interactive`     | Enable interactive mode for per-commit editing                                                         |
| `--sign-mode <mode>`    | Signing mode: `keep\|gpg\|ssh\|none` (default: `keep`)                                                 |
| `--gpg-key <keyid>`     | GPG key ID for re-signing (required if `--sign-mode=gpg`)                                              |
| `--ssh-key <path>`      | SSH key path for re-signing (required if `--sign-mode=ssh`)                                            |
| `--nosign`              | Shorthand for `--sign-mode=none`                                                                       |
| `--save-dir <path>`     | Save the rewritten repository to `<path>` (directory must not already exist)                           |
| `--no-push`             | Rewrite history locally but skip pushing to Codeberg                                                   |
| `-h, --help`            | Show help message                                                                                      |

### Co-authored-by Options

| Option                         | Description                                                              |
| ------------------------------ | ------------------------------------------------------------------------ |
| `--coauthor-mode <mode>`       | Trailer mode: `rewrite\|remove\|interactive`                             |
| `--coauthor-old <name>`        | Name to match in `Co-authored-by` trailers (default: first `--old-name`) |
| `--coauthor-old-email <email>` | Email to narrow trailer match (optional)                                 |
| `--coauthor-new-name <name>`   | Replacement trailer name (default: `--new-name` value)                   |
| `--coauthor-new-email <email>` | Replacement trailer email (default: `--new-email` value)                 |

### Examples

<details>
<summary>Click to expand examples</summary>

**Basic migration:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" \
              -e myuser@noreply.codeberg.org
```

**Interactive mode with custom author:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" -n "NewName" \
              -e newuser@noreply.codeberg.org \
              -i
```

**With GPG re-signing:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" \
              -e myuser@noreply.codeberg.org \
              --sign-mode gpg --gpg-key ABCD1234
```

**Multiple old author names:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName1" -o "OldName2" \
              -n "NewName" \
              -e newuser@noreply.codeberg.org
```

**Rewrite locally and save without pushing:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -o "OldName" -n "NewName" \
              -e newuser@noreply.codeberg.org \
              --no-push --save-dir /path/to/output
```

**Save a local copy and push:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" -n "NewName" \
              -e newuser@noreply.codeberg.org \
              --save-dir /path/to/output
```

**Remove all matching Co-authored-by trailers:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" \
              -e myuser@noreply.codeberg.org \
              --coauthor-mode remove --coauthor-old "OldName"
```

**Rewrite Co-authored-by trailers:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" \
              -e myuser@noreply.codeberg.org \
              --coauthor-mode rewrite \
              --coauthor-old "OldName" \
              --coauthor-old-email "oldname@users.noreply.github.com" \
              --coauthor-new-name "NewName" \
              --coauthor-new-email "newname@noreply.codeberg.org"
```

**Interactive Co-authored-by review:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
              -o "OldName" \
              -e myuser@noreply.codeberg.org \
              --coauthor-mode interactive --coauthor-old "OldName"
```

</details>

### Interactive Mode

When using `-i` flag, you can review each commit and choose:

| Key | Action                             |
| --- | ---------------------------------- |
| `E` | Edit author name and email         |
| `M` | Edit commit message                |
| `A` | Apply default changes              |
| `S` | Skip (keep original)               |
| `D` | Apply defaults to all remaining    |
| `Q` | Quit and apply changes made so far |

### Co-authored-by Interactive Mode

When using `--coauthor-mode interactive`, you can review each affected commit and choose:

| Key | Action                                  |
| --- | --------------------------------------- |
| `R` | Rewrite the trailer with the new values |
| `D` | Delete (remove) the trailer             |
| `S` | Skip (keep original trailer)            |
| `Q` | Quit and apply decisions made so far    |

### Push Failure Recovery

If a push to Codeberg fails after the rewrite has completed, the script prompts you to either:

- **[S] Save** — copy the rewritten repository to a local path you specify, along with the exact commands needed to push it later
- **[C] Cleanup** — discard the temporary directory and exit

If `--save-dir` was already provided, the save proceeds automatically regardless of the push failure.

> [!NOTE]
>
> - The script operates on a temporary clone — your original repository is never modified
> - Rewriting history invalidates any existing commit signatures
> - `--no-push` is useful for inspecting the rewritten repository before committing to a push
> - `--save-dir` and `--dest` can be combined to both save a local copy and push in one run
> - `Co-authored-by` name matching is case-sensitive; use `--coauthor-old-email` to narrow matches when multiple authors share a name
> - When `--save-dir` is used alone (without `--dest`), `--no-push` is implied automatically
