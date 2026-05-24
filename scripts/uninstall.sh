#!/usr/bin/env bash
set -euo pipefail

MODE="keep-data"
CONFIRM_PURGE="false"
DRY_RUN="false"

BIN_PATH="/usr/local/bin/omo"
OMOCTL_BIN_PATH="/usr/local/bin/omoctl"
CONFIG_DIR="/etc/omo"
DATA_DIR="/var/lib/omo"
LOG_DIR="/var/log/omo"
BACKUP_DIR="/var/backups/omo"
CADDY_CONFIG_DIR="/etc/caddy/omo"
UNIT_PATH="/etc/systemd/system/omo.service"
INIT_UNIT_PATH="/etc/systemd/system/omo-init.service"
INIT_WATCH_UNIT_PATH="/etc/systemd/system/omo-init-watch.service"

usage() {
  cat <<'USAGE'
OMO safe uninstall

Usage:
  uninstall.sh [--keep-data] [--purge --confirm-purge] [--dry-run]

Default mode stops OMO services and removes service units and binaries while
preserving configuration, data, logs, and backups. Destructive data removal
requires both --purge and --confirm-purge.
USAGE
}

log() {
  printf '[omo-uninstall] %s\n' "$*"
}

fail() {
  printf '[omo-uninstall] ERROR: %s\n' "$*" >&2
  exit 1
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '[omo-uninstall] dry-run:'
    printf ' %q' "$@"
    printf '\n'
  else
    "$@"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep-data)
      MODE="keep-data"
      shift
      ;;
    --purge)
      MODE="purge"
      shift
      ;;
    --confirm-purge)
      CONFIRM_PURGE="true"
      shift
      ;;
    --dry-run)
      DRY_RUN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

if [[ "$MODE" == "purge" && "$CONFIRM_PURGE" != "true" ]]; then
  fail "purge mode requires --confirm-purge"
fi

stop_unit() {
  local unit="$1"
  if command -v systemctl >/dev/null 2>&1; then
    run systemctl disable --now "$unit" || true
  fi
}

remove_file() {
  local path="$1"
  if [[ -e "$path" || -L "$path" ]]; then
    run rm -f "$path"
  fi
}

remove_dir() {
  local path="$1"
  if [[ -d "$path" ]]; then
    run rm -rf "$path"
  fi
}

main() {
  if [[ "$DRY_RUN" != "true" && "${EUID:-$(id -u)}" -ne 0 ]]; then
    fail "please run as root"
  fi

  log "mode: ${MODE}"
  stop_unit omo-init-watch.service
  stop_unit omo-init.service
  stop_unit omo.service

  remove_file "$UNIT_PATH"
  remove_file "$INIT_UNIT_PATH"
  remove_file "$INIT_WATCH_UNIT_PATH"
  remove_file "$BIN_PATH"
  remove_file "$OMOCTL_BIN_PATH"
  remove_file "${CONFIG_DIR}/init.env"
  remove_file "${CONFIG_DIR}/init-link.txt"

  if command -v systemctl >/dev/null 2>&1; then
    run systemctl daemon-reload
  fi

  if [[ "$MODE" == "purge" ]]; then
    remove_dir "$CONFIG_DIR"
    remove_dir "$DATA_DIR"
    remove_dir "$LOG_DIR"
    remove_dir "$BACKUP_DIR"
    remove_dir "$CADDY_CONFIG_DIR"
    if id -u omo >/dev/null 2>&1; then
      run userdel omo || true
    fi
    log "purge complete"
  else
    log "uninstall complete; preserved ${CONFIG_DIR}, ${DATA_DIR}, ${LOG_DIR}, ${BACKUP_DIR}, and ${CADDY_CONFIG_DIR}"
  fi
}

main "$@"
