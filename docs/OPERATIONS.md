# OMO Operations

## Local Development

Expected Phase 0 flow after dependencies are installed:

```bash
pnpm --dir web install
pnpm --dir web build
go test ./...
make build
./dist/omo serve --addr 127.0.0.1:8080
```

Health check:

```bash
curl http://127.0.0.1:8080/api/system/health
```

## Deployment

Install the latest stable release from GitHub Releases:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/install.sh | sudo bash -s -- --channel stable
```

Preview installation actions:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/install.sh | bash -s -- --dry-run
```

Upgrade with binary backup and health validation:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/upgrade.sh | sudo bash -s -- --channel stable
```

Safely uninstall while preserving data and backups:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/uninstall.sh | sudo bash -s -- --keep-data
```

Remove all local OMO data only with explicit purge confirmation:

```bash
curl -fsSL https://raw.githubusercontent.com/clover-eric/omo/main/scripts/uninstall.sh | sudo bash -s -- --purge --confirm-purge
```

## New Server Initialization

The installer prints a temporary one-time initialization link and also stores it in:

```bash
/etc/omo/init-link.txt
```

If the terminal output is lost before initialization completes, read that file as root. The file is removed automatically after bootstrap succeeds.

The temporary initialization service is:

```bash
systemctl status omo-init --no-pager
journalctl -u omo-init -n 80 --no-pager
```

The regular panel service is:

```bash
systemctl status omo --no-pager
journalctl -u omo -n 80 --no-pager
```

Before requesting the HTTPS panel entry, confirm:

- the domain resolves to this server IP;
- TCP 80 and 443 are reachable from the public internet;
- the temporary initialization port printed by the installer is reachable from the administrator browser;
- server time is synchronized.

## Phase 2 Target Validation

After bootstrap completes on an authorized target server with a public domain,
run the read-only HTTPS entry validation:

```bash
scripts/validate-acme.sh --domain ops.example.com --expected-ip 203.0.113.10
```

Or through Make:

```bash
make validate-acme DOMAIN=ops.example.com EXPECTED_IP=203.0.113.10
```

The validation checks:

- DNS records for the panel domain;
- local OMO health endpoint;
- systemd status for `omo` and `caddy` when `systemctl` is available;
- public HTTP and HTTPS entry responses;
- public HTTPS health and panel paths;
- TLS certificate subject alternative names, issuer, and validity dates.

If the command passes on the target server, record the exact command output in
`docs/STATUS.md` and mark the Phase 2 ACME target validation task complete.
If `--expected-ip` is omitted, the script still checks DNS resolution but leaves
IP matching as a warning.
