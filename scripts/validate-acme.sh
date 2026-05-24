#!/usr/bin/env bash
set -euo pipefail

DOMAIN=""
EXPECTED_IP=""
LOCAL_HEALTH_URL="http://127.0.0.1:8080/api/system/health"
HEALTH_PATH="/api/system/health"
PANEL_PATH="/dashboard"
SKIP_SERVICE_CHECKS=0

usage() {
  cat <<'EOF'
Usage: scripts/validate-acme.sh --domain DOMAIN [options]

Validate the Phase 2 HTTPS entry on an authorized OMO target server.

Options:
  --domain DOMAIN              Public panel domain to validate. Required.
  --expected-ip IP             Expected server IP in DNS records.
  --local-health-url URL       Local OMO health URL. Default: http://127.0.0.1:8080/api/system/health
  --health-path PATH           Public HTTPS health path. Default: /api/system/health
  --panel-path PATH            Public HTTPS panel path. Default: /dashboard
  --skip-service-checks        Skip systemd service status checks.
  -h, --help                   Show this help.

The script is read-only. It checks DNS, local service health, public HTTP and
HTTPS entry responses, TLS certificate metadata, and optional systemd status.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --domain)
      DOMAIN="${2:-}"
      shift 2
      ;;
    --expected-ip)
      EXPECTED_IP="${2:-}"
      shift 2
      ;;
    --local-health-url)
      LOCAL_HEALTH_URL="${2:-}"
      shift 2
      ;;
    --health-path)
      HEALTH_PATH="${2:-}"
      shift 2
      ;;
    --panel-path)
      PANEL_PATH="${2:-}"
      shift 2
      ;;
    --skip-service-checks)
      SKIP_SERVICE_CHECKS=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$DOMAIN" ]]; then
  printf 'missing required --domain\n' >&2
  usage >&2
  exit 2
fi

failures=0
warnings=0

log() {
  printf '[validate-acme] %s\n' "$*"
}

ok() {
  log "ok: $*"
}

warn() {
  warnings=$((warnings + 1))
  log "warn: $*"
}

fail() {
  failures=$((failures + 1))
  log "failed: $*"
}

require_command() {
  if command -v "$1" >/dev/null 2>&1; then
    ok "$1 is available"
  else
    fail "$1 is not installed"
  fi
}

path_url() {
  local scheme="$1"
  local path="$2"
  case "$path" in
    /*) printf '%s://%s%s' "$scheme" "$DOMAIN" "$path" ;;
    *) printf '%s://%s/%s' "$scheme" "$DOMAIN" "$path" ;;
  esac
}

resolve_domain() {
  if command -v getent >/dev/null 2>&1; then
    getent ahosts "$DOMAIN" | awk '{print $1}' | sort -u
    return 0
  fi
  if command -v dig >/dev/null 2>&1; then
    { dig +short A "$DOMAIN"; dig +short AAAA "$DOMAIN"; } | sort -u
    return 0
  fi
  if command -v nslookup >/dev/null 2>&1; then
    nslookup "$DOMAIN" | awk '/^Address: / {print $2}' | sort -u
    return 0
  fi
  return 1
}

check_http_status() {
  local label="$1"
  local url="$2"
  local min_status="$3"
  local max_status="$4"
  local status
  set +e
  status="$(curl -fsS -o /dev/null -w '%{http_code}' --max-time 15 "$url" 2>/dev/null)"
  local rc=$?
  set -e
  if [[ "$rc" -ne 0 || -z "$status" ]]; then
    fail "$label did not return an HTTP response from $url"
    return
  fi
  if [[ "$status" -ge "$min_status" && "$status" -le "$max_status" ]]; then
    ok "$label returned HTTP $status"
  else
    fail "$label returned HTTP $status from $url"
  fi
}

require_command curl
require_command openssl

if [[ "$failures" -ne 0 ]]; then
  log "required command checks failed"
  exit 1
fi

log "validating domain: $DOMAIN"

if records="$(resolve_domain)"; then
  if [[ -n "$records" ]]; then
    ok "DNS records resolved: $(printf '%s' "$records" | paste -sd ',' -)"
    if [[ -n "$EXPECTED_IP" ]]; then
      if printf '%s\n' "$records" | grep -Fxq "$EXPECTED_IP"; then
        ok "DNS contains expected server IP $EXPECTED_IP"
      else
        fail "DNS records do not contain expected server IP $EXPECTED_IP"
      fi
    else
      warn "no --expected-ip provided; DNS was resolved but not compared to a target IP"
    fi
  else
    fail "domain did not resolve"
  fi
else
  fail "no DNS resolver command is available; install getent, dig, or nslookup"
fi

if [[ "$SKIP_SERVICE_CHECKS" -eq 0 ]]; then
  if command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet omo; then
      ok "omo service is active"
    else
      fail "omo service is not active"
    fi
    if systemctl is-active --quiet caddy; then
      ok "caddy service is active"
    else
      fail "caddy service is not active"
    fi
  else
    warn "systemctl is not available; service status checks skipped"
  fi
fi

check_http_status "local OMO health endpoint" "$LOCAL_HEALTH_URL" 200 299
check_http_status "public HTTP entry" "$(path_url http "/")" 200 399
check_http_status "public HTTPS health endpoint" "$(path_url https "$HEALTH_PATH")" 200 299
check_http_status "public HTTPS panel entry" "$(path_url https "$PANEL_PATH")" 200 399

cert_text=""
set +e
cert_text="$(printf '' | openssl s_client -servername "$DOMAIN" -connect "$DOMAIN:443" -showcerts 2>/dev/null | openssl x509 -noout -subject -issuer -dates -ext subjectAltName 2>/dev/null)"
cert_rc=$?
set -e

if [[ "$cert_rc" -ne 0 || -z "$cert_text" ]]; then
  fail "TLS certificate metadata could not be read from $DOMAIN:443"
else
  ok "TLS certificate metadata is readable"
  if printf '%s\n' "$cert_text" | grep -Fq "DNS:$DOMAIN"; then
    ok "TLS certificate SAN includes $DOMAIN"
  else
    fail "TLS certificate SAN does not include $DOMAIN"
  fi
  if printf '%s\n' "$cert_text" | grep -Eq '^issuer='; then
    ok "TLS certificate issuer is present"
  else
    fail "TLS certificate issuer is missing"
  fi
  if printf '%s\n' "$cert_text" | grep -Eq '^notBefore=' && printf '%s\n' "$cert_text" | grep -Eq '^notAfter='; then
    ok "TLS certificate validity dates are present"
  else
    fail "TLS certificate validity dates are missing"
  fi
fi

if [[ "$failures" -ne 0 ]]; then
  log "validation failed with ${failures} failed checks and ${warnings} warnings"
  exit 1
fi

log "validation passed with ${warnings} warnings"
