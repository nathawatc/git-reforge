#!/usr/bin/env bash
#===============================================================================
# Commit Rewriting — Batch & Interactive Filter Generation
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_REWRITE_LOADED:-}" ]] && return 0
readonly _LIB_REWRITE_LOADED=1

#-------------------------------------------------------------------------------
# Generate Filter Script for Interactive Mode
#
# Builds a shell script that maps commit hashes to per-commit author/email
# overrides based on decisions stored in COMMIT_OVERRIDES.
#
# Outputs: the path to the generated filter script
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

  # Default arm: commits not in COMMIT_OVERRIDES are left untouched
  cat >>"$filter_script" <<'FILTER_DEFAULT'
    *)
        ;;
FILTER_DEFAULT
  echo 'esac' >>"$filter_script"

  chmod +x "$filter_script"
  echo "$filter_script"
}

#-------------------------------------------------------------------------------
# Generate Message Filter Script for Interactive Mode
#
# Builds a shell script that replaces commit messages for specific commits
# based on decisions stored in COMMIT_MESSAGES.
#
# Outputs: the path to the generated message filter script
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
      # Write the message to a sidecar file so the filter script can read it
      # safely without any quoting, escaping, or truncation issues.
      local msg_file="$TEMP_DIR/msg_${commit_hash}.txt"
      printf '%s' "$custom_msg" >"$msg_file"

      cat >>"$msg_filter_script" <<EOF
    $commit_hash)
        cat "$msg_file"
        ;;
EOF
    done

    cat >>"$msg_filter_script" <<'MSG_FILTER_DEFAULT'
    *)
        printf '%s' "$original_msg"
        ;;
esac
MSG_FILTER_DEFAULT
  else
    # No custom messages, just pass through
    echo 'printf "%s" "$original_msg"' >>"$msg_filter_script"
  fi

  chmod +x "$msg_filter_script"
  echo "$msg_filter_script"
}

#-------------------------------------------------------------------------------
# Rewrite Commits (Batch Mode)
#
# Rewrites author/committer identity for every commit whose author name
# matches one of the OLD_NAMES entries.  Uses git filter-branch with a
# generated env-filter script.
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
#
# Applies per-commit overrides collected by interactive_edit().  Uses the
# filter scripts produced by generate_interactive_filter() and
# generate_message_filter().
#-------------------------------------------------------------------------------
rewrite_commits_interactive() {
  log_header "Rewriting Commit History (Interactive)"

  if [[ ${#COMMIT_OVERRIDES[@]} -eq 0 && ${#COMMIT_MESSAGES[@]} -eq 0 ]]; then
    log_warning "No commits selected for modification"
    return
  fi

  log_info "Applying $((${#COMMIT_OVERRIDES[@]} + ${#COMMIT_MESSAGES[@]})) commit modification(s)..."

  if [[ ${#COMMIT_OVERRIDES[@]} -gt 0 ]]; then
    log_info "  - ${#COMMIT_OVERRIDES[@]} author/email change(s)"
  fi
  if [[ ${#COMMIT_MESSAGES[@]} -gt 0 ]]; then
    log_info "  - ${#COMMIT_MESSAGES[@]} message change(s)"
  fi

  # Generate filter scripts only for the passes that are actually needed
  local filter_script msg_filter_script
  msg_filter_script=$(generate_message_filter)

  # Run env-filter pass only when there are author/email overrides to apply.
  # When COMMIT_OVERRIDES is empty the generated script would be a vacuous
  # case block that rewrites every commit object for no reason.
  if [[ ${#COMMIT_OVERRIDES[@]} -gt 0 ]]; then
    filter_script=$(generate_interactive_filter)

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
  else
    # Only message changes — no identity rewriting needed
    git filter-branch -f \
      --msg-filter "source $msg_filter_script" \
      --tag-name-filter cat -- --all
  fi

  log_success "Commit history rewritten successfully"
}
