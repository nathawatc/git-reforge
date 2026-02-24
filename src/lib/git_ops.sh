#!/usr/bin/env bash
#===============================================================================
# Git Operations — Clone, Push, Save, Dependency Check, Cleanup
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_GIT_OPS_LOADED:-}" ]] && return 0
readonly _LIB_GIT_OPS_LOADED=1

#-------------------------------------------------------------------------------
# Cleanup Function
#-------------------------------------------------------------------------------
cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    log_info "Cleaning up temporary directory..."
    rm -rf "$TEMP_DIR"
  fi
}

#-------------------------------------------------------------------------------
# Save Repository
#
# Copies the rewritten working repository to SAVE_DIR as a standard git
# repository (non-bare). The destination directory must not already exist;
# this is validated by validate_args() before we ever reach this function.
#-------------------------------------------------------------------------------
save_repository() {
  log_header "Saving Rewritten Repository"

  log_info "Copying rewritten repository to: $SAVE_DIR"

  # cp -r preserves all git internals. We copy the working tree, not the
  # intermediate bare repo, so the saved copy is immediately usable.
  if ! cp -r "$TEMP_DIR/working" "$SAVE_DIR"; then
    log_error "Failed to save repository to: $SAVE_DIR"
    exit 1
  fi

  log_success "Repository saved to: $SAVE_DIR"
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

  if [[ "$SIGN_MODE" == "ssh" ]]; then
    if ! command -v ssh-keygen &>/dev/null; then
      log_error "ssh-keygen is not installed (required for SSH signing)"
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

  # Convert bare repo to normal repo for easier manipulation.
  # Scoped to a subshell so the cd calls do not change the caller's working
  # directory as a side effect.
  log_info "Converting to working repository..."
  if ! (
    cd "$TEMP_DIR"
    if ! git clone repo.git working; then
      log_error "Failed to convert mirror to working repository"
      exit 1
    fi
    cd working
    git fetch --all
  ); then
    log_error "Failed to prepare working repository"
    exit 1
  fi

  log_success "Repository cloned successfully"
}

#-------------------------------------------------------------------------------
# Push to Codeberg
#
# On failure the user is prompted to either:
#   [S]ave — copy the rewritten working tree to a path they specify, or
#   [C]leanup — let the EXIT trap discard the temporary directory normally.
#-------------------------------------------------------------------------------
push_to_destination() {
  log_header "Pushing to Codeberg"

  log_info "Setting remote URL to: $DEST_URL"
  git remote set-url origin "$DEST_URL"

  log_info "Pushing all branches..."
  if ! git push --all --force; then
    log_error "Failed to push branches to Codeberg"
    _prompt_save_or_cleanup "branch push"
    exit 1
  fi

  log_info "Pushing all tags..."
  # Use the explicit refspec instead of --tags so that re-created annotated tag
  # objects (whose SHA changed after resign_tags) are pushed correctly.
  # --tags only pushes tags that are missing on the remote; it silently skips
  # tags whose local object SHA differs from the remote's, dropping re-signed tags.
  if ! git push --force origin "refs/tags/*:refs/tags/*"; then
    log_warning "Failed to push some tags (this may be expected if no tags exist)"
  fi

  log_success "Repository pushed to Codeberg successfully"
}

#-------------------------------------------------------------------------------
# _prompt_save_or_cleanup  (private helper)
#
# Called after a push failure. Gives the user an interactive choice to either
# save the already-rewritten working tree to a local path, or let cleanup run.
# The $1 argument is a short description of the step that failed, used only in
# the prompt text.
#-------------------------------------------------------------------------------
_prompt_save_or_cleanup() {
  local failed_step="${1:-push}"

  # If SAVE_DIR was already set via --save-dir the repo will be saved by
  # main() regardless — just inform the user and return so exit 1 proceeds.
  if [[ -n "$SAVE_DIR" ]]; then
    log_warning "Push failed during $failed_step."
    log_info "The rewritten repository will still be saved to: $SAVE_DIR"
    return
  fi

  echo ""
  echo -e "${YELLOW}[WARNING]${NC} The $failed_step to Codeberg failed." >&2
  echo -e "${BOLD}The rewritten repository is still available in the temporary directory.${NC}" >&2
  echo "" >&2
  echo -e "  ${YELLOW}[S]ave${NC}    — copy the rewritten repository to a local path" >&2
  echo -e "  ${RED}[C]leanup${NC} — discard the temporary directory and exit" >&2
  echo "" >&2

  local choice
  while true; do
    read -r -p "  Choice [S/C]: " choice </dev/tty
    case "${choice^^}" in
    S)
      local save_path
      while true; do
        read -r -p "  Enter destination path: " save_path </dev/tty
        if [[ -z "$save_path" ]]; then
          log_warning "Path cannot be empty. Please enter a valid directory path."
        elif [[ -e "$save_path" ]]; then
          log_warning "Path already exists: $save_path — choose a path that does not exist."
        else
          break
        fi
      done

      log_info "Saving rewritten repository to: $save_path"
      if cp -r "$TEMP_DIR/working" "$save_path"; then
        log_success "Repository saved to: $save_path"
        log_info "You can push it later with:"
        printf '    cd %s\n' "$save_path" >&2
        printf '    git remote set-url origin %s\n' "$DEST_URL" >&2
        printf '    git push --all --force\n' >&2
        printf '    git push --force origin "refs/tags/*:refs/tags/*"\n' >&2
      else
        log_error "Failed to save repository to: $save_path"
        log_info "The temporary directory will be cleaned up on exit."
      fi
      return
      ;;
    C)
      log_info "Cleanup selected. The temporary directory will be removed on exit."
      return
      ;;
    *)
      log_warning "Invalid choice. Please enter S to save or C to clean up."
      ;;
    esac
  done
}
