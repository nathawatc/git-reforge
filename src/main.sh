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
# - Rewrite or remove Co-authored-by trailers
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

# Co-authored-by configuration
COAUTHOR_MODE=""           # rewrite|remove|interactive (empty = disabled)
COAUTHOR_OLD_NAME=""       # Old name to match inside Co-authored-by trailer
COAUTHOR_OLD_EMAIL=""      # Old email to match inside Co-authored-by trailer (optional)
COAUTHOR_NEW_NAME=""       # Replacement name  (defaults to NEW_NAME)
COAUTHOR_NEW_EMAIL=""      # Replacement email (defaults to NEW_EMAIL)

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

    --coauthor-mode <mode>         Co-authored-by trailer mode: rewrite|remove|interactive
                                   - rewrite:     Replace matching trailers in all commits
                                   - remove:      Delete matching trailers from all commits
                                   - interactive: Review each affected commit individually
    --coauthor-old <name>          Name to match in Co-authored-by trailers (default: first --old-name)
    --coauthor-old-email <email>   Email to match in Co-authored-by trailers (optional, narrows match)
    --coauthor-new-name <name>     Replacement name  (default: --new-name value)
    --coauthor-new-email <email>   Replacement email (default: --new-email value)

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

    # Remove Co-authored-by trailers from all commits
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     --coauthor-mode remove --coauthor-old "Scott-Nx"

    # Rewrite Co-authored-by trailers with a specific old email to narrow the match
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     --coauthor-mode rewrite \\
                     --coauthor-old "Scott-Nx" \\
                     --coauthor-old-email "158021536+Scott-Nx@users.noreply.github.com" \\
                     --coauthor-new-name "Byteintosh" \\
                     --coauthor-new-email "myuser@noreply.codeberg.org"

    # Interactive Co-authored-by review
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     --coauthor-mode interactive --coauthor-old "Scott-Nx"

${BOLD}NOTES:${NC}
    - This script only modifies commits from the specified old author name(s)
    - Multiple old names can be specified to rewrite commits from different authors
    - Original commit dates are preserved
    - The script works on a clone, not your original repository
    - Rewriting history invalidates existing signatures
    - Co-authored-by matching is case-sensitive on the name field
    - When --coauthor-old-email is omitted, any email paired with --coauthor-old matches

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
    --coauthor-mode)
      COAUTHOR_MODE="$2"
      if [[ ! "$COAUTHOR_MODE" =~ ^(rewrite|remove|interactive)$ ]]; then
        log_error "Invalid coauthor mode: $COAUTHOR_MODE (must be: rewrite|remove|interactive)"
        exit 1
      fi
      shift 2
      ;;
    --coauthor-old)
      COAUTHOR_OLD_NAME="$2"
      shift 2
      ;;
    --coauthor-old-email)
      COAUTHOR_OLD_EMAIL="$2"
      shift 2
      ;;
    --coauthor-new-name)
      COAUTHOR_NEW_NAME="$2"
      shift 2
      ;;
    --coauthor-new-email)
      COAUTHOR_NEW_EMAIL="$2"
      shift 2
      ;;
    -h | --help)
      show_help
      exit 0
      ;;
    --*=*)
      set -- "${1%%=*}" "${1#*=}" "${@:2}"
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

  # Apply Co-authored-by defaults after OLD_NAMES is finalised
  if [[ -n "$COAUTHOR_MODE" ]]; then
    if [[ -z "$COAUTHOR_OLD_NAME" ]]; then
      COAUTHOR_OLD_NAME="${OLD_NAMES[0]}"
    fi
    if [[ -z "$COAUTHOR_NEW_NAME" ]]; then
      COAUTHOR_NEW_NAME="$NEW_NAME"
    fi
    if [[ -z "$COAUTHOR_NEW_EMAIL" ]]; then
      COAUTHOR_NEW_EMAIL="$NEW_EMAIL"
    fi
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
  if [[ -n "$COAUTHOR_MODE" ]]; then
    echo ""
    echo -e "  ${BOLD}Co-author Mode:${NC}  $COAUTHOR_MODE"
    local coauthor_match="$COAUTHOR_OLD_NAME"
    [[ -n "$COAUTHOR_OLD_EMAIL" ]] && coauthor_match+=" <$COAUTHOR_OLD_EMAIL>"
    echo -e "  ${BOLD}Co-author Match:${NC} $coauthor_match"
    if [[ "$COAUTHOR_MODE" == "rewrite" ]]; then
      echo -e "  ${BOLD}Co-author New:${NC}   $COAUTHOR_NEW_NAME <$COAUTHOR_NEW_EMAIL>"
    fi
  fi
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
declare -A COMMIT_MESSAGES
# Coauthor interactive decisions: hash -> "rewrite"|"remove"|"skip"
declare -A COMMIT_COAUTHOR_ACTIONS

