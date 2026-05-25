# OMO Status

## Current Phase

Phase 7: backup/restore, audit listing, online update, release automation, and security scan integration are locally implemented. Phase 7 now implements backend backup creation, backup listing, encrypted archive storage, checksum verification, operator-confirmed restore, managed Caddy/sing-box config capture and restore, panel certificate metadata capture without private key material, durable jobs, audit entries, concrete OpenAPI schemas for `/api/backups`, a concrete `/api/audit` listing API, online update check/apply/rollback APIs with pre-update backup, checksum and signature verification, systemd restart, health check, and automatic binary restore on failed apply, `/logs` audit review UI, `/settings` backup/restore and update operation UI, GoReleaser release configuration with checksums, cosign checksum signing, SBOM generation, build-time version metadata injection, and `make security-scan` local hardening automation. Phase 6 cascade nodes is locally complete. Phase 5 server checkup is locally complete. Phase 4 smart subscriptions and QR import is locally implemented, including active managed service metadata in backend-generated subscription descriptors, token update/disable/delete APIs, and a console workflow for selecting, rotating, revealing, disabling, and deleting distribution entries without redisplaying historical plaintext tokens. Phase 3 implementation is locally complete at source level: the MVP service profile set now covers standard, high throughput, broad compatibility, lightweight fallback, and mobile optimized access; OpenAPI-declared system overview and service instance list/create/update APIs are wired to backend persistence; apply/rollback jobs synchronize service instance state; tests were added; and the service library UI now presents guided access plans, next actions, instance state, and expert details without turning ordinary use into a raw protocol picker. Phase 2 is locally complete except for real target-server ACME validation; a read-only target-server validation script now exists to make that final external acceptance check repeatable.

## Completed

