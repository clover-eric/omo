# OMO Tasks

## Phase 0: Scaffold And Standards

- [x] Create required repository directories.
- [x] Create `AGENTS.md`.
- [x] Create baseline project docs.
- [x] Create OpenAPI 3.1 contract.
- [x] Create SQLite migration baseline.
- [x] Create Go module and backend health endpoint.
- [x] Embed SvelteKit static assets in Go server.
- [x] Create SvelteKit static frontend shell.
- [x] Create Makefile.
- [x] Add initial deploy script/unit placeholders.
- [x] Run Phase 0 verification commands.

## Phase 1: Installer And Bootstrap

- [x] Implement initial `scripts/install.sh` dry-run checks.
- [x] Implement initialization token.
- [x] Implement bootstrap state machine.
- [x] Implement admin creation.
- [x] Implement jobs and SSE.
- [x] Expand installer from dry-run skeleton to real package/user/directory/systemd preparation.
- [x] Add one-command new-server initialization link with temporary systemd service.
- [x] Add root-only recovery file for the one-time initialization link.
- [x] Add installer time-sync check, firewall guidance, and temporary service health verification.
- [x] Add automatic switch from temporary initialization service to regular panel service after bootstrap.
- [x] Redirect an already completed temporary initialization page to the HTTPS dashboard.
- [x] Recover repeated installs that are already initialized by starting the regular panel service instead of waiting only for a new ready marker.
- [x] Close temporary initialization services immediately when a repeated install detects an already initialized regular panel.
- [x] Add bootstrap retry controls for failed jobs.
- [x] Make installer reruns stop stale OMO systemd services before writing a fresh initialization port, token, and recovery link.
- [x] Add login/logout flow using stored sessions.
- [x] Add CSRF protection middleware for browser state-changing APIs.
- [x] Add persistent login rate-limit records for restart-safe lockout.

## Phase 2: Domain, Caddy, HTTPS

- [x] Implement domain resolution checks.
- [x] Implement port checks.
- [x] Implement Caddy config generation and reload.
- [x] Implement certificate status reporting.
- [x] Preserve old Caddy config on reload failure.
- [x] Return readable domain failure messages.
- [x] Add installer Caddy detection and installation preparation.
- [x] Add expected server IP support for domain verification.
- [x] Enforce HTTPS-domain-only dashboard access after initialization.
- [x] Recognize trusted Caddy forwarded HTTPS requests to avoid dashboard redirect loops.
- [x] Add explicit degraded temporary-entry state when Caddy is unavailable.
- [x] Require a successful TLS certificate handshake before completing HTTPS bootstrap handoff.
- [x] Allow installer recovery tokens to repair HTTPS entry setup after an administrator already exists.
- [x] Add read-only target-server validation script for the ACME acceptance check.
- [ ] Validate full ACME issuance on a real target server with public domain and reachable 80/443.

## Phase 3: sing-box And Default Services

- [x] Implement sing-box version/install detection.
- [x] Implement service profile templates.
- [x] Cover the MVP service profile set: standard, high throughput, broad compatibility, lightweight fallback, and mobile optimized access.
- [x] Implement persisted service instance list/create/update APIs declared by OpenAPI.
- [x] Implement system overview API declared by OpenAPI.
- [x] Implement config apply and rollback.
- [x] Synchronize service instance state through config apply and rollback jobs.
- [x] Start/reload the managed sing-box entry after successful config apply and keep service credentials synchronized with subscription output.
- [x] Implement dashboard service cards.
- [x] Add `/services` frontend route for the service library.
- [x] Rework `/services` into a guided access-plan workflow with profile selection, instance status, backend apply state, and expert details separated from the ordinary operator path.
- [x] Tighten `/services` workflow actions so active plans lead directly to configuration distribution and planned/unplanned services expose only the next useful operation.

## Phase 4: Smart Subscriptions And QR Import

