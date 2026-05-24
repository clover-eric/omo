#!/usr/bin/env bash
set -euo pipefail

APP_NAME="omo"
CHANNEL="stable"
DRY_RUN="false"
VERSION="latest"

INSTALL_DIR="/opt/omo"
CONFIG_DIR="/etc/omo"
DATA_DIR="/var/lib/omo"
LOG_DIR="/var/log/omo"
BACKUP_DIR="/var/backups/omo"
BIN_PATH="/usr/local/bin/omo"
SING_BOX_BIN_PATH="/usr/local/bin/sing-box"
INIT_ENV_PATH="/etc/omo/init.env"
INIT_LINK_PATH="/etc/omo/init-link.txt"
UNIT_PATH="/etc/systemd/system/omo.service"
INIT_UNIT_PATH="/etc/systemd/system/omo-init.service"
INIT_WATCH_UNIT_PATH="/etc/systemd/system/omo-init-watch.service"
CADDY_ROOT_CONFIG="/etc/caddy/Caddyfile"
CADDY_CONFIG_DIR="/etc/caddy/omo"
CADDY_CONFIG_PATH="${CADDY_CONFIG_DIR}/omo.caddy"
READY_MARKER="${DATA_DIR}/.bootstrap-ready"
INIT_PORT=""
INIT_TOKEN=""
SERVER_IP=""

usage() {
  cat <<'USAGE'
OMO installer

Usage:
  install.sh [--channel stable|beta|nightly] [--version VERSION] [--dry-run]

This installer prepares OMO 边界运维管理平台 for authorized server operations.
USAGE
}

log() {
  printf '[omo] %s\n' "$*"
}

fail() {
  printf '[omo] ERROR: %s\n' "$*" >&2
  exit 1
}

run() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '[omo] dry-run:'
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

detect_os() {
  if [[ ! -r /etc/os-release ]]; then
    fail "/etc/os-release is required"
  fi
  . /etc/os-release
  case "${ID:-}" in
    ubuntu)
      version_at_least "${VERSION_ID:-0}" "20.04" || fail "Ubuntu 20.04+ is required"
      ;;
    debian)
      version_at_least "${VERSION_ID:-0}" "11" || fail "Debian 11+ is required"
      ;;
    almalinux)
      version_at_least "${VERSION_ID:-0}" "8" || fail "AlmaLinux 8+ is required"
      ;;
    *)
      fail "unsupported distribution: ${ID:-unknown}"
      ;;
  esac
  printf '%s %s' "${PRETTY_NAME:-$ID}" "${VERSION_ID:-}"
}

version_at_least() {
  local actual="$1"
  local required="$2"
  [[ "$(printf '%s\n%s\n' "$required" "$actual" | sort -V | head -n1)" == "$required" ]]
}

check_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command missing: $1"
}

install_sqlite() {
  if command -v sqlite3 >/dev/null 2>&1; then
    log "sqlite: found $(sqlite3 -version | awk '{print $1}')"
    return
  fi

  . /etc/os-release
  log "sqlite: not found; preparing installation"
  case "${ID:-}" in
    ubuntu|debian)
      check_command apt-get
      run apt-get update
      run apt-get install -y sqlite3
      ;;
    almalinux)
      check_command dnf
      run dnf install -y sqlite
      ;;
    *)
      fail "unsupported distribution for automatic sqlite installation: ${ID:-unknown}"
      ;;
  esac
}

check_port_free() {
  local port="$1"
  if command -v ss >/dev/null 2>&1 && ss -ltn "( sport = :$port )" | grep -q ":$port"; then
    fail "port $port is already in use"
  fi
}