interactive_edit() {
  log_header "Interactive Commit Editing"

  log_info "Fetching commits from author(s): ${OLD_NAMES[*]}"

  # Get list of commits by all old authors
  local commits=""
  for old_name in "${OLD_NAMES[@]}"; do
    local author_commits
    # Use --fixed-strings equivalent: wrap name in \Q...\E so regex metacharacters
    # like [bot] are treated as literals by git log's POSIX regex engine.
    local escaped_name
    escaped_name=$(printf '%s' "$old_name" | sed 's/[][\\.^$*{}()|+?]/\\&/g')
    author_commits=$(git log --all --format="%H" --author="$escaped_name" 2>/dev/null || true)
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
    # Check if message has been customized
    local msg_status="(unchanged)"
    if [[ -v COMMIT_MESSAGES["$commit_hash"] ]]; then
      msg_status="${YELLOW}(customized)${NC}"
    fi
    echo -e "  ${BOLD}Default changes:${NC}"
    echo -e "    Name:    $author_name → $NEW_NAME"
    echo -e "    Email:   $author_email → $NEW_EMAIL"
    echo -e "    Message: $msg_status"
    echo ""

    while true; do
      echo -e "  ${YELLOW}[E]dit${NC} - Customize name/email for this commit"
      echo -e "  ${YELLOW}[M]essage${NC} - Edit commit message"
      echo -e "  ${GREEN}[A]pply${NC} - Apply default changes"
      echo -e "  ${BLUE}[S]kip${NC} - Keep original (no changes)"
      echo -e "  ${CYAN}[D]efault all${NC} - Apply default to all remaining"
      echo -e "  ${RED}[Q]uit${NC} - Stop and apply changes made so far"
      echo ""
      read -r -p "  Choice [E/A/M/S/D/Q]: " choice </dev/tty
      echo ""

      case "${choice^^}" in
      E)
        echo ""
        read -r -p "  New author name [$NEW_NAME]: " custom_name </dev/tty
        read -r -p "  New author email [$NEW_EMAIL]: " custom_email </dev/tty

        custom_name="${custom_name:-$NEW_NAME}"
        custom_email="${custom_email:-$NEW_EMAIL}"

        COMMIT_OVERRIDES["$commit_hash"]="$custom_name|$custom_email"
        log_success "Custom name/email saved for commit ${commit_hash:0:12}"
        echo ""
        ;;
      M)
        echo ""
        echo -e "  ${CYAN}Current message:${NC}"
        echo -e "    $message"
        echo ""
        echo -e "  ${CYAN}Enter new commit message (or press Enter to keep current):${NC}"
        read -r custom_message </dev/tty

        if [[ -n "$custom_message" ]]; then
          COMMIT_MESSAGES["$commit_hash"]="$custom_message"
          log_success "Custom message saved for commit ${commit_hash:0:12}"
        else
          log_info "Keeping original message for commit ${commit_hash:0:12}"
        fi
        echo ""
        ;;
      A)
        COMMIT_OVERRIDES["$commit_hash"]="$NEW_NAME|$NEW_EMAIL"
        log_success "Changes will be applied to commit ${commit_hash:0:12}"
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
        log_warning "Invalid choice. Please select E, M, A, S, D, or Q."
        ;;
      esac
    done
    echo ""
  done <<<"$commits"
}

