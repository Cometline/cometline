SHELL := /bin/bash

GO ?= go
PNPM ?= pnpm

GO_MODULES := comet-sdk cometmind

# Dev tools run through `go run` at pinned versions so nothing needs installing
# and nothing is added to the modules' go.mod files.
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.8.0
SQLC ?= $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

# Provider settings default to empty so `make dev` reads everything from
# ~/.cometmind/cometline-settings.json. Pass any of these on the command line
# only when you want to override the saved settings, e.g.:
#   COMETMIND_API_KEY=sk-... make dev
COMETMIND_PROVIDER ?=
COMETMIND_MODEL ?=
COMETMIND_BASE_URL ?=
COMETMIND_API_KEY ?=
COMETMIND_WORKSPACE_PATH ?= $(CURDIR)
COMETMIND_BINARY_PATH ?= $(CURDIR)/cometmind/dist/cometmind

.PHONY: help install generate check-generated check-sqlc check test build package dev fmt fmt-check lint lint-budget test-race vuln readability sdk-build sdk-test cometmind-build cometmind-test cometline-check cometline-build cometline-package cometline-dev port clean-log

help:
	@printf "Cometline targets:\n"
	@printf "  make install          Install Cometline frontend dependencies\n"
	@printf "  make generate         Regenerate OpenAPI clients (TS + Go types)\n"
	@printf "  make check            Run codegen freshness, gofmt, lint, tests, and Svelte checks\n"
	@printf "  make fmt              Format Go code (gofmt + goimports)\n"
	@printf "  make lint             Run golangci-lint on the Go modules\n"
	@printf "  make lint-budget      Report funlen/gocyclo budget findings (non-blocking)\n"
	@printf "  make test-race        Run Go tests with the race detector\n"
	@printf "  make vuln             Run govulncheck on the Go modules\n"
	@printf "  make readability      Print the readability scorecard\n"
	@printf "  make build            Build SDK, CometMind binary, and Cometline renderer\n"
	@printf "  make package          Build CometMind and package the Electron app\n"
	@printf "  make dev              Build CometMind and launch Electron dev app\n"
	@printf "  make port             Show process listening on 127.0.0.1:7700\n"
	@printf "  make clean-log        Remove CometMind sidecar + gateway logs under ~/.cometmind/logs/\n"
	@printf "\nmake dev reads all provider settings from ~/.cometmind/cometline-settings.json\n"
	@printf "(configured in the in-app Settings panel). Optional one-off overrides:\n"
	@printf "  COMETMIND_PROVIDER, COMETMIND_MODEL, COMETMIND_BASE_URL, COMETMIND_API_KEY\n"
	@printf "  Example: COMETMIND_API_KEY=sk-... make dev\n"

install:
	cd cometline && $(PNPM) install

generate:
	cd cometline && $(PNPM) run generate:api
	cd cometmind && $(GO) generate ./internal/apigen

check-generated: generate
	@git diff --exit-code -- cometline/src/lib/generated/cometmind-api cometmind/internal/apigen/types.gen.go \
		|| (printf '\nGenerated API artifacts are out of date. Run make generate and commit.\n' && exit 1)

check-sqlc:
	cd cometmind && $(SQLC) generate
	@git diff --exit-code -- cometmind/internal/db \
		|| (printf '\nsqlc output is out of date. Run sqlc generate in cometmind/ and commit.\n' && exit 1)

check: check-generated check-sqlc fmt-check lint sdk-test cometmind-test cometline-check

test: check

fmt:
	@for m in $(GO_MODULES); do (cd $$m && $(GOLANGCI_LINT) fmt ./...) || exit 1; done

fmt-check:
	@unformatted="$$(gofmt -l $(GO_MODULES))"; \
	if [ -n "$$unformatted" ]; then \
		printf 'These files need gofmt (run make fmt):\n%s\n' "$$unformatted"; exit 1; \
	fi

lint:
	@for m in $(GO_MODULES); do (cd $$m && $(GOLANGCI_LINT) run ./...) || exit 1; done

lint-budget:
	-@for m in $(GO_MODULES); do (cd $$m && $(GOLANGCI_LINT) run -c .golangci.budget.yml ./...); done

test-race:
	@for m in $(GO_MODULES); do (cd $$m && $(GO) test -race ./...) || exit 1; done

vuln:
	@for m in $(GO_MODULES); do (cd $$m && $(GOVULNCHECK) ./...) || exit 1; done

readability:
	@scripts/readability-report.sh

build: sdk-build cometmind-build cometline-build

package: cometline-package

dev: cometmind-build
	cd cometline && \
		COMETMIND_PROVIDER="$(COMETMIND_PROVIDER)" \
		COMETMIND_MODEL="$(COMETMIND_MODEL)" \
		COMETMIND_BASE_URL="$(COMETMIND_BASE_URL)" \
		COMETMIND_API_KEY="$(COMETMIND_API_KEY)" \
		COMETMIND_WORKSPACE_PATH="$(COMETMIND_WORKSPACE_PATH)" \
		COMETMIND_BINARY_PATH="$(COMETMIND_BINARY_PATH)" \
		$(PNPM) run dev

sdk-build:
	cd comet-sdk && $(GO) build ./...

sdk-test:
	cd comet-sdk && $(GO) test ./...

cometmind-build:
	mkdir -p cometmind/dist
	cd cometmind && $(GO) build -o dist/cometmind .

cometmind-test:
	cd cometmind && $(GO) test ./...

cometline-check:
	cd cometline && $(PNPM) run check
	cd cometline && $(PNPM) run lint
	cd cometline && $(PNPM) run test

cometline-build:
	cd cometline && $(PNPM) run build

cometline-package:
	cd cometline && $(PNPM) run build:electron

cometline-dev: dev

port:
	lsof -nP -iTCP:7700 -sTCP:LISTEN || true

clean-log:
	rm -f "$(HOME)/.cometmind/logs/cometline.log" "$(HOME)/.cometmind/logs/cometline.log.1"
	rm -f "$(HOME)/.cometmind/logs/cometline-gateway.log" "$(HOME)/.cometmind/logs/cometline-gateway.log.1"
	# Legacy root-level paths (pre-logs/ migration)
	rm -f "$(HOME)/.cometmind/cometline.log" "$(HOME)/.cometmind/cometline.log.1"
	rm -f "$(HOME)/.cometmind/cometline-gateway.log" "$(HOME)/.cometmind/cometline-gateway.log.1"
