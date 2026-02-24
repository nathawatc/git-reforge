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

  printf "  ${BOLD}Source:${NC}        %s\n" "$SOURCE_URL"
  if [[ -n "$DEST_URL" ]]; then
    printf "  ${BOLD}Destination:${NC}   %s\n" "$DEST_URL"
  else
    printf "  ${BOLD}Destination:${NC}   ${YELLOW}(none — push skipped)${NC}\n"
  fi
  echo ""
  if [[ ${#OLD_NAMES[@]} -eq 1 ]]; then
    printf "  ${BOLD}Old Author:${NC}    %s\n" "${OLD_NAMES[0]}"
  else
    printf "  ${BOLD}Old Authors:${NC}   %s\n" "${OLD_NAMES[0]}"
    for ((i = 1; i < ${#OLD_NAMES[@]}; i++)); do
      printf "                 %s\n" "${OLD_NAMES[$i]}"
    done
  fi
  printf "  ${BOLD}New Author:${NC}    %s\n" "$NEW_NAME"
  printf "  ${BOLD}New Email:${NC}     %s\n" "$NEW_EMAIL"
  echo ""
  printf "  ${BOLD}Interactive:${NC}   %s\n" "$INTERACTIVE"
  printf "  ${BOLD}Sign Mode:${NC}     %s\n" "$SIGN_MODE"
  [[ -n "$GPG_KEY" ]] && printf "  ${BOLD}GPG Key:${NC}       %s\n" "$GPG_KEY"
  [[ -n "$SSH_KEY" ]] && printf "  ${BOLD}SSH Key:${NC}       %s\n" "$SSH_KEY"
  printf "  ${BOLD}Push to Dest:${NC}  %s\n" "$( [[ "$NO_PUSH" == true ]] && printf 'no (--no-push)' || printf 'yes' )"
  [[ -n "$SAVE_DIR" ]] && printf "  ${BOLD}Save To:${NC}       %s\n" "$SAVE_DIR"
  if [[ -n "$COAUTHOR_MODE" ]]; then
    echo ""
    printf "  ${BOLD}Co-author Mode:${NC}  %s\n" "$COAUTHOR_MODE"
    local coauthor_match="$COAUTHOR_OLD_NAME"
    [[ -n "$COAUTHOR_OLD_EMAIL" ]] && coauthor_match+=" <$COAUTHOR_OLD_EMAIL>"
    printf "  ${BOLD}Co-author Match:${NC} %s\n" "$coauthor_match"
    if [[ "$COAUTHOR_MODE" == "rewrite" ]]; then
      printf "  ${BOLD}Co-author New:${NC}   %s <%s>\n" "$COAUTHOR_NEW_NAME" "$COAUTHOR_NEW_EMAIL"
    fi
  fi
  echo ""

  if [[ "$NO_PUSH" == true && -z "$SAVE_DIR" ]]; then
    printf "  ${YELLOW}Note:${NC} --no-push is set and --save-dir is not. The rewritten\n"
    printf "  ${YELLOW}      ${NC} repository will only exist in the temporary directory\n"
    printf "  ${YELLOW}      ${NC} until the script exits. Use --save-dir <path> to keep it.\n"
    echo ""
  fi

  read -r -p "Proceed with migration? [y/N] " -n 1 REPLY </dev/tty
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
  commits=$(printf '%s' "$commits" | sort -u)

  if [[ -z "$commits" ]]; then
    log_warning "No commits found from author(s): ${OLD_NAMES[*]}"
    return
  fi

  local commit_count
  commit_count=$(printf '%s' "$commits" | wc -l)
  log_info "Found $commit_count commit(s) to review"
  echo ""

  local current=0
  while IFS= read -r commit_hash; do
    current=$((current + 1))

    # Get commit details
    local author_name author_email author_date message
    author_name=$(git log -1 --format="%an" "$commit_hash")
    author_email=$(git log -1 --format="%ae" "$commit_hash")
    author_date=$(git log -1 --format="%ai" "$commit_hash")
    message=$(git log -1 --format="%s" "$commit_hash")

    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    printf "${BOLD}Commit %s of %s${NC}\n" "$current" "$commit_count"
    echo -e "${BOLD}────────────────────────────────────────────────────────────────${NC}"
    printf "  ${CYAN}Hash:${NC}       %s\n" "${commit_hash:0:12}"
    printf "  ${CYAN}Author:${NC}     %s <%s>\n" "$author_name" "$author_email"
    printf "  ${CYAN}Date:${NC}       %s\n" "$author_date"
    printf "  ${CYAN}Message:${NC}    %s\n" "$message"
    echo ""
    # Check if message has been customized
    local msg_status="(unchanged)"
    if [[ -v COMMIT_MESSAGES["$commit_hash"] ]]; then
      msg_status="${YELLOW}(customized)${NC}"
    fi
    echo -e "  ${BOLD}Default changes:${NC}"
    printf "    Name:    %s → %s\n" "$author_name" "$NEW_NAME"
    printf "    Email:   %s → %s\n" "$author_email" "$NEW_EMAIL"
    printf "    Message: %s\n" "$msg_status"
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

        COMMIT_OVERRIDES["$commit_hash"]="${custom_name}"$'\x1F'"${custom_email}"
        log_success "Custom name/email saved for commit ${commit_hash:0:12}"
        echo ""
        ;;
      M)
        echo ""
        echo -e "  ${CYAN}Current message:${NC}"
        printf "    %s\n" "$message"
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
        COMMIT_OVERRIDES["$commit_hash"]="${NEW_NAME}"$'\x1F'"${NEW_EMAIL}"
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
        COMMIT_OVERRIDES["$commit_hash"]="${NEW_NAME}"$'\x1F'"${NEW_EMAIL}"
        while IFS= read -r remaining_hash; do
          COMMIT_OVERRIDES["$remaining_hash"]="${NEW_NAME}"$'\x1F'"${NEW_EMAIL}"
        done <<<"$(printf '%s' "$commits" | tail -n +$((current + 1)))"
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
    printf "  ${GREEN}✓${NC} Author '%s' changed to '%s'\n" "${OLD_NAMES[0]}" "$NEW_NAME"
  else
    printf "  ${GREEN}✓${NC} Authors changed to '%s':\n" "$NEW_NAME"
    for old_name in "${OLD_NAMES[@]}"; do
      printf "      - %s\n" "$old_name"
    done
  fi
  printf "  ${GREEN}✓${NC} Email changed to '%s'\n" "$NEW_EMAIL"
  echo -e "  ${GREEN}✓${NC} Original commit dates preserved"

  if [[ -n "$COAUTHOR_MODE" ]]; then
    case "$COAUTHOR_MODE" in
    rewrite)
      printf "  ${GREEN}✓${NC} Co-authored-by trailers rewritten to '%s <%s>'\n" \
        "$COAUTHOR_NEW_NAME" "$COAUTHOR_NEW_EMAIL"
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
      printf "  ${GREEN}✓${NC} Co-authored-by trailers: %s rewritten, %s removed\n" \
        "$rewrite_count" "$remove_count"
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
    printf "  ${GREEN}✓${NC} Commits re-signed with GPG key: %s\n" "$GPG_KEY"
    ;;
  ssh)
    printf "  ${GREEN}✓${NC} Commits re-signed with SSH key: %s\n" "$SSH_KEY"
    ;;
  esac

  echo ""
  if [[ "$NO_PUSH" == true ]]; then
    echo -e "  ${YELLOW}!${NC} Push to Codeberg was skipped"
  else
    printf "  ${BOLD}Codeberg Repository:${NC} %s\n" "$DEST_URL"
  fi

  if [[ -n "$SAVE_DIR" ]]; then
    printf "  ${GREEN}✓${NC} Rewritten repository saved to: %s\n" "$SAVE_DIR"
    if [[ "$NO_PUSH" == true && -n "$DEST_URL" ]]; then
      echo ""
      echo -e "  ${CYAN}To push later, run:${NC}"
      printf '    cd %s\n' "$SAVE_DIR"
      printf '    git remote set-url origin %s\n' "$DEST_URL"
      printf '    git push --all --force\n'
      printf '    git push --force origin "refs/tags/*:refs/tags/*"\n'
    fi
  fi

  echo ""
}
