#!/usr/bin/env bash

#===============================================================================
# GitHub to Codeberg Migration Script
#
# Migrates a GitHub repository to Codeberg with commit history rewriting:
# - Change author name
# - Change author email (full replacement to @noreply.codeberg.org)
# - Preserve commit dates
# - Interactive mode for per-commit editing
# - GPG/SSH signing options
#===============================================================================

set -euo pipefail

#-------------------------------------------------------------------------------
# Colors and Formatting
#-------------------------------------------------------------------------------
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly NC='\033[0m' # No Color

#-------------------------------------------------------------------------------
# Default Configuration
#-------------------------------------------------------------------------------
OLD_NAMES=() # Array of old names to replace
NEW_NAME=""
NEW_EMAIL=""
SOURCE_URL=""
DEST_URL=""
INTERACTIVE=false
SIGN_MODE="keep"
GPG_KEY=""
SSH_KEY=""
TEMP_DIR=""
DEFAULT_OLD_NAME="Scott-Nx"

#-------------------------------------------------------------------------------
# Utility Functions
#-------------------------------------------------------------------------------
log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_header() {
  echo ""
  echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BOLD}${CYAN}  $1${NC}"
  echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════════════════${NC}"
  echo ""
}

#-------------------------------------------------------------------------------
# Cleanup Function
#-------------------------------------------------------------------------------
cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    log_info "Cleaning up temporary directory..."
    rm -rf "$TEMP_DIR"
  fi
}

trap cleanup EXIT

#-------------------------------------------------------------------------------
# Help Message
#-------------------------------------------------------------------------------
show_help() {
  cat <<EOF
${BOLD}GitHub to Codeberg Migration Script${NC}

${BOLD}USAGE:${NC}
    $(basename "$0") [OPTIONS]

${BOLD}REQUIRED OPTIONS:${NC}
    -s, --source <url>       GitHub repository URL
    -d, --dest <url>         Codeberg repository URL
    -e, --new-email <email>  New author email (e.g., username@noreply.codeberg.org)

${BOLD}OPTIONAL:${NC}
    -n, --new-name <name>    New author name (default: Byteintosh)
    -o, --old-name <name>    Old author name(s) to replace. Can be specified multiple times
                             or as comma-separated values (default: Scott-Nx)
                             Examples: -o "Name1" -o "Name2" or -o "Name1,Name2"
    -i, --interactive        Enable interactive mode for per-commit editing
    --sign-mode <mode>       Signing mode: keep|gpg|ssh|none (default: keep)
                             - keep: Preserve original signatures (note: invalid after rewrite)
                             - gpg:  Re-sign all commits with GPG key
                             - ssh:  Re-sign all commits with SSH key
                             - none: Strip all signatures
    --gpg-key <keyid>        GPG key ID for re-signing (required if --sign-mode=gpg)
    --ssh-key <path>         SSH key path for re-signing (required if --sign-mode=ssh)
    --nosign                 Shorthand for --sign-mode=none (strip all signatures)
    -h, --help               Show this help message

${BOLD}EXAMPLES:${NC}
    # Basic migration
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org

    # Interactive mode with GPG re-signing
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     -i --sign-mode gpg --gpg-key ABCD1234

    # Custom author names
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -o "OldName" -n "NewName" \\
                     -e newuser@noreply.codeberg.org

    # Multiple old author names
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -o "OldName1" -o "OldName2" -o "OldName3" \\
                     -n "NewName" -e newuser@noreply.codeberg.org

    # Multiple old names (comma-separated)
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -o "OldName1,OldName2,OldName3" \\
                     -n "NewName" -e newuser@noreply.codeberg.org

${BOLD}NOTES:${NC}
    - This script only modifies commits from the specified old author name(s)
    - Multiple old names can be specified to rewrite commits from different authors
    - Original commit dates are preserved
    - The script works on a clone, not your original repository
    - Rewriting history invalidates existing signatures

EOF
}

