SHELL := /bin/bash

GO ?= go
PNPM ?= pnpm

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

.PHONY: help install check test build package dev sdk-build sdk-test cometmind-build cometmind-test cometline-check cometline-build cometline-package cometline-dev port clean-log

help:
	@printf "Cometline targets:\n"
	@printf "  make install          Install Cometline frontend dependencies\n"
	@printf "  make check            Run SDK tests, CometMind tests, and Svelte checks\n"
	@printf "  make build            Build SDK, CometMind binary, and Cometline renderer\n"
	@printf "  make package          Build CometMind and package the Electron app\n"
	@printf "  make dev              Build CometMind and launch Electron dev app\n"
	@printf "  make port             Show process listening on 127.0.0.1:7700\n"
	@printf "  make clean-log        Remove ~/.cometmind/cometline.log\n"
	@printf "\nmake dev reads all provider settings from ~/.cometmind/cometline-settings.json\n"
	@printf "(configured in the in-app Settings panel). Optional one-off overrides:\n"
	@printf "  COMETMIND_PROVIDER, COMETMIND_MODEL, COMETMIND_BASE_URL, COMETMIND_API_KEY\n"
	@printf "  Example: COMETMIND_API_KEY=sk-... make dev\n"

install:
	cd cometline && $(PNPM) install

check: sdk-test cometmind-test cometline-check

test: check

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

cometline-build:
	cd cometline && $(PNPM) run build

cometline-package:
	cd cometline && $(PNPM) run build:electron

cometline-dev: dev

port:
	lsof -nP -iTCP:7700 -sTCP:LISTEN || true

clean-log:
	rm -f "$(HOME)/.cometmind/cometline.log"
