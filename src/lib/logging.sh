#!/usr/bin/env bash
#===============================================================================
# Logging & Color Constants
#===============================================================================

# Guard against double-sourcing
[[ -n "${_LIB_LOGGING_LOADED:-}" ]] && return 0
readonly _LIB_LOGGING_LOADED=1

#-------------------------------------------------------------------------------
# Colors and Formatting
#-------------------------------------------------------------------------------
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly BOLD='\033[1m'
readonly NC='\033[0m' # No Color

#-------------------------------------------------------------------------------
# Logging Functions
#-------------------------------------------------------------------------------
log_info() {
  printf "${BLUE}[INFO]${NC} %s\n" "$1"
}

log_success() {
  printf "${GREEN}[SUCCESS]${NC} %s\n" "$1"
}

log_warning() {
  printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
}

log_error() {
  printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
}

log_header() {
  printf '\n'
  printf "${BOLD}${CYAN}═══════════════════════════════════════════════════════════════${NC}\n"
  printf "${BOLD}${CYAN}  %s${NC}\n" "$1"
  printf "${BOLD}${CYAN}═══════════════════════════════════════════════════════════════${NC}\n"
  printf '\n'
}