#-------------------------------------------------------------------------------
# Parse Arguments
#-------------------------------------------------------------------------------
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
    -s | --source)
      SOURCE_URL="$2"
      shift 2
      ;;
    -d | --dest)
      DEST_URL="$2"
      shift 2
      ;;
    -n | --new-name)
      NEW_NAME="$2"
      shift 2
      ;;
    -e | --new-email)
      NEW_EMAIL="$2"
      shift 2
      ;;
    -o | --old-name)
      # Support comma-separated values
      IFS=',' read -ra NAMES <<<"$2"
      for name in "${NAMES[@]}"; do
        # Trim whitespace
        name="$(echo -e "${name}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
        OLD_NAMES+=("$name")
      done
      shift 2
      ;;
    -i | --interactive)
      INTERACTIVE=true
      shift
      ;;
    --sign-mode)
      SIGN_MODE="$2"
      if [[ ! "$SIGN_MODE" =~ ^(keep|gpg|ssh|none)$ ]]; then
        log_error "Invalid sign mode: $SIGN_MODE (must be: keep|gpg|ssh|none)"
        exit 1
      fi
      shift 2
      ;;
    --gpg-key)
      GPG_KEY="$2"
      shift 2
      ;;
    --ssh-key)
      SSH_KEY="$2"
      shift 2
      ;;
    --nosign)
      SIGN_MODE="none"
      shift
      ;;
    -h | --help)
      show_help
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      echo "Use --help for usage information"
      exit 1
      ;;
    esac
  done
}

