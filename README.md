# Git Reforge

Reforge Git repository history (authors, committers, messages, co-authors, signatures) and
migrate seamlessly across Git forges (GitHub, Codeberg, Forgejo, GitLab).

## Features

- **Fast History Rewriting** - Uses `git-filter-repo` for near-instant history rewrites.
- **Author Rewriting** - Change author and committer names and emails across all commits.
- **Multiple Author Support** - Replace commits originating from multiple historical authors.
- **Interactive Mode** - Review and customize changes per commit:
  - Edit author name and email individually
  - Customize commit messages
  - Skip specific commits
  - Apply defaults to all remaining commits
- **Co-author Management** - Rewrite, remove, or review `Co-authored-by` trailers interactively.
- **Commit & Tag Signing** - Comprehensive GPG and SSH signing options:
  - Keep original signatures
  - Re-sign with GPG key
  - Re-sign with SSH key
  - Strip all signatures
  - Preserves original tagger names, emails, and historical release timestamps.
- **Atomic Multi-Branch Push** - Pushes all branches and tags in a single atomic network operation.
- **Local Inspection** - Save the rewritten repository locally without pushing (`--no-push`).
- **CI / Automation Ready** - Headless execution support using the `-y, --yes` flag.
- **Preserve History** - Original commit dates, parent links, and directory trees are preserved.

## Requirements

- Git 2.20+
- `git-filter-repo` (required)
- Go 1.24+ (required only if building from source)
- GPG (optional, for GPG signing)
- SSH (optional, for SSH signing)
- Access to source and destination Git repositories

## Installation

```bash
git clone https://github.com/nathawatc/git-reforge.git
cd git-reforge
go build .
```

## Usage

```bash
./git-reforge [OPTIONS] <source> [dest]
```

### Arguments

| Argument   | Description                                                                                       |
| ---------- | ------------------------------------------------------------------------------------------------- |
| `<source>` | Source repository URL                                                                             |
| `[dest]`   | Destination repository URL or local path (not required when `--no-push` or `--save-dir` is given) |

### Required Options

| Option              | Description                                                   |
| ------------------- | ------------------------------------------------------------- |
| `-t, --to <author>` | New author identity (`"Name <email>"` or `"email"`)           |
| `-f, --from <name>` | Old author name(s) to replace (repeatable or comma-separated) |

### Optional Options

| Option              | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `-i, --interactive` | Enable interactive mode for per-commit editing             |
| `--gpg <keyid>`     | Re-sign commits and tags with GPG key ID                   |
| `--ssh <path>`      | Re-sign commits and tags with SSH private key path         |
| `--no-sign`         | Strip all commit and tag signatures                        |
| `--strip-title <p>` | Regex to strip from commit title (repeatable)              |
| `--strip-body <p>`  | Regex to strip from commit body (repeatable)               |
| `--save-dir <path>` | Save rewritten repo to `<path>` (directory must not exist) |
| `--no-push`         | Rewrite history locally but skip pushing to destination    |
| `-y, --yes`         | Skip confirmation prompt for non-interactive / CI runs     |
| `-h, --help`        | Show help message                                          |

### Co-Authored-by Options

| Option                   | Description                                                |
| ------------------------ | ---------------------------------------------------------- |
| `-c, --coauthor <mode>`  | Trailer mode: `rewrite\|remove\|interactive`               |
| `--coauthor-from <n>`    | Co-author name(s) to match (repeatable; default: `--from`) |
| `--coauthor-to <author>` | Replacement identity (`"Name <email>"`; default: `--to`)   |

### Examples

<details>
<summary>Click to expand examples</summary>

**Basic migration:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org
```

**Non-interactive / CI run:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t "NewName <myuser@noreply.codeberg.org>" \
  -y
```

**Interactive mode with custom author:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t "NewName <newuser@noreply.codeberg.org>" \
  -i
```

**With GPG re-signing:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  --gpg ABCD1234
```

**With SSH re-signing:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  --ssh ~/.ssh/id_ed25519
```

**Multiple old author names:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName1" -f "OldName2" \
  -t "NewName <newuser@noreply.codeberg.org>"
```

**Rewrite locally and save without pushing:**

```bash
./git-reforge https://github.com/user/repo.git \
  /path/to/output \
  -f "OldName" \
  -t "NewName <newuser@noreply.codeberg.org>"
```

**Save a local copy and push:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t "NewName <newuser@noreply.codeberg.org>" \
  --save-dir /path/to/output
```

**Remove all matching Co-authored-by trailers:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  -c remove
```

**Rewrite Co-authored-by trailers:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  -c rewrite
```

**Rewrite custom Co-authored-by trailers:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  --coauthor-from "Charlie <charlie@old.com>" \
  --coauthor-to "Charlie <charlie@codeberg.org>"
```

**Interactive Co-authored-by review:**

```bash
./git-reforge https://github.com/user/repo.git \
  https://codeberg.org/user/repo.git \
  -f "OldName" \
  -t myuser@noreply.codeberg.org \
  -c interactive
```

</details>

### Interactive Mode

When using the `-i` flag, you can review each commit individually:

| Key | Action                             |
| --- | ---------------------------------- |
| `E` | Edit author name and email         |
| `M` | Edit commit message                |
| `A` | Apply default changes              |
| `S` | Skip (keep original)               |
| `D` | Apply defaults to all remaining    |
| `Q` | Quit and apply changes made so far |

### Co-Authored-by Interactive Mode

With `--coauthor interactive`, you can review each affected commit:

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
> - The tool uses a temporary clone — your original repository is never modified.
> - Rewriting commit history changes commit SHAs, which invalidates original cryptographic signatures.
> - `--no-push` allows you to inspect the rewritten repository locally before pushing.
> - Combine `--save-dir` and `<dest>` to save a local backup copy and push to destination in one run.
> - `Co-authored-by` matching defaults to `--from`; supply `--coauthor-from` to match custom identities.
> - `--save-dir` without `<dest>` automatically implies `--no-push`.
