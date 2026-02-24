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
# Resolve the directory this script lives in (follows symlinks)
#-------------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$SCRIPT_DIR/lib"

#-------------------------------------------------------------------------------
# Source library modules
#
# Order matters: logging first (used by everything), then config (global state),
# then the rest which depend on both.
#-------------------------------------------------------------------------------
source "$LIB_DIR/logging.sh"
source "$LIB_DIR/config.sh"
source "$LIB_DIR/args.sh"
source "$LIB_DIR/git_ops.sh"
source "$LIB_DIR/rewrite.sh"
source "$LIB_DIR/coauthor.sh"
source "$LIB_DIR/signing.sh"
source "$LIB_DIR/ui.sh"

#-------------------------------------------------------------------------------
# Register cleanup trap
#-------------------------------------------------------------------------------
trap cleanup EXIT

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
