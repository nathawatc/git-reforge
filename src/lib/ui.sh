#!/usr/bin/env bash
#===============================================================================
# UI — Display Configuration, Summary & Interactive Commit Editing
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_UI_LOADED:-}" ]] && return 0
readonly _LIB_UI_LOADED=1

#-------------------------------------------------------------------------------
# Display Configuration
#
# Prints a human-readable summary of all migration settings and prompts the
# user for confirmation before proceeding.
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
#
# Walks through every commit authored by one of the OLD_NAMES entries and
# presents the user with per-commit options:
#   [E]dit    — customise the author name/email for this commit
#   [M]essage — replace the commit message
#   [A]pply   — use the default NEW_NAME / NEW_EMAIL
#   [S]kip    — leave the commit unchanged
#   [D]efault — apply defaults to all remaining commits
#   [Q]uit    — stop reviewing; changes collected so far are kept
#
# Populates: COMMIT_OVERRIDES  (associative array: hash -> "name|email"|"SKIP")
#            COMMIT_MESSAGES   (associative array: hash -> new message)
#-------------------------------------------------------------------------------
interactive_edit() {
  log_header "Interactive Commit Editing"

  log_info "Fetching commits from author(s): ${OLD_NAMES[*]}"

  # Get list of commits by all old authors
  local commits=""
  for old_name in "${OLD_NAMES[@]}"; do
    local author_commits
    # Escape regex metacharacters (e.g. [bot]) so git log treats them literally
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
# Summary
#
# Prints a final report of everything the migration accomplished.
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
