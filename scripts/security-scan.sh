#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

failed=0
skipped=0

log() {
  printf '[security-scan] %s\n' "$*"
}

run_required() {
  log "required: $*"
  if "$@"; then
    log "ok: $*"
  else
    log "failed: $*"
    failed=1
  fi
}

run_optional() {
  local tool="$1"
  shift
  if ! command -v "$tool" >/dev/null 2>&1; then
    log "skip: $tool is not installed"
    skipped=$((skipped + 1))
    return
  fi
  log "optional: $tool $*"
  if "$tool" "$@"; then
    log "ok: $tool $*"
  else
    log "failed: $tool $*"
    failed=1
  fi
}

run_forbidden_scan() {
  local label="$1"
  shift
  log "required: ${label}"
  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e
  if [[ "$status" -eq 1 ]]; then
    log "ok: ${label}"
    return
  fi
  if [[ "$status" -eq 0 ]]; then
    printf '%s\n' "$output"
    log "failed: ${label}"
    failed=1
    return
  fi
  printf '%s\n' "$output"
  log "failed: ${label} command error"
  failed=1
}

run_forbidden_scan "product-boundary wording" rg -n "bypass|stealth|attack|evasion|bulk scanning|circumvention" internal openapi web/src scripts deploy docs .goreleaser.yaml -S --glob '!docs/PROJECT_SPEC.md' --glob '!docs/STATUS.md' --glob '!internal/api/router_test.go' --glob '!scripts/security-scan.sh'
run_forbidden_scan "damaged-text" rg -n "\\x{9427}|\\x{934B}\\x{52DD}|\\x{9366}|\\x{FFFD}|\\?/span|\\?/h1|\\?/p|\\?;" web/src internal openapi docs .goreleaser.yaml Makefile -S --glob '!docs/STATUS.md'

if command -v go >/dev/null 2>&1; then
  run_required go test ./...
  run_required go vet ./...
else
  log "failed: go is not installed"
  failed=1
fi

if command -v pnpm >/dev/null 2>&1; then
  run_required pnpm --dir web test
  run_required pnpm --dir web build
else
  log "failed: pnpm is not installed"
  failed=1
fi

run_optional goreleaser check
run_optional govulncheck ./...
run_optional gosec ./...
run_optional syft packages dir:. --scope all-layers -o spdx-json
run_optional cosign version

if [[ "$failed" -ne 0 ]]; then
  log "security scan failed; skipped optional tools: ${skipped}"
  exit 1
fi

log "security scan completed; skipped optional tools: ${skipped}"
