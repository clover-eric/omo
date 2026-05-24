# OMO Decisions

## 2026-05-23: Phase 0 Baseline Stack

Context: The project starts from only the master development prompt.

Decision: Create the required repository structure, Go module, OpenAPI contract, SQLite migration baseline, SvelteKit static frontend, and embedded Go server shell.

Impact: Later phases can extend concrete contracts and modules without replacing the project skeleton.

## 2026-05-23: API Response Envelope

Context: The specification requires a unified API format.

Decision: Implement an initial response helper in `internal/api` and document the same envelope in OpenAPI.

Impact: Handlers should return consistent success and error shapes from the beginning.

## 2026-05-23: Frontend Embedding

Context: The final artifact must be a single main binary with embedded frontend assets.

Decision: Build SvelteKit with `@sveltejs/adapter-static` into `web/build`, then embed that directory from `cmd/omo`.

Impact: `pnpm --dir web build` must run before `go build` when assets change.

## 2026-05-23: Phase 1 SQLite Store

Context: Bootstrap, jobs, sessions, and future modules need durable state without Redis.

Decision: Add a small embedded migration runner over `modernc.org/sqlite`, using SQLite WAL and foreign keys through the DSN.

Impact: Phase 1 can persist administrators, sessions, settings, jobs, and job events; later phases can add migrations without changing deployment topology.

## 2026-05-23: Bootstrap Event Persistence

Context: The specification requires long-running initialization work to be durable and streamed over SSE.

Decision: Add `job_events` as an append-only event table and make `/api/bootstrap/events` stream stored events by id.

Impact: The initialization page can recover progress after refresh and future jobs can reuse the same pattern.

## 2026-05-23: Phase 1 Installer Scope

Context: The installer must support real deployment preparation but the release pipeline, Caddy, and sing-box download paths are later phases.

Decision: Expand `scripts/install.sh` to validate OS, architecture, memory, disk, commands, systemd, and ports; prepare the `omo` system user, directories, binary placement, systemd unit, and dry-run output. It uses a local `dist/omo` binary when available and otherwise follows the future GitHub Release checksum flow.

Impact: Phase 1 dry-run is realistic without pretending that Phase 2/3 components are installed.

## 2026-05-23: Session Auth Foundation

Context: Administrators need to log into the main panel after initialization.

Decision: Add cookie-based session login/logout/me APIs, backed by hashed session tokens in SQLite and simple in-memory login failure lockout.

Impact: Phase 2 management APIs can build on a concrete session model; distributed rate limiting remains a later hardening topic.

## 2026-05-23: Caddy Config Rollback

Context: Phase 2 requires Caddy reload failures to avoid breaking the old entry configuration.

Decision: Implement Caddy application as render-to-temp, validate, atomic rename, reload, and restore previous config on reload failure.

Impact: The bootstrap process can surface readable failures while preserving the previous Caddy config. True ACME issuance still requires a real domain, open 80/443, and Caddy installed on the target server.

## 2026-05-23: HTTPS Domain Panel Access

Context: The specification forbids pure IP access to the main panel after initialization.

Decision: Add panel access middleware that redirects non-API panel requests to the configured HTTPS domain once bootstrap settings indicate the entry is configured.

Impact: Local bootstrap and APIs remain available, while `/dashboard` and other SPA routes move to the canonical HTTPS domain after initialization.

## 2026-05-23: New-Server Initialization Handoff

Context: A fresh server must show a direct one-time initialization link and close the temporary HTTP entry after the HTTPS panel is ready.

Decision: The installer creates a random-port `omo-init.service`, stores the one-time token in a root-only `/etc/omo/init.env`, writes a root-only recovery link at `/etc/omo/init-link.txt`, and starts `omo-init-watch.service`. Bootstrap success writes a ready marker, then the watcher starts the regular loopback-only `omo.service`, confirms its local health endpoint, deletes the initialization recovery files, and disables the temporary services.

Impact: First-run setup is reachable by direct server IP and random port, while the normal panel path is HTTPS-domain-only after initialization. Lost terminal output can be recovered before bootstrap completes. The systemd handoff still needs validation on a real target server with public DNS and reachable 80/443.

## 2026-05-23: Caddy Unavailable Bootstrap State

Context: Phase 2 requires a clear fallback state when Caddy cannot be used, and the installer path must not close the temporary initialization entry before HTTPS is actually ready.

