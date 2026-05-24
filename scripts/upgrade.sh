#!/usr/bin/env bash
set -euo pipefail

CHANNEL="stable"
VERSION="latest"
DRY_RUN="false"
SKIP_BACKUP="false"

RELEASE_OWNER="${OMO_RELEASE_OWNER:-clover-eric}"
RELEASE_REPO="${OMO_RELEASE_REPO:-omo}"
BIN_PATH="/usr/local/bin/omo"
OMOCTL_BIN_PATH="/usr/local/bin/omoctl"
BACKUP_DIR="/var/backups/omo"
SERVICE_NAME="omo.service"
HEALTH_URL="http://127.0.0.1:8080/api/system/health"

usage() {
  cat <<'USAGE'
OMO upgrade

Usage:
  upgrade.sh [--channel stable|beta|nightly] [--version VERSION] [--skip-backup] [--dry-run]

Downloads a release archive, verifies checksums, preserves current binaries,
restarts OMO, checks local service health, and restores previous binaries if
validation fails.
USAGE
}

log() {
  printf '[omo-upgrade] %s\n' "$*"
}

fail() {
  printf '[omo-upgrade] ERROR: %s\n' "$*" >&2
  exit 1
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '[omo-upgrade] dry-run:'
    printf ' %q' "$@"
    printf '\n'
  else
    "$@"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --channel)
      CHANNEL="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --skip-backup)
      SKIP_BACKUP="true"
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

case "$CHANNEL" in
  stable|beta|nightly) ;;
  *) fail "unsupported channel: $CHANNEL" ;;
esac

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *) fail "unsupported CPU architecture: $(uname -m)" ;;
  esac
}

check_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command missing: $1"
}

resolve_release_tag() {
  local requested="$1" tag
  if [[ "$requested" != "latest" ]]; then
    printf '%s' "$requested"
    return
  fi
  if [[ "$DRY_RUN" == "true" ]]; then
    printf 'latest'
    return
  fi
  tag="$(curl -fsSL --max-time 8 "https://api.github.com/repos/${RELEASE_OWNER}/${RELEASE_REPO}/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1 || true)"
  [[ -n "$tag" ]] || fail "could not resolve latest OMO release tag"
  printf '%s' "$tag"
}

wait_for_health() {
  if [[ "$DRY_RUN" == "true" ]]; then
    log "dry-run: would verify ${HEALTH_URL}"
    return 0
  fi

  for _ in $(seq 1 30); do
    if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

restart_service() {
  run systemctl restart "$SERVICE_NAME"
}

restore_from_backup() {
  local backup_path="$1"
  log "restoring previous binaries from ${backup_path}"
  if [[ -f "${backup_path}/omo" ]]; then
    run install -m 0755 "${backup_path}/omo" "$BIN_PATH"
  fi
  if [[ -f "${backup_path}/omoctl" ]]; then
    run install -m 0755 "${backup_path}/omoctl" "$OMOCTL_BIN_PATH"
  fi
  restart_service || true
}

main() {
  if [[ "$DRY_RUN" != "true" && "${EUID:-$(id -u)}" -ne 0 ]]; then
    fail "please run as root"
  fi

  check_command uname
  check_command curl
  check_command tar
  check_command install
  check_command sed
  check_command sha256sum
  check_command systemctl

  local arch release_tag archive_version archive_name tmp url checksum_url backup_path timestamp
  arch="$(detect_arch)"
  release_tag="$(resolve_release_tag "$VERSION")"
  archive_version="${release_tag#v}"
  archive_name="omo_${archive_version}_linux_${arch}.tar.gz"
  tmp="$(mktemp -d)"
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  backup_path="${BACKUP_DIR}/pre-upgrade-${timestamp}"

  url="https://github.com/${RELEASE_OWNER}/${RELEASE_REPO}/releases/download/${release_tag}/${archive_name}"
  checksum_url="https://github.com/${RELEASE_OWNER}/${RELEASE_REPO}/releases/download/${release_tag}/checksums.txt"

  log "channel: ${CHANNEL}"
  log "release: ${release_tag}"
  log "download: ${url}"
  run mkdir -p "$backup_path"
  if [[ "$SKIP_BACKUP" == "false" ]]; then
    if [[ -x "$BIN_PATH" ]]; then
      run cp "$BIN_PATH" "${backup_path}/omo"
    fi
    if [[ -x "$OMOCTL_BIN_PATH" ]]; then
      run cp "$OMOCTL_BIN_PATH" "${backup_path}/omoctl"
    fi
    log "binary backup: ${backup_path}"
  else
    log "binary backup skipped by operator request"
  fi

  run curl -fsSL "$url" -o "$tmp/$archive_name"
  run curl -fsSL "$checksum_url" -o "$tmp/checksums.txt"

  if [[ "$DRY_RUN" != "true" ]]; then
    (cd "$tmp" && grep " ${archive_name}\$" checksums.txt | sha256sum -c -)
    tar -xzf "$tmp/$archive_name" -C "$tmp"
    [[ -f "$tmp/omo" ]] || fail "omo binary not found in release archive"
    install -m 0755 "$tmp/omo" "$BIN_PATH"
    if [[ -f "$tmp/omoctl" ]]; then
      install -m 0755 "$tmp/omoctl" "$OMOCTL_BIN_PATH"
    fi
  fi

  if ! restart_service || ! wait_for_health; then
    if [[ "$SKIP_BACKUP" == "false" ]]; then
      restore_from_backup "$backup_path"
    fi
    fail "upgrade validation failed; previous binary was restored when available"
  fi

  rm -rf "$tmp"
  log "upgrade complete"
}

main "$@"