- Initial project requirements read from `MASTER_DEVELOPMENT_PROMPT.md`.
- Required directory structure started.
- Documentation baseline created.
- OpenAPI 3.1 baseline created in `openapi/openapi.yaml`.
- SQLite initial migration baseline created with required MVP tables.
- Go backend shell created with `/api/system/health` and `/api/bootstrap/status`.
- SvelteKit static frontend shell created and embedded into the Go server build path.
- Makefile, installer dry-run skeleton, systemd unit, and Caddy example created.
- Phase 0 verification commands passed.
- SQLite store and embedded migration runner implemented.
- Phase 1 `job_events` migration added.
- One-time initialization token generated at server startup when no administrator exists.
- `POST /api/bootstrap/start` implemented with token validation, strong password validation, Argon2id admin password hashing, session cookie creation, persistent job updates, and token invalidation.
- `GET /api/bootstrap/events` implemented with SSE over persisted bootstrap job events.
- `/init` frontend page implemented with real API submission and SSE progress display.
- OpenAPI updated for bootstrap start/status schemas.
- Administrator login/logout/session check implemented with HttpOnly cookie sessions.
- Login failure lockout implemented with SQLite-persisted counters and temporary lockout state.
- `/login` page added.
- `/dashboard` route added as the post-initialization panel entry.
- Bootstrap retry confirmation flag added for failed jobs.
- `scripts/install.sh` expanded to perform OS/arch/resource/command/systemd/port checks, prepare users/directories, install binary, write systemd unit, and support realistic dry-run output.
- `internal/caddy` implemented domain DNS checks, port checks, Caddyfile rendering, validate/reload, rollback on reload failure, and TLS certificate status reading.
- Bootstrap state machine extended through `DOMAIN_VERIFY`, `TLS_PROVISION`, and `PANEL_HTTPS_ENABLE`.
- Phase 2 failures keep the initialization token valid and preserve failed job state for retry.
- Domain resolution failures now return `DOMAIN_NOT_RESOLVED` with user-readable Chinese guidance.
- OpenAPI and `/init` progress labels updated for Phase 2.
- `scripts/install.sh` now prepares Caddy installation paths and installs Caddy for Ubuntu/Debian/AlmaLinux when missing, with dry-run support.
- Bootstrap domain checks can compare DNS results with detected public server IPs, or an explicit `--expected-ip`.
- Panel access middleware redirects initialized main panel routes from HTTP/IP access to the configured HTTPS domain.
- Panel access middleware recognizes trusted loopback reverse-proxy requests with `X-Forwarded-Proto: https`, preventing HTTPS redirect loops behind Caddy while ignoring untrusted forwarded headers from remote clients.
- `omoctl dev-seed-ready` added for local verification of initialized panel access behavior.
- New-server initialization optimized: installer now generates a random temporary initialization port, a one-time token, and a direct `http://SERVER_IP:RANDOM_PORT/init?token=...` link.
- Installer writes `omo-init.service` for the temporary HTTP initialization entry and `omo-init-watch.service` to switch to the regular loopback-only `omo.service` after the backend writes the bootstrap ready marker and the regular service health check passes.
- Installer writes a root-only `/etc/omo/init-link.txt` recovery file for the temporary initialization link and removes it after successful handoff.
- Installer reruns stop existing OMO regular, temporary initialization, and watcher services before writing fresh units, tokens, and recovery links so failed target-server bootstrap attempts can be retried cleanly.
- Installer checks system time synchronization, verifies the temporary initialization service local health endpoint, and prints firewall/security-group guidance for the temporary port plus 80/443.
- Installer prepares Caddy with an OMO-managed import directory so the default public entry is empty until domain verification applies the HTTPS panel entry.
- Bootstrap success now returns `https://{domain}/dashboard` and the `/init` page redirects there after the final entry configuration step.
- Bootstrap Phase 2 now waits for a real TLS certificate handshake before marking HTTPS ready, writing the handoff marker, or redirecting to the dashboard.
- Installer recovery tokens can be refreshed even after an administrator exists, allowing a failed or premature HTTPS handoff to be recovered through a new temporary initialization link.
- Caddy unavailable handling now records an explicit degraded `temporary_http` Phase 2 result, returns `CADDY_UNAVAILABLE`, keeps the initialization token valid, and leaves the temporary entry retryable instead of marking HTTPS ready.
- Browser CSRF protection is implemented for non-safe `/api/*` methods with a readable `omo_csrf` cookie, `X-CSRF-Token` validation, and `GET /api/security/csrf` preparation for first POST flows.
- Login failure counters and temporary administrator lockouts are persisted in SQLite via `login_rate_limits`, so active lockouts survive service restarts and successful login clears the record.
- `internal/core/singbox` now detects sing-box by configured path, `PATH`, or standard candidate paths; parses `sing-box version`; and reports installation/health/version through `/api/core/singbox/status`.
- `scripts/install.sh` now prepares sing-box installation from official release assets when missing and passes the binary path to both regular and temporary OMO systemd services.
- `internal/protocol` now defines a backend-owned service profile registry with standard secure access, high throughput access, and broad compatibility access templates, including dependency, compatibility, score weight, golden test, and rollback metadata.
- The backend-owned service profile registry now covers the full MVP set from the project prompt: standard secure access, high throughput access, broad compatibility access, lightweight fallback access, and mobile optimized access.
- `GET /api/services/profiles` now exposes the read-only service profile template catalog through the unified response envelope, with the OpenAPI contract updated before later apply/rollback handlers.
- `GET /api/system/overview` now exposes concrete bootstrap/core/version/count data through the OpenAPI-declared system overview endpoint.
- `GET /api/services`, `POST /api/services`, and `PATCH /api/services/{id}` now implement the OpenAPI-declared managed access service instance catalog using `service_profiles` and `service_instances`.
- Service instance persistence now stores backend-synchronized profile metadata, planned/active/disabled status, managed listen port intent, config version, display name, and timestamps.
- API error responses in `internal/api/router.go` now use ASCII enterprise operations wording to avoid corrupting public response text in the current Windows PowerShell encoding environment.
- `internal/configgen` now renders OMO-managed sing-box JSON from service profile selections, validates temporary output, backs up the active config, atomically applies the new config, and restores the previous config if post-apply validation fails.
- `POST /api/services/{id}/apply` and `POST /api/services/{id}/rollback` now run backend-only configuration operations through durable jobs and return the unified API envelope.
- Apply/rollback jobs now synchronize persisted managed service instances after file-level configuration succeeds and before the durable job is marked successful, keeping job state consistent with the service catalog.
- Dashboard service cards now load backend-owned service profile templates, persisted service instances, and sing-box core status, then call backend create/apply/rollback APIs without generating core configuration in the frontend.
- `internal/subscription` now creates, lists, and rotates smart subscription tokens while storing only token hashes.
- `/s/{token}` now validates active subscription tokens, records request metadata with hashed remote addresses, and returns sing-box JSON, Clash/Mihomo-style text, direct URI text, or an adaptive manual import page.
- Backend-generated sing-box and Clash/Mihomo subscription descriptors now include active managed service metadata for service name, profile, listen port, status, configuration version, and update time.
- `/s/{token}?format=qr` now returns backend-generated SVG QR code output for subscription import.
- `/subscriptions` frontend page added for configuration distribution token creation, rotation, one-time import URL copy, and QR preview.
- Damaged frontend source text in `/init` and `/login` was replaced with clean enterprise operations wording so the Svelte pages are syntactically valid and user-facing text remains within product boundaries.
- OpenAPI diagnostics responses now define concrete server checkup job and report schemas.
- `internal/diagnostics` now runs a durable local server checkup job, records SSE events, and persists the latest diagnostic report.
- `POST /api/diagnostics/run`, `GET /api/diagnostics/latest`, and `GET /api/diagnostics/events` are wired through the API router.
- `/diagnostics` frontend page added for running authorized server checkups, reading local evidence, viewing runtime snapshot data, and following SSE progress.
- Diagnostics now includes configured panel domain DNS, configured panel domain TLS, local 80/443 reachability, and sing-box access-core health providers in addition to runtime, CPU, memory, and loopback checks.
- `/api/settings` now exposes a constrained diagnostics external provider setting, redacts saved credentials, requires HTTPS endpoints, and validates bounded timeouts.
- Diagnostics now supports an optional operator-configured external provider that is disabled by default and runs only after explicit configuration.
- `/diagnostics` now includes provider settings controls for enabling an operator-configured check, saving an optional credential, and clearing the saved credential.
- OpenAPI Phase 6 schemas added for cascade pairing code creation, pairing acceptance, cascade node list/update/delete, and one-hop pair records.
- `internal/pairing` now creates short-lived signed one-time pairing codes while storing only code hashes.
- Pairing acceptance now verifies the signed code envelope, enforces one-time use, creates local entry and remote exit trust records, creates a pending one-hop pair, persists a durable job, and writes audit logs.
- Pairing acceptance now performs a bounded HTTPS exchange with `https://{exit-domain}/api/pairing/exchange` when the entry OMO instance does not own the pairing code hash.
- `/api/pairing/exchange` now verifies and consumes a signed one-time pairing code, records the entry node trust relationship on the exit instance, returns exit metadata to the entry instance, and remains a narrow machine-to-machine endpoint outside browser CSRF.
- `POST /api/cascade/pairs/{id}/plan` now generates a backend-owned one-hop cascade configuration preview and records planned pair state.
- `POST /api/cascade/pairs/{id}/apply` now requires `confirm: true`, writes the backend-generated cascade configuration, creates a durable apply job, records audit history, and marks the pair as applied.
- `POST /api/cascade/health/sample` now runs bounded health sampling for registered cascade nodes, updates online state, latency, response-throughput estimate, and latest error, and records a durable sampling job.
- `GET /api/cascade/nodes`, `PATCH /api/cascade/nodes/{id}`, and `DELETE /api/cascade/nodes/{id}` are wired through the API router.
- `/cascade` frontend page added for pairing code creation, pairing acceptance, trust record review, configuration planning, confirmed apply, health sampling, disabling nodes, and deleting node relationships.
- OpenAPI Phase 7 backup schemas added for backup records, backup list/create results, and restore confirmation requests.
- `internal/backup` now creates ZIP backup archives with a manifest and consistent SQLite database snapshot, records SHA-256 checksums, persists durable backup jobs, and writes audit history.
- `GET /api/backups`, `POST /api/backups`, and `POST /api/backups/{id}/restore` are wired through the API router, with restore requiring `confirm: true`.
- Store support added for backup records, SQLite `VACUUM INTO` snapshots, and operator-confirmed database restore with a pre-restore local copy.
- Unit and router tests were added for backup record persistence, backup create/list, restore confirmation, and restored database state.
- Backup archives now include runtime/version metadata plus configured managed Caddy and sing-box files when present, with per-file SHA-256 metadata and operator-confirmed restore back to the currently configured allowlisted paths.
- Managed file restore now preserves overwritten files as `.pre-restore-*` copies and rolls back restored files when the database restore step fails.
- Backup manifests now include panel certificate metadata from the stored Phase 2 result: configured domain, availability, issuer when known, metadata source, capture time, and an explicit metadata-only note. Certificate private key material and certificate store contents are not included.
- Backup archives are now encrypted at rest with AES-256-GCM. The encrypted archive checksum is stored in `backup_records`, the local encryption key is generated with secure randomness and written as a `0600` key file under the backup directory by default, and restore remains compatible with older unencrypted ZIP archives.
- OpenAPI Phase 7 audit schemas added for audit log entries and audit list results.
- `internal/audit` now lists recent audit logs, parses JSON details into structured response data, and preserves legacy/non-JSON details under a `raw` field.
- `GET /api/audit` is wired through the API router with a bounded `limit` query parameter.
- Store, service, and router tests were added for audit log persistence, latest-first listing, details parsing, and invalid limit handling.
- OpenAPI Phase 7 update-check schema added for read-only update manifest checks.
- `internal/update` now reads an operator-configured HTTPS manifest URL, fetches bounded JSON release metadata, reports version/channel/summary/platform, selects the matching OS/architecture artifact, and returns checksum/signature metadata without downloading or applying an update.
- `GET /api/update/check` is wired through the API router, and `omo serve --update-manifest` can persist the manifest URL.
- Service and router tests were added for unconfigured update checks, HTTPS manifest checks, artifact verification metadata selection, and invalid manifest URL handling.
- OpenAPI Phase 7 update apply/rollback schemas added for explicit confirmation requests and durable update job responses.
- `internal/update` now implements operator-confirmed update apply with pre-update backup, HTTPS artifact download, SHA-256 verification, required signature verification, binary replacement, service restart, health check, audit history, and automatic previous-binary restore on failed apply health validation.
- `internal/update` now implements operator-confirmed rollback to the last preserved update binary with restart and health validation.
- `POST /api/update/apply` and `POST /api/update/rollback` are wired through the API router with confirmation, unavailable-artifact, verification-failed, and rollback-unavailable error mapping.
- `omo serve --update-work-dir` now configures the update workspace, while the default service wiring uses the active executable path, `systemctl restart omo.service`, and the local health endpoint.
- Service tests were added for update apply, checksum/signature verification wiring, pre-update backup result wiring, rollback metadata persistence, explicit confirmation, rollback restore, and automatic previous-binary restoration after health failure.
- `/api/settings` now includes update manifest URL read/write support, with HTTPS validation and an empty-string clear path.
- `/logs` frontend page added for bounded audit log review with newest-first records and structured details.
- `/settings` frontend page added for update manifest configuration, update check/apply/rollback confirmation controls, encrypted backup creation, confirmed restore, and diagnostics provider settings.
- `/services` frontend route now aliases the service library console so the implemented page set matches the product page map while `/dashboard` remains the post-initialization entry.
- Main console navigation now includes service library, audit logs, and settings across the dashboard, distribution, diagnostics, and cascade pages.
- `.goreleaser.yaml` now defines Linux amd64/arm64 release builds for `omo` and `omoctl`, frontend pre-build hooks, archive naming compatible with `scripts/install.sh`, SHA-256 checksums, cosign checksum signing through a `.sigstore.json` bundle, and archive/binary SBOM generation.
- README.md now provides the primary operator entry point with project overview, one-command install, first initialization, upgrade, safe uninstall, health checks, HTTPS validation, backup/update notes, developer commands, and product-boundary guidance.
- `scripts/install.sh` now resolves OMO GitHub Releases from `clover-eric/omo`, supports `latest` release tag lookup, installs `omoctl` when present, and uses checksum verification against the actual release archive name.
- `scripts/upgrade.sh` now provides a one-command upgrade path with current binary backup, release archive checksum verification, service restart, local health validation, and automatic binary restoration on failed validation.
- `scripts/uninstall.sh` now provides conservative uninstall by default with data preservation, plus explicit `--purge --confirm-purge` for intentional removal of local OMO data, logs, backups, and managed Caddy snippets.
- `.gitattributes` now pins shell scripts, Markdown, and YAML files to LF line endings so lifecycle scripts remain Linux-server friendly from Windows workspaces.
- `internal/version` now carries release metadata injected by GoReleaser ldflags, and the main server uses it for health responses, backup manifests, and update checks.
- `omoctl version` now reports the shared release version, and Makefile targets were added for GoReleaser config validation and local snapshot releases.
- `scripts/security-scan.sh` now provides a local hardening gate for product-boundary wording, damaged-text detection, Go tests, Go vet, frontend tests/build, and optional GoReleaser, govulncheck, gosec, syft, and cosign checks.
- `make security-scan` now exposes the comprehensive scan entry point for local and CI use.
- `scripts/validate-acme.sh` and `make validate-acme` now provide a read-only target-server validation gate for Phase 2 DNS, local health, OMO/Caddy service state, public HTTP/HTTPS entries, and TLS certificate metadata.