#-------------------------------------------------------------------------------
# Validate Arguments
#-------------------------------------------------------------------------------
validate_args() {
  local errors=0

  if [[ -z "$SOURCE_URL" ]]; then
    log_error "Source URL is required (-s, --source)"
    errors=$((errors + 1))
  fi

  if [[ -z "$DEST_URL" ]]; then
    log_error "Destination URL is required (-d, --dest)"
    errors=$((errors + 1))
  fi

  if [[ -z "$NEW_EMAIL" ]]; then
    log_error "New email is required (-e, --new-email)"
    errors=$((errors + 1))
  fi

  if [[ "$SIGN_MODE" == "gpg" && -z "$GPG_KEY" ]]; then
    log_error "GPG key is required when using --sign-mode=gpg (--gpg-key)"
    errors=$((errors + 1))
  fi

  if [[ "$SIGN_MODE" == "ssh" && -z "$SSH_KEY" ]]; then
    log_error "SSH key path is required when using --sign-mode=ssh (--ssh-key)"
    errors=$((errors + 1))
  fi

  if [[ "$SIGN_MODE" == "ssh" && -n "$SSH_KEY" && ! -f "$SSH_KEY" ]]; then
    log_error "SSH key file not found: $SSH_KEY"
    errors=$((errors + 1))
  fi

  if [[ $errors -gt 0 ]]; then
    echo ""
    echo "Use --help for usage information"
    exit 1
  fi

  # If no old names specified, use default
  if [[ ${#OLD_NAMES[@]} -eq 0 ]]; then
    OLD_NAMES=("$DEFAULT_OLD_NAME")
  fi
}

#-------------------------------------------------------------------------------
# Check Dependencies
#-------------------------------------------------------------------------------
check_dependencies() {
  log_info "Checking dependencies..."

  if ! command -v git &>/dev/null; then
    log_error "git is not installed"
    exit 1
  fi

  if [[ "$SIGN_MODE" == "gpg" ]]; then
    if ! command -v gpg &>/dev/null; then
      log_error "gpg is not installed (required for GPG signing)"
      exit 1
    fi

    # Check if GPG key exists
    if ! gpg --list-secret-keys "$GPG_KEY" &>/dev/null; then
      log_error "GPG key not found: $GPG_KEY"
      exit 1
    fi
  fi

  log_success "All dependencies satisfied"
}

#-------------------------------------------------------------------------------
# Clone Repository
#-------------------------------------------------------------------------------
clone_repository() {
  log_header "Cloning Repository"

  TEMP_DIR=$(mktemp -d -t github-codeberg-migration.XXXXXX)
  log_info "Created temporary directory: $TEMP_DIR"

  log_info "Cloning from: $SOURCE_URL"
  if ! git clone --mirror "$SOURCE_URL" "$TEMP_DIR/repo.git"; then
    log_error "Failed to clone repository"
    exit 1
  fi

  # Convert bare repo to normal repo for easier manipulation
  log_info "Converting to working repository..."
  cd "$TEMP_DIR"
  git clone repo.git working
  cd working

  # Fetch all branches
  git fetch --all

  log_success "Repository cloned successfully"
}

#-------------------------------------------------------------------------------
# Display Configuration
#-------------------------------------------------------------------------------
display_config() {
  log_header "Migration Configuration"

  echo -e "  ${BOLD}Source:${NC}        $SOURCE_URL"
  echo -e "  ${BOLD}Destination:${NC}   $DEST_URL"
  echo ""
  if [[ ${#OLD_NAMES[@]} -eq 1 ]]; then
    echo -e "  ${BOLD}Old Author:${NC}    ${OLD_NAMES[0]}"
  else
    echo -e "  ${BOLD}Old Authors:${NC}   ${OLD_NAMES[0]}"
    for ((i = 1; i < ${#OLD_NAMES[@]}; i++)); do
      echo -e "                 ${OLD_NAMES[$i]}"
    done
  fi
  echo -e "  ${BOLD}New Author:${NC}    $NEW_NAME"
  echo -e "  ${BOLD}New Email:${NC}     $NEW_EMAIL"
  echo ""
  echo -e "  ${BOLD}Interactive:${NC}   $INTERACTIVE"
  echo -e "  ${BOLD}Sign Mode:${NC}     $SIGN_MODE"
  [[ -n "$GPG_KEY" ]] && echo -e "  ${BOLD}GPG Key:${NC}       $GPG_KEY"
  [[ -n "$SSH_KEY" ]] && echo -e "  ${BOLD}SSH Key:${NC}       $SSH_KEY"
  echo ""

  read -p "Proceed with migration? [y/N] " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Migration cancelled"
    exit 0
  fi
}

#-------------------------------------------------------------------------------
# Interactive Commit Editor
#-------------------------------------------------------------------------------
declare -A COMMIT_OVERRIDES

interactive_edit() {
  log_header "Interactive Commit Editing"

  log_info "Fetching commits from author(s): ${OLD_NAMES[*]}"

  # Get list of commits by all old authors
  local commits=""
  for old_name in "${OLD_NAMES[@]}"; do
    local author_commits
    author_commits=$(git log --all --format="%H" --author="$old_name" 2>/dev/null || true)
    if [[ -n "$author_commits" ]]; then
      if [[ -n "$commits" ]]; then
        commits="$commits"$'\n'"$author_commits"
      else
        commits="$author_commits"
      fi
    fi
  done

  # Remove duplicates and sort
  commits=$(echo "$commits" | sort -u)

  if [[ -z "$commits" ]]; then
    log_warning "No commits found from author(s): ${OLD_NAMES[*]}"
    return
  fi

  local commit_count
  commit_count=$(echo "$commits" | wc -l)
  log_info "Found $commit_count commit(s) to review"
  echo ""

  local current=0
  while IFS= read -r commit_hash; do
    current=$((current + 1))

    # Get commit details
    local author_name author_email author_date committer_name committer_email message
    author_name=$(git log -1 --format="%an" "$commit_hash")
    author_email=$(git log -1 --format="%ae" "$commit_hash")
    author_date=$(git log -1 --format="%ai" "$commit_hash")
    committer_name=$(git log -1 --format="%cn" "$commit_hash")
    committer_email=$(git log -1 --format="%ce" "$commit_hash")
    message=$(git log -1 --format="%s" "$commit_hash")

    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    echo -e "${BOLD}Commit $current of $commit_count${NC}"
    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    echo -e "  ${CYAN}Hash:${NC}       ${commit_hash:0:12}"
    echo -e "  ${CYAN}Author:${NC}     $author_name <$author_email>"
    echo -e "  ${CYAN}Committer:${NC}  $committer_name <$committer_email>"
    echo -e "  ${CYAN}Date:${NC}       $author_date"
    echo -e "  ${CYAN}Message:${NC}    $message"
    echo ""
    echo -e "  ${BOLD}Default changes:${NC}"
    echo -e "    Name:  $author_name → $NEW_NAME"
    echo -e "    Email: $author_email → $NEW_EMAIL"
    echo ""

    while true; do
      echo -e "  ${YELLOW}[E]dit${NC} - Customize changes for this commit"
      echo -e "  ${GREEN}[A]pply${NC} - Apply default changes"
      echo -e "  ${BLUE}[S]kip${NC} - Keep original (no changes)"
      echo -e "  ${CYAN}[D]efault all${NC} - Apply default to all remaining"
      echo -e "  ${RED}[Q]uit${NC} - Stop and apply changes made so far"
      echo ""
      read -p "  Choice [E/A/S/D/Q]: " -n 1 -r choice
      echo ""

      case "${choice^^}" in
      E)
        echo ""
        read -p "  New author name [$NEW_NAME]: " custom_name
        read -p "  New author email [$NEW_EMAIL]: " custom_email

        custom_name="${custom_name:-$NEW_NAME}"
        custom_email="${custom_email:-$NEW_EMAIL}"

        COMMIT_OVERRIDES["$commit_hash"]="$custom_name|$custom_email"
        log_success "Custom changes saved for commit ${commit_hash:0:12}"
        break
        ;;
      A)
        COMMIT_OVERRIDES["$commit_hash"]="$NEW_NAME|$NEW_EMAIL"
        log_success "Default changes will be applied to commit ${commit_hash:0:12}"
        break
        ;;
      S)
        COMMIT_OVERRIDES["$commit_hash"]="SKIP"
        log_info "Commit ${commit_hash:0:12} will be kept unchanged"
        break
        ;;
      D)
        log_info "Applying default changes to all remaining commits..."
        # Apply default to current and all remaining
        COMMIT_OVERRIDES["$commit_hash"]="$NEW_NAME|$NEW_EMAIL"
        while IFS= read -r remaining_hash; do
          COMMIT_OVERRIDES["$remaining_hash"]="$NEW_NAME|$NEW_EMAIL"
        done <<<"$(echo "$commits" | tail -n +$((current + 1)))"
        return
        ;;
      Q)
        log_info "Stopping interactive mode. Changes made so far will be applied."
        return
        ;;
      *)
        log_warning "Invalid choice. Please select E, A, S, D, or Q."
        ;;
      esac
    done
    echo ""
  done <<<"$commits"
}

#-------------------------------------------------------------------------------
# Generate Filter Script for Interactive Mode
#-------------------------------------------------------------------------------
generate_interactive_filter() {
  local filter_script="$TEMP_DIR/filter.sh"

  cat >"$filter_script" <<'FILTER_HEADER'
#!/usr/bin/env bash
FILTER_HEADER

  # Add commit overrides as a case statement
  echo 'case "$GIT_COMMIT" in' >>"$filter_script"

  for commit_hash in "${!COMMIT_OVERRIDES[@]}"; do
    local override="${COMMIT_OVERRIDES[$commit_hash]}"

    if [[ "$override" == "SKIP" ]]; then
      # Keep original - no changes
      echo "    $commit_hash)" >>"$filter_script"
      echo "        ;;" >>"$filter_script"
    else
      local custom_name custom_email
      IFS='|' read -r custom_name custom_email <<<"$override"

      cat >>"$filter_script" <<EOF
    $commit_hash)
        export GIT_AUTHOR_NAME="$custom_name"
        export GIT_AUTHOR_EMAIL="$custom_email"
        export GIT_COMMITTER_NAME="$custom_name"
        export GIT_COMMITTER_EMAIL="$custom_email"
        ;;
EOF
    fi
  done

  echo 'esac' >>"$filter_script"

  chmod +x "$filter_script"
  echo "$filter_script"
}

#-------------------------------------------------------------------------------
# Rewrite Commits (Batch Mode)
#-------------------------------------------------------------------------------
rewrite_commits_batch() {
  log_header "Rewriting Commit History"

  if [[ ${#OLD_NAMES[@]} -eq 1 ]]; then
    log_info "Rewriting commits from '${OLD_NAMES[0]}' to '$NEW_NAME' <$NEW_EMAIL>"
  else
    log_info "Rewriting commits from ${#OLD_NAMES[@]} authors to '$NEW_NAME' <$NEW_EMAIL>"
    for old_name in "${OLD_NAMES[@]}"; do
      log_info "  - $old_name"
    done
  fi

  # Export variables for the filter script
  export FILTER_NEW_NAME="$NEW_NAME"
  export FILTER_NEW_EMAIL="$NEW_EMAIL"

  # Build the condition for matching old names
  local filter_script="$TEMP_DIR/batch_filter.sh"

  cat >"$filter_script" <<'FILTER_START'
#!/usr/bin/env bash
# Check if current author matches any of the old names
FILTER_START

  # Add array of old names
  echo 'OLD_NAMES_ARRAY=(' >>"$filter_script"
  for old_name in "${OLD_NAMES[@]}"; do
    echo "    \"$old_name\"" >>"$filter_script"
  done
  echo ')' >>"$filter_script"

  cat >>"$filter_script" <<'FILTER_LOGIC'

# Check if author matches any old name
for old_name in "${OLD_NAMES_ARRAY[@]}"; do
    if [ "$GIT_AUTHOR_NAME" = "$old_name" ]; then
        export GIT_AUTHOR_NAME="$FILTER_NEW_NAME"
        export GIT_AUTHOR_EMAIL="$FILTER_NEW_EMAIL"
        break
    fi
done

# Check if committer matches any old name
for old_name in "${OLD_NAMES_ARRAY[@]}"; do
    if [ "$GIT_COMMITTER_NAME" = "$old_name" ]; then
        export GIT_COMMITTER_NAME="$FILTER_NEW_NAME"
        export GIT_COMMITTER_EMAIL="$FILTER_NEW_EMAIL"
        break
    fi
done
FILTER_LOGIC

  chmod +x "$filter_script"

  # Use git filter-branch with the generated script
  git filter-branch -f --env-filter "source $filter_script" --tag-name-filter cat -- --all

  log_success "Commit history rewritten successfully"
}

#-------------------------------------------------------------------------------
# Rewrite Commits (Interactive Mode)
#-------------------------------------------------------------------------------
rewrite_commits_interactive() {
  log_header "Rewriting Commit History (Interactive)"

  if [[ ${#COMMIT_OVERRIDES[@]} -eq 0 ]]; then
    log_warning "No commits selected for modification"
    return
  fi

  log_info "Applying ${#COMMIT_OVERRIDES[@]} commit modification(s)..."

  # Generate filter script
  local filter_script
  filter_script=$(generate_interactive_filter)

  # Apply using filter-branch
  git filter-branch -f --env-filter "source $filter_script" --tag-name-filter cat -- --all

  log_success "Commit history rewritten successfully"
}

#-------------------------------------------------------------------------------
# Handle Commit Signing
#-------------------------------------------------------------------------------
handle_signing() {
  case "$SIGN_MODE" in
  keep)
    log_info "Keeping original signatures (note: they are now invalid due to content changes)"
    ;;
  none)
    log_header "Removing Signatures"
    log_info "Stripping all commit signatures..."

    git filter-branch -f --commit-filter '
                git commit-tree "$@"
            ' --tag-name-filter cat -- --all

    log_success "Signatures removed"
    ;;
  gpg)
    log_header "Re-signing with GPG"
    log_info "Re-signing all commits with GPG key: $GPG_KEY"

    git config user.signingkey "$GPG_KEY"

    git filter-branch -f --commit-filter "
                git commit-tree -S\"$GPG_KEY\" \"\$@\"
            " --tag-name-filter cat -- --all

    log_success "Commits re-signed with GPG"
    ;;
  ssh)
    log_header "Re-signing with SSH"
    log_info "Re-signing all commits with SSH key: $SSH_KEY"

    git config gpg.format ssh
    git config user.signingkey "$SSH_KEY"

    git filter-branch -f --commit-filter '
                git commit-tree -S "$@"
            ' --tag-name-filter cat -- --all

    log_success "Commits re-signed with SSH"
    ;;
  esac
}

#-------------------------------------------------------------------------------
# Push to Codeberg
#-------------------------------------------------------------------------------
push_to_destination() {
  log_header "Pushing to Codeberg"

  log_info "Setting remote URL to: $DEST_URL"
  git remote set-url origin "$DEST_URL"

  log_info "Pushing all branches..."
  if ! git push --all --force; then
    log_error "Failed to push branches"
    exit 1
  fi

  log_info "Pushing all tags..."
  if ! git push --tags --force; then
    log_warning "Failed to push some tags (this may be expected if no tags exist)"
  fi

  log_success "Repository pushed to Codeberg successfully"
}

#-------------------------------------------------------------------------------
# Summary
#-------------------------------------------------------------------------------
show_summary() {
  log_header "Migration Complete!"

  echo -e "  ${GREEN}✓${NC} Repository migrated from GitHub to Codeberg"
  if [[ ${#OLD_NAMES[@]} -eq 1 ]]; then
    echo -e "  ${GREEN}✓${NC} Author '${OLD_NAMES[0]}' changed to '${NEW_NAME}'"
  else
    echo -e "  ${GREEN}✓${NC} Authors changed to '${NEW_NAME}':"
    for old_name in "${OLD_NAMES[@]}"; do
      echo -e "      - $old_name"
    done
  fi
  echo -e "  ${GREEN}✓${NC} Email changed to '${NEW_EMAIL}'"
  echo -e "  ${GREEN}✓${NC} Original commit dates preserved"

  case "$SIGN_MODE" in
  keep)
    echo -e "  ${YELLOW}!${NC} Original signatures kept (but are now invalid)"
    ;;
  none)
    echo -e "  ${GREEN}✓${NC} Signatures removed"
    ;;
  gpg)
    echo -e "  ${GREEN}✓${NC} Commits re-signed with GPG key: $GPG_KEY"
    ;;
  ssh)
    echo -e "  ${GREEN}✓${NC} Commits re-signed with SSH key: $SSH_KEY"
    ;;
  esac

  echo ""
  echo -e "  ${BOLD}Codeberg Repository:${NC} $DEST_URL"
  echo ""
}

#-------------------------------------------------------------------------------
# Main
#-------------------------------------------------------------------------------
main() {
  log_header "GitHub to Codeberg Migration"

  parse_args "$@"
  validate_args
  check_dependencies
  display_config
  clone_repository

  cd "$TEMP_DIR/working"

  if [[ "$INTERACTIVE" == true ]]; then
    interactive_edit
    rewrite_commits_interactive
  else
    rewrite_commits_batch
  fi

  handle_signing
  push_to_destination
  show_summary
}

main "$@"
