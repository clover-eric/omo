# OMO Security

OMO is only for user-owned servers, user-owned networks, enterprise remote operations, and explicitly authorized environments.

## Baseline Requirements

- One-time initialization token.
- Strong password validation.
- Argon2id password hashing.
- HttpOnly, Secure, SameSite session cookies.
- CSRF protection for browser APIs.
- Login rate limiting and temporary lockout.
- Admin authorization for management APIs.
- Audited high-risk operations.
- Rotatable and disableable subscription tokens.
- Sensitive field redaction.
- Configuration files with restrictive permissions.
- Checksum or signature verification for updates.
- Backup encryption and explicit restore confirmation.

## Current Implementation State

Implemented baseline controls now include one-time initialization tokens, strong administrator password validation, Argon2id password hashing, HttpOnly SameSite cookies, browser CSRF protection for `/api/*` state-changing requests, SQLite-persisted login failure counters and temporary lockouts, restrictive installer permissions for initialization recovery files, explicit degraded state when the HTTPS entry cannot be completed, encrypted backup archives, explicit confirmation for backup restore/update apply/update rollback, update checksum and signature verification enforcement, durable audit records for high-risk operations, and local release checksum/signature/SBOM configuration.

## Security Scan Automation

`scripts/security-scan.sh` is the local hardening entry point and is exposed through `make security-scan`.

Required checks:

- Product-boundary wording scan over source, docs, deployment files, and release configuration.
- Damaged-text scan over source, docs, OpenAPI, release configuration, and Makefile.
- `go test ./...` and `go vet ./...`.
- `pnpm --dir web test` and `pnpm --dir web build`.

Optional supply-chain checks run when tools are installed:

- `goreleaser check`.
- `govulncheck ./...`.
- `gosec ./...`.
- `syft packages dir:. --scope all-layers -o spdx-json`.
- `cosign version`.

The scan returns a non-zero exit code when required checks fail. Optional tools that are not installed are reported as skipped so local runs remain honest about coverage.

Remaining hardening items include management API authorization checks as new modules are added and target-server validation of system service operations.
