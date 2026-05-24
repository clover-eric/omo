SHELL := /usr/bin/env bash

APP := omo
DIST_DIR := dist
GO_BIN ?= $(shell if command -v go >/dev/null 2>&1; then command -v go; elif [ -x "/mnt/c/Program Files/Go/bin/go.exe" ]; then printf '/mnt/c/Program Files/Go/bin/go.exe'; else printf go; fi)

.PHONY: help web-build test build validate-acme security-scan release-check release-snapshot clean

help:
	@printf "OMO development targets:\n"
	@printf "  make web-build  Build SvelteKit static assets\n"
	@printf "  make test       Run backend and frontend tests\n"
	@printf "  make build      Build frontend assets and Go binaries\n"
	@printf "  make validate-acme DOMAIN=ops.example.com EXPECTED_IP=203.0.113.10\n"
	@printf "  make security-scan Run local hardening and supply-chain scans\n"
	@printf "  make release-check     Validate GoReleaser configuration\n"
	@printf "  make release-snapshot  Build local release artifacts without publishing\n"
	@printf "  make clean      Remove build output\n"

web-build:
	pnpm --dir web install
	pnpm --dir web build

test:
	"$(GO_BIN)" test ./...
	pnpm --dir web test

build: web-build
	mkdir -p $(DIST_DIR)
	"$(GO_BIN)" build -o $(DIST_DIR)/$(APP) ./cmd/omo
	"$(GO_BIN)" build -o $(DIST_DIR)/omoctl ./cmd/omoctl

validate-acme:
	test -n "$(DOMAIN)" || (printf "DOMAIN is required\n" >&2; exit 2)
	bash scripts/validate-acme.sh --domain "$(DOMAIN)" $(if $(EXPECTED_IP),--expected-ip "$(EXPECTED_IP)",)

security-scan:
	bash scripts/security-scan.sh

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(DIST_DIR) cmd/omo/web web/.svelte-kit web/node_modules