#-------------------------------------------------------------------------------
# Interactive Co-authored-by Editor
#-------------------------------------------------------------------------------
interactive_coauthor_edit() {
  log_header "Interactive Co-authored-by Editing"

  # Build a regex pattern to find matching Co-authored-by lines.
  # We always match the name literally; email is optional extra constraint.
  local escaped_old_name
  escaped_old_name=$(printf '%s' "$COAUTHOR_OLD_NAME" | sed 's/[][\\.^$*{}()|+?]/\\&/g')

  local grep_pattern
  if [[ -n "$COAUTHOR_OLD_EMAIL" ]]; then
    local escaped_old_email
    escaped_old_email=$(printf '%s' "$COAUTHOR_OLD_EMAIL" | sed 's/[][\\.^$*{}()|+?]/\\&/g')
    grep_pattern="Co-authored-by: ${escaped_old_name} <${escaped_old_email}>"
  else
    grep_pattern="Co-authored-by: ${escaped_old_name} "
  fi

  log_info "Scanning all commits for Co-authored-by trailers matching: $COAUTHOR_OLD_NAME"

  # Collect hashes of every commit whose message contains a matching trailer
  local matching_commits=""
  while IFS= read -r commit_hash; do
    local body
    body=$(git log -1 --format="%B" "$commit_hash")
    if echo "$body" | grep -qE "$grep_pattern"; then
      matching_commits+="$commit_hash"$'\n'
    fi
  done < <(git log --all --format="%H")

  # Trim trailing newline and de-duplicate
  matching_commits=$(printf '%s' "$matching_commits" | sort -u)

  if [[ -z "$matching_commits" ]]; then
    log_warning "No commits found containing matching Co-authored-by trailers"
    return
  fi

  local commit_count
  commit_count=$(echo "$matching_commits" | wc -l)
  log_info "Found $commit_count commit(s) with matching Co-authored-by trailer(s)"
  echo ""

  local current=0
  while IFS= read -r commit_hash; do
    [[ -z "$commit_hash" ]] && continue
    current=$((current + 1))

    local author_name author_date subject body
    author_name=$(git log -1 --format="%an" "$commit_hash")
    author_date=$(git log -1 --format="%ai" "$commit_hash")
    subject=$(git log -1 --format="%s" "$commit_hash")
    body=$(git log -1 --format="%B" "$commit_hash")

    # Extract just the matching trailer lines for display
    local matching_lines
    matching_lines=$(echo "$body" | grep -E "$grep_pattern" || true)

    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    echo -e "${BOLD}Co-author commit $current of $commit_count${NC}"
    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    echo -e "  ${CYAN}Hash:${NC}    ${commit_hash:0:12}"
    echo -e "  ${CYAN}Author:${NC}  $author_name"
    echo -e "  ${CYAN}Date:${NC}    $author_date"
    echo -e "  ${CYAN}Subject:${NC} $subject"
    echo -e "  ${CYAN}Matching trailer(s):${NC}"
    while IFS= read -r line; do
      echo -e "    ${YELLOW}$line${NC}"
    done <<<"$matching_lines"
    echo ""

    # Show current decision if already set
    if [[ -v COMMIT_COAUTHOR_ACTIONS["$commit_hash"] ]]; then
      local current_action="${COMMIT_COAUTHOR_ACTIONS[$commit_hash]}"
      echo -e "  ${CYAN}Current decision:${NC} ${BOLD}$current_action${NC}"
      echo ""
    fi

    while true; do
      echo -e "  ${GREEN}[R]ewrite${NC} - Replace trailer with new Co-authored-by"
      echo -e "  ${RED}[D]elete${NC}  - Remove the matching trailer line(s)"
      echo -e "  ${BLUE}[S]kip${NC}   - Leave this commit unchanged"
      echo -e "  ${CYAN}[RA]${NC}     - Rewrite all remaining commits"
      echo -e "  ${CYAN}[DA]${NC}     - Delete trailers in all remaining commits"
      echo -e "  ${RED}[Q]uit${NC}   - Stop and apply decisions made so far"
      echo ""
      read -r -p "  Choice [R/D/S/RA/DA/Q]: " choice </dev/tty
      echo ""

      case "${choice^^}" in
      R)
        COMMIT_COAUTHOR_ACTIONS["$commit_hash"]="rewrite"
        log_success "Will rewrite Co-authored-by trailer in commit ${commit_hash:0:12}"
        break
        ;;
      D)
        COMMIT_COAUTHOR_ACTIONS["$commit_hash"]="remove"
        log_success "Will remove Co-authored-by trailer from commit ${commit_hash:0:12}"
        break
        ;;
      S)
        COMMIT_COAUTHOR_ACTIONS["$commit_hash"]="skip"
        log_info "Commit ${commit_hash:0:12} will be left unchanged"
        break
        ;;
      RA)
        COMMIT_COAUTHOR_ACTIONS["$commit_hash"]="rewrite"
        while IFS= read -r remaining_hash; do
          [[ -z "$remaining_hash" ]] && continue
          COMMIT_COAUTHOR_ACTIONS["$remaining_hash"]="rewrite"
        done <<<"$(echo "$matching_commits" | tail -n +$((current + 1)))"
        log_info "Will rewrite Co-authored-by trailers in all remaining commits"
        return
        ;;
      DA)
        COMMIT_COAUTHOR_ACTIONS["$commit_hash"]="remove"
        while IFS= read -r remaining_hash; do
          [[ -z "$remaining_hash" ]] && continue
          COMMIT_COAUTHOR_ACTIONS["$remaining_hash"]="remove"
        done <<<"$(echo "$matching_commits" | tail -n +$((current + 1)))"
        log_info "Will remove Co-authored-by trailers from all remaining commits"
        return
        ;;
      Q)
        log_info "Stopping Co-authored-by interactive mode. Decisions made so far will be applied."
        return
        ;;
      *)
        log_warning "Invalid choice. Please select R, D, S, RA, DA, or Q."
        ;;
      esac
    done
    echo ""
  done <<<"$matching_commits"
}