## In Progress

- Phase 7 target-server validation for real update restart/rollback behavior.
- Phase 2 target-server ACME validation on a real public server remains pending external environment, but the read-only validation command has been added.

## Known Risks

- `go` is not available in the WSL PATH. Windows Go 1.26.3 is available at `/mnt/c/Program Files/Go/bin/go.exe`, and Makefile auto-detects it as a fallback.
- In the current PowerShell session, system-level `git`, `go`, `gofmt`, `pnpm`, and `make` are not available in PATH. A project-local portable toolchain now exists under `.tools/` for Go, Node.js, pnpm, and MinGit; `.tools/` is ignored by Git.
- Binaries built with the Windows Go fallback are Windows PE executables even when the filename has no `.exe` suffix. Linux release targets should be built with a Linux Go toolchain or release automation.
- This directory is not yet a Git repository.
- `pnpm install` reports that esbuild build scripts are ignored by the local pnpm policy, but current frontend test and build commands completed successfully.
- Runtime verification uses Windows curl because the detected Go toolchain builds Windows PE executables in this environment.
- Installer release download URLs are placeholders for the future GitHub Releases pipeline; local `dist/omo` is used when present.
- ACME certificate provisioning cannot be fully validated in this local environment without a real domain resolving to this machine and externally reachable 80/443.
- The temporary initialization service switch is validated by unit tests and installer dry-run output, but a real systemd stop/start handoff still needs target-server validation.
- Caddy unavailable fallback is explicit and retryable, but a true internal ACME/self-signed alternative entry is still a later implementation option if the product requires serving a local HTTPS fallback.
- The installer can fetch sing-box release archives, but upstream sing-box releases do not expose a standalone checksum asset in the latest release metadata; release verification still needs the project-level supply-chain strategy before production distribution.
- Phase 3 config apply currently uses JSON validation only. Production service reload and `sing-box check` integration still need a target sing-box binary and service manager integration.
- Phase 4 subscription output now includes active managed service metadata. Health-aware service selection remains a follow-up once service health sampling is wired to subscriptions.
- The QR generator is intentionally small and local, and router coverage verifies SVG output. Wider scanner/client compatibility validation remains a follow-up once browser and mobile client test tooling is available.
- Phase 5 optional external provider checks require explicit operator configuration and are disabled by default. Wider provider compatibility validation remains pending until local and target-server test tooling is available.
- Phase 6 currently persists local and cross-instance trust workflows plus operator-confirmed one-hop configuration intent and health sampling. Full mTLS service-manager integration remains follow-up work.
- Phase 7 backup/restore source changes could not be formatted or test-compiled in the current PowerShell PATH because `gofmt` and `go` are unavailable.
- Phase 7 audit listing source changes could not be formatted or test-compiled in the current PowerShell PATH because `gofmt` and `go` are unavailable.
- Phase 7 update-check source changes could not be formatted or test-compiled in the current PowerShell PATH because `gofmt` and `go` are unavailable.
- Phase 7 release automation source/config changes could not be formatted, test-compiled, or checked with GoReleaser in the current PowerShell PATH because `gofmt`, `go`, `pnpm`, `make`, `goreleaser`, `cosign`, and `syft` are unavailable.
- Phase 7 update apply/rollback source changes could not be formatted or test-compiled in the current PowerShell PATH because `gofmt` and `go` are unavailable.
- Phase 7 security scan automation could not be fully executed in the current PowerShell PATH because `bash`, `go`, `pnpm`, `goreleaser`, `govulncheck`, `gosec`, `syft`, and `cosign` are unavailable.
- Phase 7 frontend operations pages and settings API changes could not be formatted or test-compiled in the current PowerShell PATH because `gofmt`, `go`, `pnpm`, and `make` are unavailable.
- Phase 3 service instance lifecycle, subscription descriptor, and MVP service profile changes now format and test with the project-local portable Go and Node.js toolchains. System-level `make build` remains unavailable because `make` is not installed, but equivalent frontend build plus Go binary build commands passed.

## Recent Commands

```bash
pwd && ls
rg --files -g 'AGENTS.md' -g 'README*' -g '*方案*' -g '*需求*' -g 'package.json' -g 'pyproject.toml' -g 'composer.json' -g 'go.mod' -g 'Cargo.toml' -g 'pom.xml' -g 'build.gradle*'
git status --short --branch
sed -n '1,240p' MASTER_DEVELOPMENT_PROMPT.md
sed -n '241,520p' MASTER_DEVELOPMENT_PROMPT.md
sed -n '521,1040p' MASTER_DEVELOPMENT_PROMPT.md
go version
node --version
pnpm --version
npm --version
mkdir -p openapi cmd/omo cmd/omoctl internal/{api,auth,audit,backup,bootstrap,caddy,configgen,diagnostics,distribution,jobs,pairing,protocol,settings,store/migrations,store/queries,subscription,update} internal/core/singbox web/src/routes web/src/lib web/static scripts deploy/systemd deploy/caddy docs
pnpm --dir web install
pnpm --dir web build
scripts/install.sh --dry-run
/mnt/c/Program\ Files/Go/bin/go.exe mod tidy
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
pnpm --dir web test
make build
/mnt/c/Windows/System32/curl.exe -fsS http://127.0.0.1:18082/api/system/health
/mnt/c/Program\ Files/Go/bin/go.exe get modernc.org/sqlite golang.org/x/crypto/argon2
/mnt/c/Program\ Files/Go/bin/go.exe mod tidy
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
pnpm --dir web test
pnpm --dir web build
make test
make build
/mnt/c/Windows/System32/curl.exe -fsS http://127.0.0.1:18087/api/bootstrap/status
/mnt/c/Windows/System32/curl.exe -fsS -X POST http://127.0.0.1:18087/api/bootstrap/start ...
/mnt/c/Windows/System32/curl.exe --max-time 2 -fsS http://127.0.0.1:18087/api/bootstrap/events
make test
make build
bash -n scripts/install.sh
scripts/install.sh --dry-run
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
pnpm --dir web test
pnpm --dir web build
/mnt/c/Windows/System32/curl.exe -fsS -X POST http://127.0.0.1:18088/api/auth/login ...
/mnt/c/Windows/System32/curl.exe -fsS http://127.0.0.1:18088/api/auth/me
/mnt/c/Windows/System32/curl.exe -fsS -X POST http://127.0.0.1:18088/api/auth/logout
make test
make build
/mnt/c/Windows/System32/curl.exe -sS -X POST http://127.0.0.1:18090/api/bootstrap/start ...
/mnt/c/Windows/System32/curl.exe -fsS http://127.0.0.1:18090/api/bootstrap/status
bash -n scripts/install.sh
scripts/install.sh --dry-run
./dist/omoctl dev-seed-ready phase2-access-sim/omo.db ops.example.com
/mnt/c/Windows/System32/curl.exe -sS -I http://127.0.0.1:18092/dashboard
/mnt/c/Program\ Files/Go/bin/gofmt.exe -w internal/api/router.go internal/api/router_test.go
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
pnpm --dir web test
pnpm --dir web build
/mnt/c/Program\ Files/Go/bin/go.exe build -o dist/omo ./cmd/omo
/mnt/c/Program\ Files/Go/bin/go.exe build -o dist/omoctl ./cmd/omoctl
/mnt/c/Program\ Files/Go/bin/gofmt.exe -w internal/store/store.go internal/store/store_test.go internal/auth/service.go internal/auth/service_test.go
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
bash -n scripts/install.sh
scripts/install.sh --dry-run
pnpm --dir web test
pnpm --dir web build
/mnt/c/Program\ Files/Go/bin/go.exe build -o dist/omo ./cmd/omo
/mnt/c/Program\ Files/Go/bin/go.exe build -o dist/omoctl ./cmd/omoctl
/mnt/c/Program\ Files/Go/bin/gofmt.exe -w cmd/omo/main.go internal/api/router.go internal/api/router_test.go internal/core/singbox/detector.go internal/core/singbox/detector_test.go
/mnt/c/Program\ Files/Go/bin/go.exe test ./...
bash -n scripts/install.sh
scripts/install.sh --dry-run
git status --short
Get-Command git -ErrorAction SilentlyContinue
where.exe go
where.exe gofmt
where.exe pnpm
where.exe make
rg -n "bypass|stealth|attack|evasion|bulk scanning|circumvention" internal openapi web scripts deploy docs -S
Select-String -Path internal\**\*.go,cmd\**\*.go,web\src\**\*.ts,web\src\**\*.svelte -Pattern "<replacement-marker>"
```