Decision: Treat Caddy unavailability as a retryable degraded Phase 2 state. The bootstrap job records `entryMode=temporary_http` and `securityState=degraded`, returns `CADDY_UNAVAILABLE`, keeps the initialization token valid, and does not write the ready marker.

Impact: Operators receive an explicit remediation path and can retry after installing or fixing Caddy. The main panel is not marked HTTPS-ready until the Caddy-managed domain entry succeeds.

## 2026-05-23: Browser CSRF Protection

Context: Session authentication uses cookies, so browser-originated state-changing APIs need a request-bound safety check.

Decision: Add double-submit CSRF protection for non-safe `/api/*` methods. The backend issues a readable `omo_csrf` cookie on safe API responses and exposes `GET /api/security/csrf` for first-request preparation. The frontend API client prepares the cookie before POST requests and sends the value in `X-CSRF-Token`; the backend rejects missing or mismatched tokens with `CSRF_TOKEN_INVALID`.

Impact: Login, bootstrap, logout, and future management writes share the same browser CSRF contract without exposing the HttpOnly session cookie to JavaScript.

## 2026-05-23: Persistent Login Rate Limit

Context: The first login lockout implementation was process-local, so a service restart could clear failed login counters and active temporary lockouts.

Decision: Add a SQLite-backed `login_rate_limits` table and move administrator login failure counting to the store layer. Failed attempts update a durable counter, five consecutive failures set a five-minute lockout, and successful login clears the persisted record.

Impact: Administrator login protection now survives service restarts while keeping the deployment simple and Redis-free.

## 2026-05-23: sing-box Core Detection Boundary

Context: Phase 3 starts sing-box integration, but configuration generation and apply/rollback need a reliable core presence check first.

Decision: Add `internal/core/singbox` as the detection boundary. It resolves a configured path, `PATH`, or standard candidate paths, executes `sing-box version`, parses the version, and exposes status through `/api/core/singbox/status`. The installer prepares the binary and passes the path into `omo serve`.

Impact: Later service profile templates and configuration apply flows can depend on a single backend-owned core status contract without frontend-generated core configuration.

## 2026-05-24: Backend-Owned Service Profile Templates

Context: Phase 3 requires default services, but the product boundary says the frontend must not assemble core service configuration.

Decision: Add `internal/protocol` registry validation for three versioned service profile templates: standard secure access, high throughput access, and broad compatibility access. Expose the read-only template catalog through `/api/services/profiles` and document the response in OpenAPI before later configuration rendering and apply/rollback handlers.

Impact: Dashboard cards and config generation can consume one backend-owned profile contract. The templates carry dependency, client compatibility, score weight, golden test, and rollback metadata without exposing raw generated core configuration in the frontend.

## 2026-05-24: File-Level Config Apply And Rollback

Context: Phase 3 acceptance requires default service generation and automatic rollback when configuration fails.

Decision: Add `internal/configgen` as the backend-only configuration manager. It renders an OMO-managed sing-box JSON file from a service profile, validates the temporary output, backs up the active config to both `.previous` and a versioned backup, atomically replaces the managed config path, and restores `.previous` if post-apply validation fails. Apply and rollback API calls are persisted through the jobs table.

Impact: OMO now has a recoverable configuration operation path without frontend-generated core configuration. The current validator checks JSON structure; later Phase 3 work should replace or wrap it with `sing-box check`, service reload, and health checks.

## 2026-05-24: Dashboard Service Cards Read Backend State

Context: Phase 3 requires dashboard service cards while the product boundary keeps configuration rendering in the backend.

Decision: Update the dashboard to read service profile templates, sing-box core status, and apply/rollback job results from backend APIs. The frontend displays profile metadata and starts backend operations, but it does not build or mutate core configuration itself.

Impact: Operators can see the default service choices and trigger the recoverable configuration path from the panel while preserving the backend-owned configuration boundary.

## 2026-05-24: Hashed Smart Subscription Tokens

Context: Phase 4 requires subscription tokens, multi-format output, and adaptive import behavior.

Decision: Add `internal/subscription` with create/list/rotate management APIs and a public `/s/{token}` distribution endpoint. Tokens are generated with secure randomness, persisted only as hashes, and returned in plaintext only on create or rotate. Public requests are recorded with a hashed remote address and can receive sing-box JSON, Clash/Mihomo-style text, direct URI text, or an adaptive HTML import page.

Impact: Configuration distribution now has a recoverable token lifecycle and client-aware output boundary. QR code output remains the next Phase 4 task.
