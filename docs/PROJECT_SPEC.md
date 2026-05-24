# OMO 边界运维管理平台 Project Spec

## Product Definition

OMO 边界运维管理平台 is a deployable operations platform for boundary access configuration, remote device management, server health checks, and operational monitoring in user-owned servers, user-owned networks, and explicitly authorized environments.

Package, service, directory, and binary names use `omo`.

## MVP Scope

The implementation follows the phased plan from `MASTER_DEVELOPMENT_PROMPT.md`.

Phase 0 establishes:

- Go backend module and command entry points.
- SvelteKit frontend shell built as static assets.
- OpenAPI 3.1 contract.
- SQLite migration baseline.
- Embedded frontend fallback served by the Go backend.
- Makefile and deployment/documentation skeleton.
- Health endpoint proving the binary can start.

Later phases add installer, bootstrap, HTTPS, sing-box configuration, service catalog, subscription distribution, diagnostics, cascade nodes, updates, backups, and release automation.

## Terms

- Boundary access: Authorized entry configuration for managed infrastructure.
- Access service: A versioned service profile generated and applied by the backend.
- Access instance: A running instance of an access service.
- Service library: User-facing collection of managed access service profiles.
- Smart subscription: Health-aware configuration distribution endpoint.
- Cascade node: A trusted one-hop node pairing managed by OMO.
- Server checkup: Legal diagnostics for owned or authorized servers.

## Explicit Non-Goals

OMO must not implement, promote, or document:

- Unauthorized third-party access, attack, stealth, or evasion workflows.
- Bulk scanning of third-party targets.
- Regulatory bypass or platform-rule circumvention.
- Third-party reputation queries without user-configured providers or API keys.
- Automatic high-risk operations without explicit user confirmation.

## User-Facing Language Rules

Use enterprise operations wording such as boundary access, remote operations, device management, service library, access service, access instance, configuration distribution, smart subscription, cascade node, server checkup, network quality, IP quality check, certificate status, and service health.

Low-level protocol names are reserved for expert details, generated configuration layers, and engineering docs.

## Acceptance Model

Each phase must be buildable, testable, runnable, and recoverable. Work is not complete until relevant docs, tests, and status files reflect reality.
