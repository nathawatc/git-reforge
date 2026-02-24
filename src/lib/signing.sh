#!/usr/bin/env bash
#===============================================================================
# Commit & Tag Signing — Re-sign, Strip, or Preserve Signatures
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_SIGNING_LOADED:-}" ]] && return 0
readonly _LIB_SIGNING_LOADED=1

#-------------------------------------------------------------------------------
# Re-sign Tags
#
# Iterates over all annotated tags and either strips their signatures or
# re-signs them with the configured GPG/SSH key, depending on SIGN_MODE.
# Lightweight tags are left untouched.
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
#
# Processes all commits according to SIGN_MODE:
#   keep — do nothing (original signatures are now invalid but preserved)
#   none — strip all commit signatures via a no-op commit-filter
#   gpg  — re-sign every commit with the configured GPG key
#   ssh  — re-sign every commit with the configured SSH key
#
# After processing commits, annotated tags are handled by resign_tags().
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