## Latest Verification

- 2026-05-24: `git status --short` attempted, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: `gofmt -w ...` attempted, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `C:\Program Files\Go\bin\gofmt.exe -w ...` attempted, but that path does not exist in the current environment.
- 2026-05-24: `where.exe go`, `where.exe gofmt`, `where.exe pnpm`, and `where.exe make` did not find the required local tools.
- 2026-05-24: `bash -lc ...` attempted to locate WSL tooling, but WSL reported no installed distribution.
- 2026-05-24: Static product-boundary wording scan found only the explicit non-goals in `docs/PROJECT_SPEC.md` and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static replacement-character scan across Go and frontend source found no replacement markers after the Phase 3 dashboard update.
- 2026-05-24: `where.exe go`, `where.exe pnpm`, `where.exe make`, and `where.exe git` still did not find the required local tools after the dashboard service card change, so Go tests, frontend tests, frontend build, and Makefile build remain blocked in this shell.
- 2026-05-24: Static replacement-character scan across Go, frontend, and docs source found no replacement markers after the Phase 4 subscription update.
- 2026-05-24: Static product-boundary wording scan after the Phase 4 subscription update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: `where.exe go`, `where.exe pnpm`, `where.exe make`, and `where.exe git` still did not find the required local tools after the Phase 4 subscription update, so Go tests, frontend tests, frontend build, and Makefile build remain blocked in this shell.
- 2026-05-24: `/subscriptions` frontend page added for smart subscription token create/rotate, one-time URL copy, and QR preview.
- 2026-05-24: `/init` and `/login` frontend source files were repaired from damaged encoded text to clean enterprise operations wording.
- 2026-05-24: Static product-boundary wording scan after the subscription UI update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static frontend damaged-text scan with `rg -n "鐧|鍒濆|杈圭|\?/span|\?/h1|\?/p|\?;" web\src -S` found no matches after repairing `/init`, `/login`, and adding `/subscriptions`.
- 2026-05-24: `go test ./...` attempted after the subscription UI update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the subscription UI update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the subscription UI update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the subscription UI update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `git status --short` attempted after the subscription UI update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 5 diagnostics backend and `/diagnostics` frontend page added, with API router coverage for run/latest/events and store/service tests for persisted reports.
- 2026-05-24: Static product-boundary wording scan after the Phase 5 diagnostics update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static frontend damaged-text scan after the Phase 5 diagnostics update found no matches.
- 2026-05-24: `go test ./...` attempted after the Phase 5 diagnostics update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 5 diagnostics update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 5 diagnostics update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 5 diagnostics update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `git status --short` attempted after the Phase 5 diagnostics update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 5 diagnostics providers expanded to include configured panel domain DNS, configured panel domain TLS, local 80/443 reachability, and sing-box access-core status.
- 2026-05-24: Static product-boundary wording scan after the Phase 5 provider update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static frontend damaged-text scan after the Phase 5 provider update found no matches.
- 2026-05-24: `go test ./...` attempted after the Phase 5 provider update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 5 provider update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 5 provider update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 5 provider update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 5 optional operator-configured external provider support added through `/api/settings`, diagnostics service integration, tests, OpenAPI schemas, and `/diagnostics` UI controls.
- 2026-05-24: `git status --short` attempted during recovery, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: `gofmt -w internal\settings\diagnostics.go internal\settings\security_test.go internal\diagnostics\service.go internal\diagnostics\service_test.go internal\api\router.go internal\api\router_test.go internal\api\panel_access_test.go` attempted after the optional provider update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the optional provider update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the optional provider update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `go test ./...` attempted after the optional provider update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the optional provider update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the optional provider update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the optional provider update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `where.exe go gofmt pnpm make git` did not find the required local tools.
- 2026-05-24: `git status --short` attempted before the Phase 6 cascade update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 6 cascade pairing local trust workflow added with OpenAPI schemas, SQLite migration, `internal/pairing`, router endpoints, tests, and `/cascade` UI.
- 2026-05-24: `gofmt -w cmd\omo\main.go internal\api\router.go internal\api\router_test.go internal\pairing\service.go internal\pairing\service_test.go internal\store\store.go internal\store\store_test.go` attempted after the Phase 6 cascade update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the Phase 6 cascade update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 6 cascade update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `go test ./...` attempted after the Phase 6 cascade update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 6 cascade update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 6 cascade update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 6 cascade update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `git status --short` attempted before the Phase 6 peer exchange update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 6 cross-instance pairing exchange added with `/api/pairing/exchange`, HTTPS peer client flow from `/api/pairing/accept`, one-time remote code consumption, local peer identity public key persistence, router CSRF exemption for the machine-to-machine endpoint, OpenAPI schemas, and service/router/store tests.
- 2026-05-24: `gofmt -w cmd\omo\main.go internal\api\router.go internal\api\router_test.go internal\pairing\service.go internal\pairing\service_test.go internal\store\store.go internal\store\store_test.go` attempted after the Phase 6 peer exchange update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the Phase 6 peer exchange update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 6 peer exchange update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `go test ./...` attempted after the Phase 6 peer exchange update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 6 peer exchange update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 6 peer exchange update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 6 peer exchange update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `where.exe go`, `where.exe pnpm`, `where.exe make`, and `where.exe git` did not find the required local tools after the Phase 6 peer exchange update.
- 2026-05-24: `git status --short` attempted before the Phase 6 cascade config update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 6 one-hop cascade configuration planning and confirmed apply added with OpenAPI schemas, store pair-state helpers, backend-generated configuration preview/write path, durable apply job, audit records, route coverage, and `/cascade` UI controls.
- 2026-05-24: `gofmt -w internal\api\router.go internal\api\router_test.go internal\pairing\service.go internal\pairing\service_test.go internal\store\store.go internal\store\store_test.go` attempted after the Phase 6 cascade config update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the Phase 6 cascade config update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 6 cascade config update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 6 cascade config update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 6 cascade config update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the Phase 6 cascade config update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 6 cascade config update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `git status --short` attempted before the Phase 6 cascade health update, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 6 live cascade health sampling added with OpenAPI schema, `POST /api/cascade/health/sample`, bounded HTTPS node health sampler, cascade node health persistence, durable sampling job, route/service/store tests, and `/cascade` UI health controls.
- 2026-05-24: `gofmt -w internal\api\router.go internal\api\router_test.go internal\pairing\service.go internal\pairing\service_test.go internal\store\store.go internal\store\store_test.go` attempted after the Phase 6 cascade health update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the Phase 6 cascade health update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 6 cascade health update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 6 cascade health update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 6 cascade health update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the Phase 6 cascade health update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 6 cascade health update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `where.exe go`, `where.exe pnpm`, `where.exe make`, and `where.exe git` did not find the required local tools after the Phase 6 cascade health update.
- 2026-05-24: `git status --short` attempted during Phase 7 recovery, but `git` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 7 backup/restore backend slice added with OpenAPI schemas, `internal/backup`, store backup record helpers, SQLite snapshot/restore helpers, API routes, service tests, store tests, and router tests.
- 2026-05-24: `gofmt -w internal\store\store.go internal\backup\service.go internal\backup\service_test.go internal\api\router.go internal\api\router_test.go cmd\omo\main.go` attempted after the Phase 7 backup update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: Static Phase 7 backup implementation scan confirmed concrete backup OpenAPI schemas and backend route/store/service wiring.
- 2026-05-24: Static product-boundary wording scan after the Phase 7 backup update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 7 backup update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `go test ./...` attempted after the Phase 7 backup update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 7 backup update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 7 backup update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 7 backup update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 7 audit listing API added with OpenAPI schemas, `internal/audit`, store query helpers, API route, service tests, store tests, and router tests.
- 2026-05-24: `gofmt -w internal\store\store.go internal\audit\service.go internal\audit\service_test.go internal\api\router.go internal\api\router_test.go cmd\omo\main.go` attempted after the Phase 7 audit update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the Phase 7 audit update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 7 audit update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 7 audit update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 7 audit update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static Phase 7 audit implementation scan confirmed concrete audit OpenAPI schemas and backend route/store/service wiring.
- 2026-05-24: Static product-boundary wording scan after the Phase 7 audit update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 7 audit update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: Phase 7 update-check API added with OpenAPI schema, `internal/update`, `GET /api/update/check`, `omo serve --update-manifest`, service tests, and router tests.
- 2026-05-24: `gofmt -w internal\update\service.go internal\update\service_test.go internal\api\router.go internal\api\router_test.go cmd\omo\main.go` attempted after the Phase 7 update-check update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the Phase 7 update-check update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the Phase 7 update-check update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the Phase 7 update-check update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the Phase 7 update-check update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static Phase 7 update-check implementation scan confirmed concrete update OpenAPI schema and backend route/service wiring.
- 2026-05-24: Static product-boundary wording scan after the Phase 7 update-check update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the Phase 7 update-check update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: Phase 7 backup archives were expanded to include runtime/version metadata plus configured managed Caddy and sing-box files when present; restore verifies per-file checksums, writes only to the current allowlisted managed config paths, preserves overwritten files as `.pre-restore-*`, and rolls back restored config files if database restore fails.
- 2026-05-24: `gofmt -w internal\backup\service.go internal\backup\service_test.go cmd\omo\main.go` attempted through `C:\Program Files\Go\bin\gofmt.exe`, but that executable is not available in this PowerShell environment.
- 2026-05-24: Static product-boundary wording scan after the backup metadata update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the backup metadata update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `go test ./...` attempted after the backup metadata update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the backup metadata update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the backup metadata update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the backup metadata update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `where.exe go`, `where.exe gofmt`, `where.exe pnpm`, `where.exe make`, and `where.exe git` did not find the required local tools during the backup metadata update.
- 2026-05-24: Phase 7 backup certificate metadata coverage added to `manifest.json`, sourced from `bootstrap.domain` and `bootstrap.phase2_result`, with a test asserting metadata-only capture and no private key material.
- 2026-05-24: Static product-boundary wording scan after the backup certificate metadata update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the backup certificate metadata update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `gofmt -w internal\backup\service.go internal\backup\service_test.go` attempted after the backup certificate metadata update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the backup certificate metadata update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the backup certificate metadata update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the backup certificate metadata update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 7 backup encryption at rest added with AES-256-GCM sealed `.omo-backup.enc` archives, secure-random local key generation, encrypted archive checksum persistence, encrypted restore decryption, and backward-compatible restore for older unencrypted ZIP archives.
- 2026-05-24: Static backup encryption implementation scan confirmed `encryptArchive`, `prepareReadableArchive`, key loading/creation, encrypted archive extension, and encrypted manifest test wiring.
- 2026-05-24: Static product-boundary wording scan after the backup encryption update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the backup encryption update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `gofmt -w internal\backup\service.go internal\backup\service_test.go` attempted after the backup encryption update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the backup encryption update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the backup encryption update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the backup encryption update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the backup encryption update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Phase 7 release automation added with `.goreleaser.yaml`, Linux amd64/arm64 builds for `omo` and `omoctl`, frontend release hooks, installer-compatible archive names, SHA-256 checksums, cosign `.sigstore.json` checksum signing, archive/binary SBOM generation, release version ldflags, shared runtime version metadata, `omoctl version`, and Makefile release targets.
- 2026-05-24: Official GoReleaser documentation was checked for archive naming, checksum generation, SBOM generation, and cosign bundle signing configuration.
- 2026-05-24: `gofmt -w cmd\omo\main.go cmd\omoctl\main.go internal\api\router.go internal\version\version.go` attempted after the release automation update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the release automation update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the release automation update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the release automation update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the release automation update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `goreleaser check` attempted after the release automation update, but `goreleaser` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the release automation update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the release automation update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: `where.exe go gofmt pnpm make git goreleaser cosign syft` did not find the required local tools after the release automation update.
- 2026-05-24: Phase 7 update apply/rollback added with concrete OpenAPI request/result schemas, `POST /api/update/apply`, `POST /api/update/rollback`, pre-update backup creation, HTTPS artifact download, checksum and required signature verification, binary replacement, systemd restart hook, health check hook, automatic previous-binary restore on failed apply, durable jobs, audit records, and service/router tests.
- 2026-05-24: `gofmt -w cmd\omo\main.go internal\api\router.go internal\api\router_test.go internal\update\service.go internal\update\service_test.go` attempted after the update apply/rollback update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the update apply/rollback update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the update apply/rollback update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the update apply/rollback update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the update apply/rollback update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `goreleaser check` attempted after the update apply/rollback update, but `goreleaser` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the update apply/rollback update found only the explicit non-goals in `docs/PROJECT_SPEC.md`, the command record in `docs/STATUS.md`, and the API regression assertion guarding disallowed wording.
- 2026-05-24: Static damaged-text scan after the update apply/rollback update found only the existing command record in `docs/STATUS.md` and no source matches.
- 2026-05-24: Static update implementation scan confirmed update apply/rollback job constants, OpenAPI result schemas, confirmation error mapping, update workspace wiring, automatic rollback error code, signature verifier wiring, and rollback path persistence.
- 2026-05-24: `where.exe go gofmt pnpm make git goreleaser cosign syft` did not find the required local tools after the update apply/rollback update.
- 2026-05-24: Phase 7 security scan integration added with `scripts/security-scan.sh`, `make security-scan`, required product-boundary/damaged-text/Go/frontend checks, and optional GoReleaser, govulncheck, gosec, syft, and cosign checks.
- 2026-05-24: Security scan product-boundary guard was tightened to exclude the scanner script's own forbidden-word pattern so the self-check does not report its own regex as a product text violation.
- 2026-05-24: Security scan damaged-text guard was rewritten with ASCII Unicode escape patterns so the scanner source remains stable in the current PowerShell encoding environment.
- 2026-05-24: Added `scripts/validate-acme.sh` plus `make validate-acme` and operations documentation for the final Phase 2 target-server ACME validation gate.
- 2026-05-24: `bash scripts/validate-acme.sh --help` attempted, but Windows `bash.exe` reported that WSL has no installed distribution, so the script could not be executed in this shell.
- 2026-05-24: `gofmt -w cmd\omo\main.go internal\api\router.go internal\api\router_test.go internal\update\service.go internal\update\service_test.go` attempted after the target validation update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the target validation update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the target validation update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the target validation update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the target validation update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `make security-scan` attempted after the target validation update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the target validation update returned no source matches.
- 2026-05-24: Static damaged-text scan after the target validation update returned no source matches.
- 2026-05-24: Static target validation implementation scan confirmed `scripts/validate-acme.sh`, `make validate-acme`, and operations/architecture/task documentation references.
- 2026-05-24: `where.exe go gofmt pnpm make git goreleaser cosign syft bash` found only `C:\Windows\System32\bash.exe`; the other required local tools are not available in this PowerShell PATH.
- 2026-05-24: Added `/logs` and `/settings` frontend pages, expanded navigation, added frontend API types for backups/audit/update, and extended `/api/settings` plus OpenAPI with update manifest URL management.
- 2026-05-24: Added `/services` as the service library route and updated main console navigation to use it while preserving `/dashboard`.
- 2026-05-24: `gofmt -w internal\api\router.go internal\api\router_test.go internal\update\service.go` attempted after the operations UI update, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-24: `go test ./...` attempted after the operations UI update, but `go` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web test` attempted after the operations UI update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `pnpm --dir web build` attempted after the operations UI update, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-24: `make build` attempted after the operations UI update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: `make security-scan` attempted after the operations UI update, but `make` is not available in the current PowerShell PATH.
- 2026-05-24: Static product-boundary wording scan after the operations UI update returned no source matches.
- 2026-05-24: Static damaged-text scan after the operations UI update returned no source matches.
- 2026-05-24: Static operations UI implementation scan confirmed update manifest settings, audit logs page, settings page backup/update controls, and navigation references.
- 2026-05-25: Added OpenAPI-concrete `/api/system/overview`, `/api/services`, `POST /api/services`, and `PATCH /api/services/{id}` schemas and backend route/store implementations for persisted managed service instances.
- 2026-05-25: Added service instance route tests for system overview, create/list/update, unknown profile rejection, and invalid service input rejection.
- 2026-05-25: Updated the service library frontend to consume `/api/services`, display persisted managed service instances, and create planned instances through backend APIs while keeping apply/rollback backend-owned.
- 2026-05-25: Added SQLite migration `0005_service_instance_metadata.sql` for service instance display names.
- 2026-05-25: `git status --short` attempted, but `git` is not available in the current PowerShell PATH.
- 2026-05-25: `gofmt -w internal\store\store.go internal\api\router.go internal\api\router_test.go` attempted after service API completion, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-25: `go test ./...` attempted after service API completion, but `go` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web test` attempted after service API completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web build` attempted after service API completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `make build` attempted after service API completion, but `make` is not available in the current PowerShell PATH.
- 2026-05-25: `make security-scan` attempted after service API completion, but `make` is not available in the current PowerShell PATH.
- 2026-05-25: Static product-boundary wording scan after service API completion returned no source matches.
- 2026-05-25: Static damaged-text scan after service API completion returned no source matches.
- 2026-05-25: Static service API implementation scan confirmed OpenAPI schemas, system overview route, service list/create/update routes, store helpers, frontend API types, and service library consumption.
- 2026-05-25: `where.exe go gofmt pnpm make git goreleaser cosign syft bash` found only `C:\Windows\System32\bash.exe`; the other required local tools are not available in this PowerShell PATH.
- 2026-05-25: Added service instance lifecycle synchronization for config apply/rollback jobs, active managed service metadata in backend-generated subscription descriptors, and service library state refresh from returned job instances.
- 2026-05-25: Adjusted service config job completion order so service instance synchronization succeeds before the durable job is marked successful; sync failures now mark the job failed with `SERVICE_INSTANCE_SYNC_FAILED`.
- 2026-05-25: Added unit coverage for service instance apply/rollback state transitions and subscription output containing active service metadata.
- 2026-05-25: `git status --short` attempted during service lifecycle completion, but `git` is not available in the current PowerShell PATH.
- 2026-05-25: `gofmt -w internal\store\store.go internal\store\store_test.go internal\configgen\manager.go internal\configgen\service.go internal\configgen\service_test.go internal\subscription\service.go internal\subscription\service_test.go internal\api\router_test.go` attempted during service lifecycle completion, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-25: `go test ./...` attempted during service lifecycle completion, but `go` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web test` attempted during service lifecycle completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web build` attempted during service lifecycle completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `make build` attempted during service lifecycle completion, but `make` is not available in the current PowerShell PATH.
- 2026-05-25: `make security-scan` attempted during service lifecycle completion, but `make` is not available in the current PowerShell PATH.
- 2026-05-25: Static product-boundary wording scan after service lifecycle completion returned no source matches.
- 2026-05-25: Static damaged-text scan after service lifecycle completion returned no source matches.
- 2026-05-25: `where.exe go gofmt pnpm make git goreleaser cosign syft bash` found only `C:\Windows\System32\bash.exe`; `C:\Program Files\Go\bin\go.exe`, `C:\Program Files\Go\bin\gofmt.exe`, `C:\Program Files\nodejs\pnpm.cmd`, and `C:\Program Files\nodejs\npm.cmd` were also not present.
- 2026-05-25: Added the two remaining MVP service profiles from the project prompt: lightweight fallback access and mobile optimized access.
- 2026-05-25: Updated OpenAPI service profile enums, frontend service profile category types, protocol registry tests, system overview expectations, config listen-port mapping, and `docs/PROTOCOL_PROFILES.md` for the five-profile MVP service catalog.
- 2026-05-25: `gofmt -w internal\protocol\profile.go internal\protocol\profile_test.go internal\configgen\manager.go internal\api\router_test.go internal\configgen\service.go` attempted after MVP profile completion, but `gofmt` is not available in the current PowerShell PATH.
- 2026-05-25: `go test ./...` attempted after MVP profile completion, but `go` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web test` attempted after MVP profile completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `pnpm --dir web build` attempted after MVP profile completion, but `pnpm` is not available in the current PowerShell PATH.
- 2026-05-25: `make build` attempted after MVP profile completion, but `make` is not available in the current PowerShell PATH.
- 2026-05-25: `git status --short` attempted after MVP profile completion, but `git` is not available in the current PowerShell PATH.
- 2026-05-25: Static product-boundary wording scan after MVP profile completion returned no source matches.
- 2026-05-25: Static damaged-text scan after MVP profile completion returned no source matches.
- 2026-05-25: Static MVP profile implementation scan confirmed lightweight fallback and mobile optimized access in the protocol registry, OpenAPI enums, frontend types, tests, and protocol research documentation.
- 2026-05-25: `where.exe go gofmt pnpm make git goreleaser cosign syft bash` found only `C:\Windows\System32\bash.exe`; the other required local tools are not available in this PowerShell PATH.
- 2026-05-25: Installed a project-local portable toolchain under `.tools/`: Go 1.26.3, Node.js 24.16.0, pnpm 11.3.0, and MinGit 2.54.0. `.tools/` was added to `.gitignore`.
- 2026-05-25: `gofmt` completed successfully through `.tools\go\bin\gofmt.exe`.
- 2026-05-25: `go test ./...` completed successfully using project-local `GOCACHE`, `GOMODCACHE`, `GOPROXY=https://goproxy.cn,direct`, and `GOSUMDB=sum.golang.google.cn`.
- 2026-05-25: `pnpm --dir web test` completed successfully after approving the `esbuild` build script for the local frontend dependency tree.
- 2026-05-25: `pnpm --dir web build` completed successfully and regenerated the embedded static frontend fallback under `cmd/omo/web`.
- 2026-05-25: `go build -o dist\omo.exe ./cmd/omo` and `go build -o dist\omoctl.exe ./cmd/omoctl` completed successfully with the project-local Go toolchain.
- 2026-05-25: `make build` was attempted after restoring the portable toolchain, but `make` remains unavailable in the current PowerShell PATH. The equivalent frontend build plus Go binary build steps passed.
- 2026-05-25: `git status --short` was attempted through project-local MinGit, but the directory is not yet initialized as a Git repository.
- 2026-05-25: `go vet ./...` completed successfully with the project-local Go toolchain.
- 2026-05-25: MinGit initialized the local repository and the current branch is `main`. Git metadata operations require the project-local MinGit plus `safe.directory=C:/Users/41419/Desktop/omo` in this sandbox.
- 2026-05-25: Project-local MinGit does not include GNU `make`; `make build` remains a host-tooling gap, while the Makefile-equivalent build steps have passed.
- 2026-05-25: `scripts/security-scan.sh` was attempted with MinGit `sh.exe`, but the script requires `bash` semantics and a Unix tool PATH; required checks were run individually instead: product-boundary scan, damaged-text scan, `go test ./...`, `go vet ./...`, `pnpm --dir web test`, and `pnpm --dir web build`.
- 2026-05-25: Added README.md as the operator-facing project entry with one-command install, first initialization recovery, upgrade, conservative uninstall, purge uninstall, service health checks, target HTTPS validation, backup/update notes, developer commands, and product-boundary guidance.
- 2026-05-25: Added `scripts/upgrade.sh` and `scripts/uninstall.sh`; `scripts/install.sh` now uses the `clover-eric/omo` release repository, resolves the latest release tag, installs `omoctl` when available, and verifies checksums against the actual archive filename.
- 2026-05-25: Updated `.goreleaser.yaml` to publish releases under `clover-eric/omo` and include the upgrade/uninstall lifecycle scripts in release archives.
- 2026-05-25: Added `.gitattributes` to keep shell scripts, Markdown, and YAML files on LF line endings for Linux server execution and release packaging.
- 2026-05-25: `go test ./...` passed with the project-local Go toolchain after the README/lifecycle script update.
- 2026-05-25: `go vet ./...` passed with the project-local Go toolchain after the README/lifecycle script update.
- 2026-05-25: `pnpm --dir web test` passed after rerunning outside the sandbox; the first sandbox attempt failed with EPERM while reading the Vitest dependency file.
- 2026-05-25: `pnpm --dir web build` passed after rerunning outside the sandbox; the first sandbox attempt failed with EPERM while reading the Vite dependency file. The build reported only the existing Svelte deprecated `<svelte:component>` warning in `web/src/routes/diagnostics/+page.svelte`.
- 2026-05-25: `go build -o dist\omo.exe ./cmd/omo` and `go build -o dist\omoctl.exe ./cmd/omoctl` passed with the project-local Go toolchain after the README/lifecycle script update.
- 2026-05-25: `sh -n scripts/install.sh scripts/upgrade.sh scripts/uninstall.sh` passed through project-local MinGit `sh.exe`.
- 2026-05-25: Static product-boundary wording and damaged-text scans returned no source matches after the README/lifecycle script update.
- 2026-05-25: Fixed `scripts/install.sh` Caddy apt keyring placement for Ubuntu/Debian so the Cloudsmith repository source can verify `caddy-stable-archive-keyring.gpg` from `/usr/share/keyrings`.
- 2026-05-25: `sh -n scripts/install.sh` passed after the Caddy keyring installer fix.
- 2026-05-25: Hardened `scripts/install.sh` for partially failed Caddy apt setup by repairing a pre-existing `caddy-stable.list` source when its keyring is missing before the next `apt-get update`.
- 2026-05-25: `sh -n scripts/install.sh` and `git diff --check` passed after the partial Caddy apt setup recovery fix.
- 2026-05-25: Fixed `scripts/install.sh` OMO release version handling so sourcing `/etc/os-release` cannot overwrite the requested OMO `--version latest` value with the operating system `VERSION` string.
- 2026-05-25: `sh -n scripts/install.sh` and `git diff --check` passed after the OMO version variable isolation fix.
- 2026-05-25: Added `deploy/bootstrap/` Linux amd64/arm64 bootstrap archives and checksum metadata so `scripts/install.sh --version latest` can fall back to a main-branch test snapshot when GitHub Releases are not published yet.
- 2026-05-25: Cross-compiled bootstrap `omo` and `omoctl` binaries for Linux amd64 and arm64 with the project-local Go toolchain, packaged them with lifecycle scripts, and verified archive contents plus checksum metadata.
- 2026-05-25: Fixed generated `omo-init-watch.service` so the health retry loop does not use command substitution while rendering the systemd unit.
- 2026-05-25: `sh -n scripts/install.sh` and `git diff --check` passed after the initialization watcher unit rendering fix.
- 2026-05-25: Updated initialization UX so the first-run page defaults to Simplified Chinese, exposes Chinese/English and light/dark preference controls, and lowers the administrator password requirement to at least 8 characters with letters and numbers.
- 2026-05-25: Fixed initialization token refresh for repeated installer runs by allowing `OMO_INIT_TOKEN` from the current systemd environment file to replace a stale uninitialized token hash in SQLite.
- 2026-05-25: Refreshed embedded frontend assets and Linux bootstrap archives so new-server installs receive the Simplified Chinese initialization page and refreshed token handling.
- 2026-05-25: `go test ./...`, `pnpm --dir web test`, and `pnpm --dir web build` passed after the initialization UX/password/token fixes.
- 2026-05-25: Extended `.gitattributes` to keep Go, Svelte, TypeScript, and CSS source files on LF line endings from Windows workspaces.
- 2026-05-25: Completed the initialization-page language/theme controls by rewriting corrupted UTF-8 strings, translating bootstrap state labels for Chinese/English, adding explicit light/dark CSS overrides that beat system color-scheme media rules, and making the main initialization button submit retry confirmation automatically after a failed attempt.
- 2026-05-25: Refreshed embedded frontend assets and Linux bootstrap archives after the complete language/theme/retry UX fix.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, `go test ./...`, and `go vet ./...` passed after the complete language/theme/retry UX fix.
- 2026-05-25: Fixed Caddy entry application to pass `--adapter caddyfile` for both validate and reload operations so OMO-managed snippet files with `.tmp` names are parsed as Caddyfile syntax instead of JSON.
- 2026-05-25: Improved bootstrap Phase 2 failure reporting so Caddy configuration errors are preserved in initialization events and no longer collapse into a generic startup failure.
- 2026-05-25: Refreshed Linux bootstrap archives after the Caddyfile adapter fix.
- 2026-05-25: `go test ./...` and `pnpm --dir web build` passed after the Caddyfile adapter fix.
- 2026-05-25: Target-server diagnostics for `hk2.i3.pub` confirmed DNS resolved to the server IP while TCP 80/443 were not listening and the OMO-managed Caddy snippet remained the placeholder, indicating the server was still running a pre-fix temporary bootstrap state.
- 2026-05-25: Hardened `scripts/install.sh` for repeatable target-server testing by stopping any existing `omo.service`, `omo-init.service`, and `omo-init-watch.service` before writing fresh units, initialization token, and recovery link.
- 2026-05-25: Refreshed Linux bootstrap archives and `deploy/bootstrap/checksums.txt` after the repeatable installer recovery fix.
- 2026-05-25: `sh -n scripts/install.sh`, `go test ./...`, `pnpm --dir web build`, archive content checks, and `git diff --check` passed after the repeatable installer recovery fix. The frontend build still reports only the existing Svelte `<svelte:component>` deprecation warning in the diagnostics page.
- 2026-05-25: Target-server testing reached `https://hk2.i3.pub/dashboard`, but the browser reported `ERR_SSL_PROTOCOL_ERROR`, indicating OMO had redirected before a valid TLS handshake was available.
- 2026-05-25: Hardened bootstrap Phase 2 so Caddy configuration alone is not enough to complete initialization; OMO now waits for a successful TLS certificate handshake, records `TLS_CERTIFICATE_NOT_READY` as a retryable temporary-entry state when needed, and preserves the temporary initialization path.
- 2026-05-25: Added installer recovery support for already-created administrators by refreshing the environment-provided initialization token on reinstall and allowing HTTPS entry recovery without a stale retry flag.
- 2026-05-25: Refreshed embedded frontend assets plus Linux bootstrap archives/checksums after the TLS readiness and recovery-token fixes.
- 2026-05-25: `go test ./...` and `pnpm --dir web build` passed after the TLS readiness and recovery-token fixes. The frontend build still reports only the existing Svelte `<svelte:component>` deprecation warning in the diagnostics page.
- 2026-05-25: Target-server testing reached the HTTPS dashboard but reported `ERR_TOO_MANY_REDIRECTS`, caused by the backend not recognizing Caddy's trusted `X-Forwarded-Proto: https` header on loopback reverse-proxy traffic.
- 2026-05-25: Fixed panel access enforcement and secure cookie decisions to trust forwarded HTTPS only from loopback reverse-proxy requests, added redirect-loop regression tests, and refreshed Linux bootstrap archives/checksums.
- 2026-05-25: `go test ./...` passed after the trusted forwarded HTTPS fix.
- 2026-05-25: Fixed the post-initialization console UX: `/dashboard` is now a real Chinese-first overview page, `/services` remains the service library, all main menu pages share one console shell, sidebar active states are consistent, and language/theme controls are embedded in the console topbar instead of floating over page content.
- 2026-05-25: Added a global frontend preference store for Simplified Chinese by default, persistent Chinese/English switching, persistent light/dark switching, and shared Chinese menu labels across overview, service library, distribution, cascade, diagnostics, audit logs, and settings.
- 2026-05-25: Hardened the frontend API client so non-JSON or unauthenticated responses become visible operator errors instead of leaving pages stuck in loading states; state-changing requests now explicitly use same-origin credentials.
- 2026-05-25: Removed duplicated console sidebars from main pages, fixed the missing/fragile distribution navigation import path by centralizing icons, and replaced the diagnostics deprecated dynamic component usage.
- 2026-05-25: Refreshed embedded frontend assets and Linux amd64/arm64 bootstrap archives/checksums after the console interaction fixes.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, and `go test ./...` passed after the console interaction fixes.
- 2026-05-25: Hardened installer reruns so TCP 80/443 already held by Caddy is accepted as the expected managed entry state instead of blocking reinstall recovery with a false port-in-use failure.
- 2026-05-25: Updated the SvelteKit frontend shell to use `$app/state` instead of legacy `$app/stores` for runes-mode page state and guarded browser-only preference event dispatch, then refreshed embedded frontend assets and Linux bootstrap archives to address a target-server initialization white screen.
- 2026-05-25: Updated the embedded SPA static handler to serve generated route HTML files such as `/init -> init.html` before falling back to `index.html`, making first-load initialization pages more reliable on temporary HTTP entry ports.
- 2026-05-25: Target-server white-screen diagnostics showed `/init` HTML loaded while `/_app` JavaScript assets were redirected to the HTTPS panel domain; panel access middleware now allows static frontend assets through the temporary initialization entry.
- 2026-05-25: Fixed completed-initialization recovery UX: `/api/bootstrap/status` now reports `initialized: true` after successful bootstrap, and the `/init` page automatically redirects to `https://{domain}/dashboard` when status already shows a completed domain entry.
- 2026-05-25: Refreshed embedded frontend assets and Linux amd64/arm64 bootstrap archives/checksums after the completed-initialization redirect fix.
- 2026-05-25: `go test ./internal/bootstrap ./internal/api` and `pnpm --dir web build` passed after the completed-initialization redirect fix.
- 2026-05-25: Improved the light-mode console visual system with cleaner surfaces, navigation active states, card depth, focused inputs, and more polished button states; initialization progress now eases toward real job progress and shows a short ready transition before entering the HTTPS dashboard.
- 2026-05-25: Hardened repeated installer recovery so `omo-init-watch.service` detects an already completed bootstrap state through the temporary service status endpoint, resets any failed regular service state, and starts `omo.service` instead of leaving Caddy with a `127.0.0.1:8080` 502.
- 2026-05-25: `sh -n scripts/install.sh`, `pnpm --dir web test`, `pnpm --dir web build`, `go test ./...`, and `git diff --check` passed after the light-mode and installer recovery improvements.
- 2026-05-25: Target-server recovery confirmed the desired service state after manual cleanup: `omo.service` active on `127.0.0.1:8080`, temporary initialization services disabled and inactive, and local health returning `success:true`.
- 2026-05-25: Installer recovery was tightened further so after the temporary service becomes healthy, `scripts/install.sh` immediately detects an existing completed bootstrap state, starts and health-checks the regular panel service, removes temporary init files, and disables the temporary services without waiting for a separate watcher cycle.
- 2026-05-25: `sh -n scripts/install.sh`, `go test ./internal/bootstrap ./internal/api ./cmd/omo`, and `pnpm --dir web test` passed after the immediate initialized-handoff installer fix.
- 2026-05-25: Fixed console data loading regressions found on the target dashboard: service instances, configuration distribution tokens, backup records, cascade records, audit records, and job events now serialize empty lists as `[]` instead of `null`, with router regression coverage for empty services, subscriptions, backups, and audit logs.
- 2026-05-25: Hardened the service library, configuration distribution, settings, cascade, and audit frontend pages against legacy `null` list payloads so they render empty states instead of staying on a loading screen or crashing during `.length` access.
- 2026-05-25: Completed Chinese/English copy coverage for settings, cascade nodes, and audit logs, including top summary cards, form labels, action buttons, loading states, empty states, and confirmation controls.
- 2026-05-25: Refreshed embedded frontend assets and Linux amd64/arm64 bootstrap archives/checksums after the console loading and localization fixes.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, `go test ./...`, `go vet ./...`, and `git diff --check` passed after the console loading and localization fixes.
- 2026-05-25: Fixed remaining console localization leaks by making the language control show the next target language, mapping API error codes to Chinese operator messages, localizing service profile transport/security summaries, and keeping service instance status labels localized.
- 2026-05-25: Fixed service configuration apply on hardened systemd installs by passing an absolute managed sing-box config path (`/var/lib/omo/sing-box/config.json`) plus backup/update directories into regular and temporary OMO units, creating those directories during install, and returning a specific `SERVICE_CONFIG_WRITE_FAILED` API error for write-path failures.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, `go test ./...`, `go vet ./...`, `sh -n scripts/install.sh scripts/upgrade.sh scripts/uninstall.sh`, `git diff --check`, local HTTP health/service/apply smoke testing, bootstrap archive content checks, frontend leaked-service-string scan, and absolute systemd path scan passed after the localization and service apply fixes. Browser plugin verification was attempted, but the required Node browser execution tool was not exposed in this session.
- 2026-05-25: Fixed English-mode Chinese leakage in form defaults and browser-locale date fields by localizing cascade and distribution default values, replacing the native subscription expiration date picker with an OMO-controlled text input, and formatting console timestamps through the active OMO language instead of the host browser locale.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, `go test ./...`, `go vet ./...`, `sh -n scripts/install.sh scripts/upgrade.sh scripts/uninstall.sh`, and `git diff --check` passed after the English-mode localization cleanup. Linux amd64/arm64 bootstrap archives and checksum metadata were refreshed.
- 2026-05-25: Reworked `/services` into a guided access-plan workflow with profile selection, next-step actions, linked instance state, distribution handoff, and expert-only low-level details so ordinary operators do not need to choose raw protocol parameters.
- 2026-05-25: Added OpenAPI, store, service, and router support for subscription update/disable/delete; disabled or deleted tokens no longer serve `/s/{token}` content, while plaintext import URLs remain one-time reveal values returned only on create or rotate.
- 2026-05-25: Reworked `/subscriptions` into a distribution-entry management workflow with record selection, rotate-and-reveal URL handling, enable/disable, two-step delete, format links, and QR preview tied to the currently revealed token.
- 2026-05-25: `pnpm --dir web test`, `pnpm --dir web build`, and `go test ./internal/api ./internal/subscription ./internal/store` passed after the service library and distribution management UX update. Browser plugin visual verification was attempted, but the required Node browser execution tool was not exposed in this session.
- 2026-05-25: Refreshed Linux amd64/arm64 bootstrap archives and checksum metadata after the subscription management API and core console UX update.
- 2026-05-25: Full verification passed after archive refresh: `go test ./...`, `go vet ./...`, `pnpm --dir web test`, `pnpm --dir web build`, `sh -n scripts/install.sh scripts/upgrade.sh scripts/uninstall.sh`, and `git diff --check`.
- `bash -n scripts/install.sh`: passed.
- `scripts/install.sh --dry-run`: passed with sqlite/Caddy preparation, time-sync check, root-only initialization env/link files, temporary init service, init watcher, firewall guidance, and direct one-time initialization link output.
- `/mnt/c/Program Files/Go/bin/go.exe test ./...`: passed.
- `pnpm --dir web test`: passed, 2 tests.
- `pnpm --dir web build`: passed.
- `/mnt/c/Program Files/Go/bin/go.exe build -o dist/omo ./cmd/omo`: passed.
- `/mnt/c/Program Files/Go/bin/go.exe build -o dist/omoctl ./cmd/omoctl`: passed.
- `make test`: passed in the previous verification pass.
- `make build`: passed in the previous verification pass.
- Latest installer dry-run includes `/etc/omo/init-link.txt`, system time synchronization check, firewall/security-group guidance, and local temporary service health verification planning.
- Runtime Phase 1 simulation passed with generated initialization token: status before initialization required token; `POST /api/bootstrap/start` returned accepted job at 100%; status after initialization showed `phase1Complete: true`, `requiresToken: false`, and domain `ops.example.com`; SSE returned persisted bootstrap events.
- Runtime auth simulation passed: bootstrap created admin, login returned admin and cookie, `/api/auth/me` returned authenticated, logout revoked the cookie, and `/api/auth/me` returned unauthenticated.
- `internal/caddy` tests passed, including rollback on Caddy reload failure.
- Runtime Phase 2 domain-failure simulation passed: unresolved domain returned `DOMAIN_NOT_RESOLVED`; job state remained `DOMAIN_VERIFY` failed with a readable message; initialization token remained required for retry.
- Runtime panel access simulation passed: initialized HTTP/IP `/dashboard` returned `307 Temporary Redirect` to `https://ops.example.com/dashboard`.
- Unit coverage now verifies `CADDY_UNAVAILABLE` returns 503 and preserves retryable degraded bootstrap state.
- Unit coverage now verifies `GET /api/security/csrf` sets a readable SameSite=Lax CSRF cookie, POST requests without a matching token return `CSRF_TOKEN_INVALID`, and the frontend API client prepares the CSRF cookie before a first login POST.
- Unit coverage now verifies login rate-limit records persist failure counts, set lockouts at threshold, clear after successful login, and still block login after constructing a new auth service instance.
- Unit coverage now verifies sing-box version parsing, configured binary detection, missing binary status, unhealthy version command status, and `/api/core/singbox/status` envelope behavior.
- Latest installer dry-run includes sing-box detection/preparation and passes `--sing-box /usr/local/bin/sing-box` into generated regular and temporary service units.