check_resources() {
  local mem_kb disk_kb
  mem_kb="$(awk '/MemTotal/ {print $2}' /proc/meminfo 2>/dev/null || printf 0)"
  disk_kb="$(df -Pk / | awk 'NR==2 {print $4}' 2>/dev/null || printf 0)"
  if [[ "$mem_kb" -lt 262144 ]]; then
    fail "at least 256 MiB memory is required"
  fi
  if [[ "$disk_kb" -lt 524288 ]]; then
    fail "at least 512 MiB free disk space is required"
  fi
  log "resources: memory and disk checks passed"
}

detect_server_ip() {
  local ip
  ip="$(curl -fsSL --max-time 3 https://api.ipify.org 2>/dev/null || true)"
  if [[ -z "$ip" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  if [[ -z "$ip" ]]; then
    ip="SERVER_IP"
  fi
  printf '%s' "$ip"
}

normalize_unit_value() {
  printf '%s' "$1" | sed 's/%/%%/g'
}

generate_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr '+/' '-_' | tr -d '='
  else
    tr -dc 'A-Za-z0-9_-' </dev/urandom | head -c 43
  fi
}

resolve_sing_box_version() {
  local version tag
  version="${SING_BOX_VERSION:-latest}"
  if [[ "$version" != "latest" ]]; then
    printf '%s' "${version#v}"
    return
  fi

  tag="$(curl -fsSL --max-time 5 https://api.github.com/repos/SagerNet/sing-box/releases/latest 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' | head -n1 || true)"
  [[ -n "$tag" ]] || fail "could not resolve latest sing-box version"
  printf '%s' "$tag"
}

choose_init_port() {
  local port
  for _ in $(seq 1 50); do
    port="$((20000 + RANDOM % 30000))"
    if port_free "$port"; then
      printf '%s' "$port"
      return
    fi
  done
  fail "could not find a free temporary initialization port"
}

port_free() {
  local port="$1"
  if command -v ss >/dev/null 2>&1 && ss -ltn "( sport = :$port )" | grep -q ":$port"; then
    return 1
  fi
  return 0
}

write_unit() {
  local unit
  unit="$(mktemp)"
  cat >"$unit" <<UNIT
[Unit]
Description=OMO Boundary Operations Platform
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=omo
Group=omo
ExecStart=${BIN_PATH} serve --addr 127.0.0.1:8080 --data ${DATA_DIR}/omo.db --caddy-config ${CADDY_CONFIG_PATH} --sing-box ${SING_BOX_BIN_PATH}
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${CONFIG_DIR} ${LOG_DIR} ${BACKUP_DIR} ${CADDY_CONFIG_DIR}

[Install]
WantedBy=multi-user.target
UNIT
  run install -m 0644 "$unit" "$UNIT_PATH"
  rm -f "$unit"
}

write_init_unit() {
  local unit
  unit="$(mktemp)"
  cat >"$unit" <<UNIT
[Unit]
Description=OMO Temporary Initialization Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=omo
Group=omo
EnvironmentFile=${INIT_ENV_PATH}
ExecStart=${BIN_PATH} serve --addr 0.0.0.0:${INIT_PORT} --data ${DATA_DIR}/omo.db --caddy-config ${CADDY_CONFIG_PATH} --caddy-upstream 127.0.0.1:8080 --expected-ip ${SERVER_IP} --sing-box ${SING_BOX_BIN_PATH}
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${CONFIG_DIR} ${LOG_DIR} ${BACKUP_DIR} ${CADDY_CONFIG_DIR}

[Install]
WantedBy=multi-user.target
UNIT
  run install -m 0644 "$unit" "$INIT_UNIT_PATH"
  rm -f "$unit"
}

write_init_env() {
  local env_file unit_token unit_host
  unit_token="$(normalize_unit_value "$INIT_TOKEN")"
  unit_host="$(normalize_unit_value "$SERVER_IP")"
  env_file="$(mktemp)"
  cat >"$env_file" <<ENV
OMO_INIT_TOKEN=${unit_token}
OMO_INIT_URL_HOST=${unit_host}
OMO_BOOTSTRAP_READY_MARKER=${READY_MARKER}
ENV
  run install -m 0600 "$env_file" "$INIT_ENV_PATH"
  rm -f "$env_file"
}

write_init_link() {
  local link_file link
  link_file="$(mktemp)"
  link="http://${SERVER_IP}:${INIT_PORT}/init?token=${INIT_TOKEN}"
  cat >"$link_file" <<LINK
OMO temporary initialization link

${link}

This one-time link is valid for the first initialization window. After initialization succeeds, OMO removes this file and closes the temporary HTTP entry.

If the page is not reachable, confirm that this server and any cloud security group allow TCP ${INIT_PORT} for temporary initialization, and TCP 80/443 for the HTTPS panel entry.
LINK
  run install -m 0600 "$link_file" "$INIT_LINK_PATH"
  rm -f "$link_file"
}

write_init_watch_unit() {
  local unit
  unit="$(mktemp)"
  cat >"$unit" <<UNIT
[Unit]
Description=OMO Initialization Completion Watcher
After=omo-init.service
Wants=omo-init.service

[Service]
Type=simple
ExecStart=/bin/sh -c 'while [ ! -f "${READY_MARKER}" ]; do sleep 5; done; systemctl enable --now omo.service || exit 1; for i in $(seq 1 20); do curl -fsS --max-time 2 http://127.0.0.1:8080/api/system/health >/dev/null 2>&1 && break; sleep 1; done; curl -fsS --max-time 2 http://127.0.0.1:8080/api/system/health >/dev/null || exit 1; rm -f "${INIT_ENV_PATH}" "${INIT_LINK_PATH}"; systemctl disable --now omo-init.service; systemctl disable omo-init-watch.service'
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
UNIT
  run install -m 0644 "$unit" "$INIT_WATCH_UNIT_PATH"
  rm -f "$unit"
}

install_caddy() {
  if command -v caddy >/dev/null 2>&1; then
    log "caddy: found $(caddy version 2>/dev/null | head -n1)"
    return
  fi

  . /etc/os-release
  log "caddy: not found; preparing installation"
  case "${ID:-}" in
    ubuntu|debian)
      check_command apt-get
      run apt-get update
      run apt-get install -y debian-keyring debian-archive-keyring apt-transport-https gnupg
      run install -d -m 0755 /etc/apt/keyrings
      run sh -c "curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor -o /etc/apt/keyrings/caddy-stable-archive-keyring.gpg"
      run sh -c "curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt > /etc/apt/sources.list.d/caddy-stable.list"
      run apt-get update
      run apt-get install -y caddy
      ;;
    almalinux)
      check_command dnf
      run dnf install -y 'dnf-command(copr)'
      run dnf copr enable -y @caddy/caddy
      run dnf install -y caddy
      ;;
    *)
      fail "unsupported distribution for automatic Caddy installation: ${ID:-unknown}"
      ;;
  esac
}

