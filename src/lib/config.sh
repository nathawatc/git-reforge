#!/usr/bin/env bash
#===============================================================================
# Global Configuration & State
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_CONFIG_LOADED:-}" ]] && return 0
readonly _LIB_CONFIG_LOADED=1

#-------------------------------------------------------------------------------
# Default Configuration
#-------------------------------------------------------------------------------
OLD_NAMES=()           # Array of old names to replace
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
# Interactive State
#
# These associative arrays store per-commit decisions made during interactive
# editing sessions. They are populated by interactive_edit() and
# interactive_coauthor_edit(), then consumed by the rewrite functions.
#-------------------------------------------------------------------------------
declare -A COMMIT_OVERRIDES
declare -A COMMIT_MESSAGES
# Coauthor interactive decisions: hash -> "rewrite"|"remove"|"skip"
declare -A COMMIT_COAUTHOR_ACTIONS