#-------------------------------------------------------------------------------
# Rewrite Co-authored-by Trailers (Batch)
#-------------------------------------------------------------------------------
rewrite_coauthors_batch() {
  [[ -z "$COAUTHOR_MODE" ]] && return
  [[ "$COAUTHOR_MODE" == "interactive" ]] && return

  log_header "Rewriting Co-authored-by Trailers"

  local coauthor_msg_filter="$TEMP_DIR/coauthor_msg_filter.sh"
  _build_coauthor_msg_filter "$coauthor_msg_filter" "batch"

  log_info "Applying Co-authored-by ${COAUTHOR_MODE} to all commits..."
  git filter-branch -f \
    --msg-filter "bash $coauthor_msg_filter" \
    --tag-name-filter cat -- --all

  log_success "Co-authored-by trailers updated"
}

#-------------------------------------------------------------------------------
# Rewrite Co-authored-by Trailers (Interactive)
#-------------------------------------------------------------------------------
rewrite_coauthors_interactive() {
  [[ "$COAUTHOR_MODE" != "interactive" ]] && return
  [[ ${#COMMIT_COAUTHOR_ACTIONS[@]} -eq 0 ]] && return

  log_header "Applying Co-authored-by Interactive Decisions"

  local coauthor_msg_filter="$TEMP_DIR/coauthor_msg_filter.sh"
  _build_coauthor_msg_filter "$coauthor_msg_filter" "interactive"

  log_info "Applying ${#COMMIT_COAUTHOR_ACTIONS[@]} Co-authored-by decision(s)..."
  git filter-branch -f \
    --msg-filter "bash $coauthor_msg_filter" \
    --tag-name-filter cat -- --all

  log_success "Co-authored-by trailers updated (interactive)"
}

#-------------------------------------------------------------------------------
# Build Co-authored-by Message Filter Script
# Usage: _build_coauthor_msg_filter <output_path> <batch|interactive>
#-------------------------------------------------------------------------------
_build_coauthor_msg_filter() {
  local out="$1"
  local mode_type="$2"

  # Escape old name/email for use inside sed's regex (BRE)
  local sed_escaped_name
  sed_escaped_name=$(printf '%s' "$COAUTHOR_OLD_NAME" | sed 's/[]\[\\/.^$*]/\\&/g')

  local sed_match_pattern
  if [[ -n "$COAUTHOR_OLD_EMAIL" ]]; then
    local sed_escaped_email
    sed_escaped_email=$(printf '%s' "$COAUTHOR_OLD_EMAIL" | sed 's/[]\[\\/.^$*]/\\&/g')
    sed_match_pattern="Co-authored-by: ${sed_escaped_name} <${sed_escaped_email}>"
  else
    # Match any email paired with the old name
    sed_match_pattern="Co-authored-by: ${sed_escaped_name} <[^>]*>"
  fi

  # Escape new name/email for use as sed replacement text
  local sed_escaped_new_name sed_escaped_new_email
  sed_escaped_new_name=$(printf '%s' "$COAUTHOR_NEW_NAME" | sed 's/[&\\\/]/\\&/g')
  sed_escaped_new_email=$(printf '%s' "$COAUTHOR_NEW_EMAIL" | sed 's/[&\\\/]/\\&/g')
  local new_trailer="Co-authored-by: ${sed_escaped_new_name} <${sed_escaped_new_email}>"

  cat >"$out" <<SCRIPT_HEADER
#!/usr/bin/env bash
# Co-authored-by message filter — generated by main.sh
input=\$(cat)
SCRIPT_HEADER

  if [[ "$mode_type" == "batch" ]]; then
    # Batch: apply the same action (COAUTHOR_MODE) to every commit
    if [[ "$COAUTHOR_MODE" == "remove" ]]; then
      cat >>"$out" <<SCRIPT_BODY
printf '%s' "\$input" | sed '/^${sed_match_pattern}\$/d'
SCRIPT_BODY
    else
      # rewrite
      cat >>"$out" <<SCRIPT_BODY
printf '%s' "\$input" | sed 's/^${sed_match_pattern}\$/${new_trailer}/'
SCRIPT_BODY
    fi
  else
    # Interactive: per-commit decisions stored in COMMIT_COAUTHOR_ACTIONS
    echo 'case "$GIT_COMMIT" in' >>"$out"

    for commit_hash in "${!COMMIT_COAUTHOR_ACTIONS[@]}"; do
      local action="${COMMIT_COAUTHOR_ACTIONS[$commit_hash]}"
      case "$action" in
      remove)
        cat >>"$out" <<CASE_BLOCK
  $commit_hash)
    printf '%s' "\$input" | sed '/^${sed_match_pattern}\$/d'
    ;;
CASE_BLOCK
        ;;
      rewrite)
        cat >>"$out" <<CASE_BLOCK
  $commit_hash)
    printf '%s' "\$input" | sed 's/^${sed_match_pattern}\$/${new_trailer}/'
    ;;
