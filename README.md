# OMO

OMO is an enterprise boundary operations platform for authorized infrastructure management. It ships as a single Go service with an embedded Svelte console, SQLite persistence, Caddy-based HTTPS entry management, managed access service profiles, configuration distribution, server checkups, cascade node trust workflows, encrypted backups, audit logs, and operator-confirmed online updates.

OMO is designed for user-owned servers, user-owned networks, and explicitly authorized operational environments.

## One-Command Install

Supported targets:

- Ubuntu 20.04+
- Debian 11+
- AlmaLinux 8+
- Linux amd64 and arm64

Install the latest stable release:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/install.sh | sudo bash -s -- --channel stable
```

Preview the actions without changing the server:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/install.sh | bash -s -- --dry-run
```

Install a specific release tag:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/install.sh | sudo bash -s -- --version v0.1.0
```

The installer prepares the `omo` system user, required directories, SQLite, Caddy, sing-box, systemd units, and a temporary initialization entry. Release downloads use GitHub Releases from `clover-eric/omo`; publish a release first when installing from a clean server.

## First Initialization

After installation, the terminal prints a one-time initialization link:

```text
http://SERVER_IP:RANDOM_PORT/init?token=...
```

Open that link in an administrator browser, create the first administrator account, and configure the panel domain. If the terminal output is lost before initialization completes, recover the link as root:

```bash
sudo cat /etc/omo/init-link.txt
```

Before enabling the HTTPS panel entry, confirm that the panel domain resolves to this server, TCP 80/443 are reachable, the temporary initialization port is reachable from your browser, and server time is synchronized.

## Upgrade

Upgrade to the latest stable release:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/upgrade.sh | sudo bash -s -- --channel stable
```

Upgrade to a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/upgrade.sh | sudo bash -s -- --version v0.1.0
```

Preview an upgrade:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/upgrade.sh | bash -s -- --dry-run
```

The upgrade script saves the current binaries under `/var/backups/omo/pre-upgrade-*`, downloads the release archive, verifies `checksums.txt`, installs `omo` and `omoctl`, restarts `omo.service`, and checks `http://127.0.0.1:8080/api/system/health`. If restart or health validation fails, it restores the previous binaries when available.

## Safe Uninstall

Remove services and binaries while keeping configuration, data, logs, Caddy snippets, and backups:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/uninstall.sh | sudo bash -s -- --keep-data
```

Preview uninstall actions:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/uninstall.sh | bash -s -- --dry-run
```

Permanently remove OMO data and local backups only when you are sure:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/uninstall.sh | sudo bash -s -- --purge --confirm-purge
```

Default uninstall is intentionally conservative. `--purge --confirm-purge` removes `/etc/omo`, `/var/lib/omo`, `/var/log/omo`, `/var/backups/omo`, and `/etc/caddy/omo`.

## Operations

Service status:

```bash
sudo systemctl status omo --no-pager
sudo journalctl -u omo -n 80 --no-pager
```

Temporary initialization service status:

```bash
sudo systemctl status omo-init --no-pager
sudo journalctl -u omo-init -n 80 --no-pager
```

Local health check:

```bash
curl http://127.0.0.1:8080/api/system/health
```

Target-server HTTPS validation after bootstrap:

```bash
scripts/validate-acme.sh --domain ops.example.com --expected-ip 203.0.113.10
```

## Backups And Updates

The OMO console includes settings for encrypted backup creation, backup listing, confirmed restore, update manifest configuration, update check, update apply, and update rollback. Restore and update apply operations require explicit operator confirmation and are executed by the backend, not by frontend-generated scripts.

Backup archives are encrypted at rest and stored under the configured backup directory. The local encryption key is intentionally not included in backup archives.

## Developer Setup

```bash
pnpm --dir web install
pnpm --dir web test
pnpm --dir web build
go test ./...
go vet ./...
go build -o dist/omo ./cmd/omo
go build -o dist/omoctl ./cmd/omoctl
```

When `make` is available:

```bash
make build
make security-scan
```

Release builds are defined by `.goreleaser.yaml` and publish Linux `amd64`/`arm64` archives containing `omo`, `omoctl`, lifecycle scripts, operations docs, checksums, SBOMs, and checksum signing metadata.

## Product Boundary

OMO is for authorized enterprise remote operations, boundary access management, service health, diagnostics, configuration distribution, encrypted backup/restore, and managed infrastructure administration. Public documentation, UI text, and normal logs should stay within that operational boundary.
