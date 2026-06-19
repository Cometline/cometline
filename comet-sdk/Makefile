.PHONY: test test-live test-live-anthropic test-live-openai test-unit test-anthropic test-openai test-verbose lint build tidy

# ── Default ───────────────────────────────────────────────────────────────────
# Run all unit + integration tests. No live API calls. Safe for CI.
test:
	go test ./...

# ── Verbose ───────────────────────────────────────────────────────────────────
test-verbose:
	go test -v ./...

# ── Per-package unit tests ────────────────────────────────────────────────────
test-unit:
	go test ./internal/... ./provider/anthropic/... ./provider/openai/...

test-anthropic:
	go test -v ./provider/anthropic/...

test-openai:
	go test -v ./provider/openai/...

# ── Live tests (requires API keys) ───────────────────────────────────────────
# Environment variables:
#
#   CUSTOM_API_KEY     — company unified API key; works for both Anthropic and OpenAI
#                         providers when CUSTOM_BASE_URL points at a unified endpoint
#   ANTHROPIC_API_KEY   — fallback when CUSTOM_API_KEY is not set (Anthropic only)
#   OPENAI_API_KEY      — fallback when CUSTOM_API_KEY is not set (OpenAI only)
#   CUSTOM_BASE_URL      — optional base URL override, applies to any provider
#                         Accepts either an API root or a /v1-suffixed base URL
#                         e.g. https://your-company-api.example.com
#                              https://your-company-api.example.com/v1
#                         Anthropic: replaces https://api.anthropic.com (switches to Bearer auth)
#                         OpenAI:    replaces https://api.openai.com
#
test-live:
	go test -v -tags=live -timeout=60s ./...

test-live-anthropic:
	go test -v -tags=live -timeout=60s ./provider/anthropic/...

test-live-openai:
	go test -v -tags=live -timeout=60s ./provider/openai/...

# ── Build ─────────────────────────────────────────────────────────────────────
build:
	go build ./...

# ── Tidy ─────────────────────────────────────────────────────────────────────
tidy:
	go mod tidy

# ── Lint (requires golangci-lint) ────────────────────────────────────────────
lint:
	golangci-lint run ./...