CASE_BLOCK
        ;;
      skip | *)
        cat >>"$out" <<CASE_BLOCK
  $commit_hash)
    printf '%s' "\$input"
    ;;
CASE_BLOCK
        ;;
      esac
    done

    cat >>"$out" <<CASE_DEFAULT
  *)
    printf '%s' "\$input"
    ;;
esac
CASE_DEFAULT
  fi

  chmod +x "$out"
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
# Generate Message Filter Script for Interactive Mode
#-------------------------------------------------------------------------------
generate_message_filter() {
  local msg_filter_script="$TEMP_DIR/msg_filter.sh"

  cat >"$msg_filter_script" <<'MSG_FILTER_HEADER'
#!/usr/bin/env bash
# Read the original message from stdin
original_msg=$(cat)
MSG_FILTER_HEADER

  # Check if there are any custom messages
  if [[ ${#COMMIT_MESSAGES[@]} -gt 0 ]]; then
    echo 'case "$GIT_COMMIT" in' >>"$msg_filter_script"

    for commit_hash in "${!COMMIT_MESSAGES[@]}"; do
      local custom_msg="${COMMIT_MESSAGES[$commit_hash]}"
      # Escape special characters for shell
      custom_msg="${custom_msg//\\/\\\\}"
      custom_msg="${custom_msg//\"/\\\"}"
      custom_msg="${custom_msg//\$/\\\$}"
      custom_msg="${custom_msg//\`/\\\`}"

      cat >>"$msg_filter_script" <<EOF
    $commit_hash)
        echo "$custom_msg"
        ;;
EOF
    done

    cat >>"$msg_filter_script" <<'MSG_FILTER_DEFAULT'
    *)
        echo "$original_msg"
        ;;
esac
MSG_FILTER_DEFAULT
  else
    # No custom messages, just pass through
    echo 'echo "$original_msg"' >>"$msg_filter_script"
  fi

  chmod +x "$msg_filter_script"
  echo "$msg_filter_script"
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

  # Add array of old names (printf %q ensures special chars like [bot] are safely escaped)
  echo 'OLD_NAMES_ARRAY=(' >>"$filter_script"
  for old_name in "${OLD_NAMES[@]}"; do
    printf '    %q\n' "$old_name" >>"$filter_script"
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

  if [[ ${#COMMIT_OVERRIDES[@]} -eq 0 && ${#COMMIT_MESSAGES[@]} -eq 0 ]]; then
    log_warning "No commits selected for modification"
    return
  fi

  local total_changes=$((${#COMMIT_OVERRIDES[@]} + ${#COMMIT_MESSAGES[@]}))
  log_info "Applying $total_changes commit modification(s)..."

  if [[ ${#COMMIT_OVERRIDES[@]} -gt 0 ]]; then
    log_info "  - ${#COMMIT_OVERRIDES[@]} author/email change(s)"
  fi
  if [[ ${#COMMIT_MESSAGES[@]} -gt 0 ]]; then
    log_info "  - ${#COMMIT_MESSAGES[@]} message change(s)"
  fi

  # Generate filter scripts
  local filter_script msg_filter_script
  filter_script=$(generate_interactive_filter)
  msg_filter_script=$(generate_message_filter)

  # Build filter-branch command with both filters
  # Use msg-filter only if there are message changes
  if [[ ${#COMMIT_MESSAGES[@]} -gt 0 ]]; then
    git filter-branch -f \
      --env-filter "source $filter_script" \
      --msg-filter "source $msg_filter_script" \
      --tag-name-filter cat -- --all
  else
    git filter-branch -f \
      --env-filter "source $filter_script" \
      --tag-name-filter cat -- --all
  fi

  log_success "Commit history rewritten successfully"
}

#-------------------------------------------------------------------------------
# Run Co-authored-by Phase
#-------------------------------------------------------------------------------
run_coauthor_phase() {
  [[ -z "$COAUTHOR_MODE" ]] && return

  case "$COAUTHOR_MODE" in
  rewrite | remove)
    rewrite_coauthors_batch
    ;;
  interactive)
    interactive_coauthor_edit
    rewrite_coauthors_interactive
    ;;
  esac
}

#-------------------------------------------------------------------------------
# Re-sign Tags
#-------------------------------------------------------------------------------
resign_tags() {
  # Collect all annotated tags
  local annotated_tags=()
  while IFS= read -r tag; do
    # git cat-file -t returns "tag" for annotated tags, "commit" for lightweight
    local obj_type
    obj_type=$(git cat-file -t "refs/tags/$tag" 2>/dev/null || true)
    if [[ "$obj_type" == "tag" ]]; then
      annotated_tags+=("$tag")
    fi
  done < <(git tag)

  if [[ ${#annotated_tags[@]} -eq 0 ]]; then
    log_info "No annotated tags found to process"
    return
  fi

  log_info "Processing ${#annotated_tags[@]} annotated tag(s)..."

  case "$SIGN_MODE" in
  none)
    log_info "Stripping signatures from annotated tags..."
    for tag in "${annotated_tags[@]}"; do
      local tag_message tag_commit
      tag_message=$(git tag -l --format='%(contents)' "$tag")
      tag_commit=$(git rev-list -1 "$tag")
      # Recreate tag without signature
      git tag -f -a "$tag" "$tag_commit" -m "$tag_message"
      log_info "  Stripped signature from tag: $tag"
    done
    log_success "Tag signatures removed"
    ;;
  gpg)
    log_info "Re-signing annotated tags with GPG key: $GPG_KEY..."
    for tag in "${annotated_tags[@]}"; do
      local tag_message tag_commit
      tag_message=$(git tag -l --format='%(contents)' "$tag")
      tag_commit=$(git rev-list -1 "$tag")
      # Recreate tag with GPG signature
      git tag -f -s "$tag" "$tag_commit" -u "$GPG_KEY" -m "$tag_message"
      log_info "  Re-signed tag: $tag"
    done
    log_success "Tags re-signed with GPG"
    ;;
  ssh)
    log_info "Re-signing annotated tags with SSH key: $SSH_KEY..."
    for tag in "${annotated_tags[@]}"; do
      local tag_message tag_commit
      tag_message=$(git tag -l --format='%(contents)' "$tag")
      tag_commit=$(git rev-list -1 "$tag")
      # Recreate tag with SSH signature
      git tag -f -s "$tag" "$tag_commit" -m "$tag_message"
      log_info "  Re-signed tag: $tag"
    done
    log_success "Tags re-signed with SSH"
    ;;
  esac
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
    resign_tags
    ;;
  gpg)
    log_header "Re-signing with GPG"
    log_info "Re-signing all commits with GPG key: $GPG_KEY"

    git config user.signingkey "$GPG_KEY"

    git filter-branch -f --commit-filter "
                git commit-tree -S\"$GPG_KEY\" \"\$@\"
            " --tag-name-filter cat -- --all

    log_success "Commits re-signed with GPG"
    resign_tags
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
    resign_tags
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

  if [[ -n "$COAUTHOR_MODE" ]]; then
    case "$COAUTHOR_MODE" in
    rewrite)
      echo -e "  ${GREEN}✓${NC} Co-authored-by trailers rewritten to '$COAUTHOR_NEW_NAME <$COAUTHOR_NEW_EMAIL>'"
      ;;
    remove)
      echo -e "  ${GREEN}✓${NC} Matching Co-authored-by trailers removed"
      ;;
    interactive)
      local rewrite_count=0 remove_count=0
      for action in "${COMMIT_COAUTHOR_ACTIONS[@]}"; do
        [[ "$action" == "rewrite" ]] && rewrite_count=$((rewrite_count + 1))
        [[ "$action" == "remove" ]]  && remove_count=$((remove_count + 1))
      done
      echo -e "  ${GREEN}✓${NC} Co-authored-by trailers: ${rewrite_count} rewritten, ${remove_count} removed"
      ;;
    esac
  fi

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

  run_coauthor_phase
  handle_signing
  push_to_destination
  show_summary
}

main "$@"
