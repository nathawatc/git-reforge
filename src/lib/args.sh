#!/usr/bin/env bash
#===============================================================================
# Argument Parsing & Validation
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_ARGS_LOADED:-}" ]] && return 0
readonly _LIB_ARGS_LOADED=1

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
    -n, --new-name <name>    New author name
    -o, --old-name <name>    Old author name(s) to replace. Can be specified multiple times
                             or as comma-separated values. Required.
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
                     --coauthor-mode remove --coauthor-old "OldName"

    # Rewrite Co-authored-by trailers with a specific old email to narrow the match
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     --coauthor-mode rewrite \\
                     --coauthor-old "OldName" \\
                     --coauthor-old-email "oldname@users.noreply.github.com" \\
                     --coauthor-new-name "NewName" \\
                     --coauthor-new-email "myuser@noreply.codeberg.org"

    # Interactive Co-authored-by review
    $(basename "$0") -s https://github.com/user/repo.git \\
                     -d https://codeberg.org/user/repo.git \\
                     -e myuser@noreply.codeberg.org \\
                     --coauthor-mode interactive --coauthor-old "OldName"

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
        name="$(printf '%s' "${name}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
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

  # --old-name is required; there is no built-in default
  if [[ ${#OLD_NAMES[@]} -eq 0 ]]; then
    log_error "At least one old author name is required (-o, --old-name)"
    echo ""
    echo "Use --help for usage information"
    exit 1
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
