#!/usr/bin/env bash
#===============================================================================
# Git Operations — Clone, Push, Dependency Check, Cleanup
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

  # Convert bare repo to normal repo for easier manipulation.
  # Scoped to a subshell so the cd calls do not change the caller's working
  # directory as a side effect.
  log_info "Converting to working repository..."
  (
    cd "$TEMP_DIR"
    if ! git clone repo.git working; then
      log_error "Failed to convert mirror to working repository"
      exit 1
    fi
    cd working
    git fetch --all
  )

  log_success "Repository cloned successfully"
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
  # Use the explicit refspec instead of --tags so that re-created annotated tag
  # objects (whose SHA changed after resign_tags) are pushed correctly.
  # --tags only pushes tags that are missing on the remote; it silently skips
  # tags whose local object SHA differs from the remote's, dropping re-signed tags.
  if ! git push --force origin "refs/tags/*:refs/tags/*"; then
    log_warning "Failed to push some tags (this may be expected if no tags exist)"
  fi

  log_success "Repository pushed to Codeberg successfully"
}