install_sing_box() {
  if command -v sing-box >/dev/null 2>&1; then
    log "sing-box: found $(sing-box version 2>/dev/null | head -n1)"
    return
  fi

  local arch tmp version url
  arch="$(detect_arch)"
  version="$(resolve_sing_box_version)"
  tmp="$(mktemp -d)"
  log "sing-box: not found; preparing installation"
  url="https://github.com/SagerNet/sing-box/releases/download/v${version}/sing-box-${version}-linux-${arch}.tar.gz"
  log "sing-box download: ${url}"
  run curl -fsSL "$url" -o "$tmp/sing-box.tar.gz"
  if [[ "$DRY_RUN" != "true" ]]; then
    tar -xzf "$tmp/sing-box.tar.gz" -C "$tmp"
    local extracted
    extracted="$(find "$tmp" -type f -name sing-box -perm -111 | head -n1)"
    [[ -n "$extracted" ]] || fail "sing-box binary not found in downloaded archive"
    install -m 0755 "$extracted" "$SING_BOX_BIN_PATH"
  fi
  rm -rf "$tmp"
}

prepare_caddy_paths() {
  local root_config managed_config
  root_config="$(mktemp)"
  managed_config="$(mktemp)"
  cat >"$root_config" <<CADDY
{
	# Keep Caddy's local admin API enabled so OMO can apply verified entry updates.
}

import ${CADDY_CONFIG_DIR}/*.caddy
CADDY
  cat >"$managed_config" <<CADDY
# OMO writes managed HTTPS entry configuration here after domain verification.
CADDY

  run mkdir -p "$CADDY_CONFIG_DIR"
  run chown omo:omo "$CADDY_CONFIG_DIR"
  run chmod 0755 "$CADDY_CONFIG_DIR"
  if [[ -f "$CADDY_ROOT_CONFIG" && ! -f "${CADDY_ROOT_CONFIG}.omo.bak" ]]; then
    run cp "$CADDY_ROOT_CONFIG" "${CADDY_ROOT_CONFIG}.omo.bak"
  fi
  run install -m 0644 "$root_config" "$CADDY_ROOT_CONFIG"
  if [[ ! -f "$CADDY_CONFIG_PATH" ]]; then
    run install -o omo -g omo -m 0644 "$managed_config" "$CADDY_CONFIG_PATH"
  else
    run chown omo:omo "$CADDY_CONFIG_PATH"
    run chmod 0644 "$CADDY_CONFIG_PATH"
  fi
  if command -v systemctl >/dev/null 2>&1; then
    run systemctl enable caddy
    run systemctl reload-or-restart caddy
  fi
  rm -f "$root_config" "$managed_config"
}

check_time_sync() {
  if ! command -v timedatectl >/dev/null 2>&1; then
    log "time sync: timedatectl not found; verify server time before HTTPS certificate provisioning"
    return
  fi

  local synchronized
  synchronized="$(timedatectl show -p NTPSynchronized --value 2>/dev/null || true)"
  if [[ "$synchronized" == "yes" ]]; then
    log "time sync: system clock is synchronized"
    return
  fi

  log "time sync: not confirmed; enabling system NTP if available"
  if ! run timedatectl set-ntp true; then
    log "time sync: could not enable NTP automatically; verify server time before continuing"
  fi
}

log_firewall_guidance() {
  local active="false"
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qi '^Status: active'; then
    log "firewall: ufw is active; allow TCP ${INIT_PORT} temporarily and TCP 80/443 for the HTTPS panel entry"
    active="true"
  fi
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    log "firewall: firewalld is active; allow TCP ${INIT_PORT} temporarily and TCP 80/443 for the HTTPS panel entry"
    active="true"
  fi
  if [[ "$active" == "false" ]]; then
    log "firewall: no active ufw/firewalld detected; also check cloud security group rules for TCP ${INIT_PORT}, 80, and 443"
  fi
}

wait_for_init_health() {
  if [[ "$DRY_RUN" == "true" ]]; then
    log "dry-run: would verify http://127.0.0.1:${INIT_PORT}/api/system/health"
    return
  fi

  local url
  url="http://127.0.0.1:${INIT_PORT}/api/system/health"
  for _ in $(seq 1 20); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      log "temporary initialization service health: ok"
      return
    fi
    sleep 1
  done

  systemctl status omo-init --no-pager || true
  journalctl -u omo-init -n 80 --no-pager || true
  fail "temporary initialization service did not become healthy; inspect the omo-init service logs"
}

install_binary() {
  if [[ -x "./dist/omo" ]]; then
    log "using local dist/omo binary"
    run install -m 0755 ./dist/omo "$BIN_PATH"
    return
  fi

  local arch url tmp checksum_url
  arch="$(detect_arch)"
  tmp="$(mktemp -d)"
  url="https://github.com/omo/omo/releases/download/${VERSION}/omo_${VERSION}_linux_${arch}.tar.gz"
  checksum_url="https://github.com/omo/omo/releases/download/${VERSION}/checksums.txt"
  log "download: $url"
  run curl -fsSL "$url" -o "$tmp/omo.tar.gz"
  run curl -fsSL "$checksum_url" -o "$tmp/checksums.txt"
  if [[ "$DRY_RUN" != "true" ]]; then
    (cd "$tmp" && grep "omo_${VERSION}_linux_${arch}.tar.gz" checksums.txt | sha256sum -c -)
    tar -xzf "$tmp/omo.tar.gz" -C "$tmp"
    install -m 0755 "$tmp/omo" "$BIN_PATH"
  fi
  rm -rf "$tmp"
}

prepare_paths() {
  if ! id -u omo >/dev/null 2>&1; then
    run useradd --system --home "$DATA_DIR" --shell /usr/sbin/nologin omo
  fi
  run mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$BACKUP_DIR"
  run chown -R omo:omo "$DATA_DIR" "$LOG_DIR" "$BACKUP_DIR"
  run chmod 0750 "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR" "$BACKUP_DIR"
  run rm -f "$READY_MARKER"
  run rm -f "$INIT_ENV_PATH"
  run rm -f "$INIT_LINK_PATH"
}

main() {
  local arch os_name
  arch="$(detect_arch)"
  os_name="$(detect_os)"

  log "channel: ${CHANNEL}"
  log "version: ${VERSION}"
  log "architecture: ${arch}"
  log "system: ${os_name}"

  check_command uname
  check_command curl
  check_command tar
  check_command install
  check_command awk
  check_command sed
  if [[ "$DRY_RUN" != "true" && "${EUID:-$(id -u)}" -ne 0 ]]; then
    fail "please run as root"
  fi
  check_resources
  install_sqlite

  if ! command -v systemctl >/dev/null 2>&1; then
    if [[ "$DRY_RUN" == "true" ]]; then
      log "systemd: not found"
    else
      fail "systemd is required"
    fi
  else
    log "systemd: found"
  fi
  check_time_sync

  if [[ "$DRY_RUN" == "true" ]]; then
    if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
      log "root: not running as root; install would require root"
    else
      log "root: ok"
    fi
    prepare_paths
    install_caddy
    prepare_caddy_paths
    install_sing_box
    check_port_free 80
    check_port_free 443
    install_binary
    INIT_PORT="$(choose_init_port)"
    INIT_TOKEN="$(generate_token)"
    SERVER_IP="$(detect_server_ip)"
    write_unit
    write_init_env
    write_init_link
    write_init_unit
    write_init_watch_unit
    log_firewall_guidance
    log "temporary initialization link:"
    log "http://${SERVER_IP}:${INIT_PORT}/init?token=${INIT_TOKEN}"
    log "recovery file: ${INIT_LINK_PATH}"
    log "after initialization succeeds, omo-init-watch.service will start omo.service and stop the temporary initialization service"
    log "dry-run complete; no files were changed"
    exit 0
  fi

  prepare_paths
  install_caddy
  prepare_caddy_paths
  install_sing_box
  check_port_free 80
  check_port_free 443
  install_binary
  INIT_PORT="$(choose_init_port)"
  INIT_TOKEN="$(generate_token)"
  SERVER_IP="$(detect_server_ip)"
  write_unit
  write_init_env
  write_init_link
  write_init_unit
  write_init_watch_unit
  run systemctl daemon-reload
  run systemctl enable omo-init
  run systemctl enable --now omo-init-watch
  run systemctl start omo-init
  wait_for_init_health
  log_firewall_guidance

  log "OMO temporary initialization service started"
  log "temporary initialization link:"
  log "http://${SERVER_IP}:${INIT_PORT}/init?token=${INIT_TOKEN}"
  log "recovery file: ${INIT_LINK_PATH}"
  log "after initialization succeeds, OMO will switch to the HTTPS panel service and close the temporary initialization entry"
}

main "$@"
