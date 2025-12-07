# GitHub-2-Codeberg

A powerful tool for migrating repositories from GitHub to Codeberg with commit history rewriting capabilities.

## Features

- **Repository Migration** - Clone and push repositories from GitHub to Codeberg
- **Author Rewriting** - Change author name and email across all commits
- **Multiple Author Support** - Rewrite commits from multiple old authors
- **Interactive Mode** - Review and customize changes per-commit:
  - Edit author name and email individually
  - **Edit commit messages**
  - Skip specific commits
  - Apply defaults to remaining commits
- **Commit Signing** - GPG and SSH signing options:
  - Keep original signatures
  - Re-sign with GPG key
  - Re-sign with SSH key
  - Strip all signatures
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
git clone https://codeberg.org/yourusername/GitHub-2-Codeberg.git
cd GitHub-2-Codeberg
chmod +x src/main.sh
```

## Usage

```bash
./src/main.sh [OPTIONS]
```

### Required Options

| Option | Description |
|--------|-------------|
| `-s, --source <url>` | GitHub repository URL |
| `-d, --dest <url>` | Codeberg repository URL |
| `-e, --new-email <email>` | New author email |

### Optional Options

| Option | Description |
|--------|-------------|
| `-n, --new-name <name>` | New author name (default: Byteintosh) |
| `-o, --old-name <name>` | Old author name(s) to replace (can be comma-separated) |
| `-i, --interactive` | Enable interactive mode for per-commit editing |
| `--sign-mode <mode>` | Signing mode: `keep\|gpg\|ssh\|none` |
| `--gpg-key <keyid>` | GPG key ID for re-signing |
| `--ssh-key <path>` | SSH key path for re-signing |
| `--nosign` | Shorthand for `--sign-mode=none` |
| `-h, --help` | Show help message |

### Examples

**Basic migration:**

```bash
./src/main.sh -s https://github.com/user/repo.git \
              -d https://codeberg.org/user/repo.git \
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
              -e myuser@noreply.codeberg.org \
              --sign-mode gpg --gpg-key ABCD1234
```

### Interactive Mode

When using `-i` flag, you can review each commit and choose:

| Key | Action |
|-----|--------|
| `E` | Edit author name and email |
| `M` | Edit commit message |
| `A` | Apply default changes |
| `S` | Skip (keep original) |
| `D` | Apply defaults to all remaining |
| `Q` | Quit and apply changes made so far |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source. See the LICENSE file for details.
