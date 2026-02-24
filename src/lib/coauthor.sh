#!/usr/bin/env bash
#===============================================================================
# Co-authored-by Trailer Rewriting — Batch, Interactive & Filter Generation
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_COAUTHOR_LOADED:-}" ]] && return 0
readonly _LIB_COAUTHOR_LOADED=1

#-------------------------------------------------------------------------------
# Interactive Co-authored-by Editor
#
# Scans every commit for Co-authored-by trailers matching COAUTHOR_OLD_NAME
# (and optionally COAUTHOR_OLD_EMAIL), then prompts the user to decide per
# commit whether to rewrite, remove, or skip the trailer.
#
# Populates: COMMIT_COAUTHOR_ACTIONS (associative array)
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
  commit_count=$(printf '%s' "$matching_commits" | wc -l)
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
# Build Co-authored-by Message Filter Script
#
# Generates a shell script consumed by `git filter-branch --msg-filter` that
# either rewrites or removes matching Co-authored-by trailer lines.
#
# Usage: _build_coauthor_msg_filter <output_path> <batch|interactive>
#
# In "batch" mode the same action (COAUTHOR_MODE) is applied to every commit.
# In "interactive" mode the per-commit decisions stored in
# COMMIT_COAUTHOR_ACTIONS are emitted as a case statement.
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
# Rewrite Co-authored-by Trailers (Batch)
#
# Applies the same action (rewrite or remove) to every commit in the
# repository.  Skipped when COAUTHOR_MODE is empty or "interactive".
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
#
# Applies per-commit decisions previously collected by
# interactive_coauthor_edit().  Each commit hash maps to one of
# "rewrite", "remove", or "skip" in COMMIT_COAUTHOR_ACTIONS.
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
# Run Co-authored-by Phase
#
# Dispatches to the appropriate handler based on COAUTHOR_MODE.  This is the
# single entry-point called from main.
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