- [x] Implement subscription token create/list/rotate.
- [x] Implement multi-format subscription output.
- [x] Include active managed service metadata in backend-generated subscription descriptors.
- [x] Implement adaptive import page for unknown clients.
- [x] Implement QR code output.
- [x] Add configuration distribution UI for token create/rotate, import URL copy, and QR preview.
- [x] Add subscription status update and deletion APIs, with frontend management for select, rotate-and-reveal, enable/disable, delete, and one-time token visibility.
- [x] Generate subscription and QR URLs from the public HTTPS panel domain or trusted loopback reverse-proxy headers instead of loopback service addresses.
- [x] Generate concrete sing-box, Clash/Mihomo, and direct URI client entries from active backend-owned service credentials instead of placeholder metadata.
- [x] Replace fragile hand-written QR generation with standard QR encoding and make scanned codes land on a clear mobile import selection page.
- [x] Clarify the Service Library to Configuration Distribution workflow so operators can see the path from active service instance to device import.

## Phase 5: Server Checkup

- [x] Add diagnostics OpenAPI response schemas.
- [x] Implement durable server checkup job with SSE events.
- [x] Persist and return latest diagnostic report.
- [x] Add initial local runtime, CPU, memory, and loopback checks.
- [x] Add `/diagnostics` frontend page for running checkups and reading reports.
- [x] Add DNS, TLS, port, and access-core health providers.
- [x] Add optional operator-configured external provider support.

## Phase 6: Cascade Nodes

- [x] Add cascade pairing OpenAPI request and response schemas.
- [x] Implement short-lived one-time pairing code creation with hashed storage.
- [x] Implement pairing acceptance with signature verification, trust record creation, durable job state, and audit log entry.
- [x] Implement cascade node list, update, and delete APIs.
- [x] Add `/cascade` frontend page for pairing code creation, acceptance, trust records, and node revocation.
- [x] Implement cross-server HTTPS handshake between two running OMO instances.
- [x] Generate one-hop cascade configuration and require operator confirmation before apply.
- [x] Add live cascade link health sampling for latency, throughput, online state, and latest error.

## Phase 7: Updates, Backups, Hardening, Release

- [x] Add backup/restore OpenAPI request and response schemas.
- [x] Implement durable backup creation with SQLite snapshot archive and checksum.
- [x] Implement backup listing API.
- [x] Implement restore API with explicit operator confirmation and checksum verification.
- [x] Add backup/restore store, service, and router tests.
- [x] Add broader managed config and runtime metadata coverage to backup archives.
- [x] Add certificate metadata coverage to backup archives without private key material.
- [x] Add backup encryption at rest.
- [x] Implement online update check.
- [x] Implement update apply with pre-update backup, checksum/signature verification, health check, and automatic rollback.
- [x] Implement update rollback API.
- [x] Implement audit log listing API.
- [x] Add `/logs` frontend page for audit log review.
- [x] Add `/settings` frontend page for backup/restore, update operations, and diagnostics provider settings.
- [x] Add Chinese-first shared console shell with working language/theme controls across all main menu pages.
- [x] Return stable empty arrays for console list APIs and render empty/error states instead of persistent loading screens.
- [x] Complete Chinese/English switching coverage for service library, configuration distribution, cascade nodes, audit logs, and settings pages.
- [x] Localize service profile protocol summaries and operator-facing API errors in Chinese mode.
- [x] Remove English-mode Chinese leakage from localized form defaults, subscription expiration input placeholders, and console timestamp formatting.
- [x] Use an absolute managed sing-box configuration path in installed systemd units.
- [x] Improve light-mode console polish and smooth initialization progress/entry transition.
- [x] Split `/dashboard` into a real overview page and keep `/services` as the service library.
- [x] Add update manifest URL management to the settings API.
- [x] Add GoReleaser configuration, checksums, signatures, and SBOM generation.
- [x] Integrate comprehensive security scans.
- [x] Add README and operator lifecycle commands for one-command install, upgrade, and safe uninstall.
