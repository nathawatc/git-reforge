# GitHub to Codeberg

Migrate repositories from GitHub to Codeberg with history rewriting.

## Features

- **Repository Migration** - Clone and push repositories from GitHub to Codeberg
- **Author Rewriting** - Change author name and email across all commits
- **Multiple Author Support** - Rewrite commits from multiple old authors
- **Interactive Mode** - Review and customize changes per-commit:
  - Edit author name and email individually
  - Edit commit messages
  - Skip specific commits
  - Apply defaults to remaining commits
- **Co-author Rewriting** - Rewrite, remove, or review trailers interactively
- **Commit Signing** - GPG and SSH signing options:
  - Keep original signatures
  - Re-sign with GPG key
  - Re-sign with SSH key
  - Strip all signatures
- **Local Save** - Save the rewritten repository locally without pushing
- **Preserve History** - Original commit dates are preserved

## Requirements

- Git
- Go 1.21+
- GPG (optional, for GPG signing)
- SSH (optional, for SSH signing)
- A GitHub account with repositories to migrate
- A Codeberg account

## Installation

```bash
git clone https://codeberg.org/nathawat/gitHub-to-codeberg.git
cd gitHub-to-codeberg
go build .
```

## Usage

```bash
./github-to-codeberg [OPTIONS]
```

### Required Options

| Option                    | Description                                                                      |
| ------------------------- | -------------------------------------------------------------------------------- |
| `-s, --source <url>`      | GitHub repository URL                                                            |
| `-d, --dest <url>`        | Codeberg repository URL (not required when `--no-push` or `--save-dir` is given) |
| `-e, --new-email <email>` | New author email                                                                 |

### Optional Options

| Option                  | Description                                                   |
| ----------------------- | ------------------------------------------------------------- |
| `-n, --new-name <name>` | New author name                                               |
| `-o, --old-name <name>` | Old author name(s) to replace (repeatable or comma-separated) |
| `-i, --interactive`     | Enable interactive mode for per-commit editing                |
| `--sign-mode <mode>`    | Signing mode: `keep\|gpg\|ssh\|none` (default: `keep`)        |
| `--gpg-key <keyid>`     | GPG key ID for re-signing (required if `--sign-mode=gpg`)     |
| `--ssh-key <path>`      | SSH key path for re-signing (required if `--sign-mode=ssh`)   |
| `--nosign`              | Shorthand for `--sign-mode=none`                              |
| `--save-dir <path>`     | Save rewritten repo to `<path>` (directory must not exist)    |
| `--no-push`             | Rewrite history locally but skip pushing to Codeberg          |
| `-h, --help`            | Show help message                                             |

### Co-Authored-by Options

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
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName" \
-e myuser@noreply.codeberg.org
```

**Interactive mode with custom author:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName" -n "NewName" \
-e newuser@noreply.codeberg.org \
-i
```

**With GPG re-signing:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName" \
-e myuser@noreply.codeberg.org \
--sign-mode gpg --gpg-key ABCD1234
```

**Multiple old author names:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName1" -o "OldName2" \
-n "NewName" \
-e newuser@noreply.codeberg.org
```

**Rewrite locally and save without pushing:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-o "OldName" -n "NewName" \
-e newuser@noreply.codeberg.org \
--no-push --save-dir /path/to/output
```

**Save a local copy and push:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName" -n "NewName" \
-e newuser@noreply.codeberg.org \
--save-dir /path/to/output
```

**Remove all matching Co-authored-by trailers:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
-d https://codeberg.org/user/repo.git \
-o "OldName" \
-e myuser@noreply.codeberg.org \
--coauthor-mode remove --coauthor-old "OldName"
```

**Rewrite Co-authored-by trailers:**

```bash
./github-to-codeberg -s https://github.com/user/repo.git \
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
./github-to-codeberg -s https://github.com/user/repo.git \
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

### Co-Authored-by Interactive Mode

With `--coauthor-mode interactive`, you can review each affected commit:

| Key  | Action                                   |
| ---- | ---------------------------------------- |
| `R`  | Rewrite the trailer with the new values  |
| `D`  | Delete (remove) the trailer              |
| `S`  | Skip (keep original trailer)             |
| `RA` | Rewrite all remaining commits            |
| `DA` | Delete trailers in all remaining commits |
| `Q`  | Quit and apply decisions made so far     |

> [!NOTE]
>
> - The tool uses a temporary clone — original repo is never modified
> - Rewriting history invalidates any existing commit signatures
> - `--no-push` lets you inspect the rewritten repo before pushing
> - Combine `--save-dir` and `--dest` to save a local copy and push
> - `Co-authored-by` is case-sensitive; use email to narrow matches
> - `--save-dir` without `--dest` automatically implies `--no-push`
