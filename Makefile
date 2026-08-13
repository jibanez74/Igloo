BINARY_NAME := igloo-server

SERVER_DIR := server
WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist
WEB_EMBED_DIR := $(SERVER_DIR)/cmd/api/webdist

DIST_DIR := $(SERVER_DIR)/dist
SERVER_BINARY := dist/$(BINARY_NAME)
BINARY_PATH := $(SERVER_DIR)/$(SERVER_BINARY)
DEV_BINARY := dist/$(BINARY_NAME)-dev
DEV_BINARY_PATH := $(SERVER_DIR)/$(DEV_BINARY)
PID_FILE := $(DIST_DIR)/$(BINARY_NAME).pid
LOG_FILE := $(DIST_DIR)/$(BINARY_NAME).log

GO_TAGS := sqlite_fts5
DEV_TAGS := externalbin sqlite_fts5
PROFILE_TAGS := externalbin sqlite_fts5 pprofdebug
TEST_TAGS := externalbin sqlite_fts5
LDFLAGS := -s -w

GOOS_CURRENT := $(shell go env GOOS 2>/dev/null)
GOARCH_CURRENT := $(shell go env GOARCH 2>/dev/null)
PLATFORM := $(GOOS_CURRENT)-$(GOARCH_CURRENT)
PAYLOAD_SUFFIX := $(GOOS_CURRENT)_$(GOARCH_CURRENT)
FFMPEG_PAYLOAD := $(SERVER_DIR)/cmd/internal/ffmpeg/ffmpeg_$(PAYLOAD_SUFFIX)
FFPROBE_PAYLOAD := $(SERVER_DIR)/cmd/internal/ffprobe/ffprobe_$(PAYLOAD_SUFFIX)

.DEFAULT_GOAL := dev

.PHONY: dev dev-profile build start stop clean help check test test-server test-tmdb-integration test-web lint-web build-web test-openapi lint-openapi generate-openapi check-openapi preview-openapi
.PHONY: check-go-tools check-web-tools check-sqlc-tools check-media-tools check-dev-tools check-build-tools check-server-test-tools check-platform check-media-payloads generate prepare-webdist-placeholder prepare-test-webdist prepare-web

dev: check-dev-tools generate prepare-webdist-placeholder
	@echo "Starting development server..."
	@echo "Start the web client separately with: cd $(WEB_DIR) && bun run dev"
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go build -tags "$(DEV_TAGS)" -o $(DEV_BINARY) ./cmd/api
	@env VITE_DEV_SERVER=http://localhost:3000 "$(DEV_BINARY_PATH)"

dev-profile: check-dev-tools generate prepare-webdist-placeholder
	@echo "Starting development server with pprof at /api/debug/pprof (admin only)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go build -tags "$(PROFILE_TAGS)" -o $(DEV_BINARY) ./cmd/api
	@env VITE_DEV_SERVER=http://localhost:3000 "$(DEV_BINARY_PATH)"

build: check-build-tools check-media-payloads generate prepare-web
	@echo "Building $(BINARY_NAME) for $(PLATFORM)..."
	@mkdir -p $(DIST_DIR)
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags="$(LDFLAGS)" -o $(SERVER_BINARY) ./cmd/api
	@echo "Built $(BINARY_PATH)."

start:
	@mkdir -p $(DIST_DIR)
	@if [ -f "$(PID_FILE)" ]; then \
		pid="$$(cat "$(PID_FILE)")"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			echo "$(BINARY_NAME) is already running with PID $$pid."; \
			exit 1; \
		fi; \
		echo "Removing stale PID file."; \
		rm -f "$(PID_FILE)"; \
	fi
	@$(MAKE) --no-print-directory build
	@echo "Starting $(BINARY_NAME) in the background..."
	@nohup env VITE_DEV_SERVER= "$(BINARY_PATH)" > "$(LOG_FILE)" 2>&1 & echo $$! > "$(PID_FILE)"
	@sleep 1
	@if ! kill -0 "$$(cat "$(PID_FILE)")" 2>/dev/null; then \
		echo "$(BINARY_NAME) exited during startup. See $(LOG_FILE)."; \
		rm -f "$(PID_FILE)"; \
		exit 1; \
	fi
	@echo "Started $(BINARY_NAME) with PID $$(cat "$(PID_FILE)")."
	@echo "Logs: $(LOG_FILE)"

stop:
	@if [ ! -f "$(PID_FILE)" ]; then \
		echo "No running $(BINARY_NAME) found."; \
		exit 0; \
	fi; \
	pid="$$(cat "$(PID_FILE)")"; \
	if [ -z "$$pid" ]; then \
		echo "Removing empty PID file."; \
		rm -f "$(PID_FILE)"; \
		exit 0; \
	fi; \
	if kill -0 "$$pid" 2>/dev/null; then \
		echo "Stopping $(BINARY_NAME) with PID $$pid..."; \
		kill -TERM "$$pid"; \
		for _ in 1 2 3 4 5 6 7 8 9 10; do \
			if ! kill -0 "$$pid" 2>/dev/null; then \
				break; \
			fi; \
			sleep 1; \
		done; \
		if kill -0 "$$pid" 2>/dev/null; then \
			echo "$(BINARY_NAME) did not stop after 10 seconds."; \
			exit 1; \
		fi; \
	else \
		echo "Removing stale PID file for PID $$pid."; \
	fi; \
	rm -f "$(PID_FILE)"; \
	echo "Stopped."

clean: stop
	@echo "Cleaning build artifacts..."
	@rm -f $(SERVER_DIR)/$(BINARY_NAME) $(SERVER_DIR)/api
	@rm -rf $(DIST_DIR) $(WEB_DIST) $(WEB_EMBED_DIR) $(WEB_DIR)/.tanstack $(WEB_DIR)/src/routeTree.gen.ts
	@echo "Cleaned."

help:
	@echo "Igloo Make targets:"
	@echo "  make dev           Generate sqlc code and run the API for local development"
	@echo "  make dev-profile   Run the dev API with pprof endpoints at /api/debug/pprof"
	@echo "  make build         Build the native binary with embedded web assets and media tools"
	@echo "  make start         Build and run the full application in the background"
	@echo "  make stop          Stop the background application"
	@echo "  make clean         Remove ignored build artifacts"
	@echo "  make check         Run contract checks, backend tests, web lint, web tests, and web build"
	@echo "  make test          Run backend and web unit tests"
	@echo "  make test-server   Run backend tests with required build tags"
	@echo "  make test-tmdb-integration Run live TMDB API tests (requires TMDB_API_KEY)"
	@echo "  make test-web      Run frontend unit tests"
	@echo "  make lint-web      Run frontend lint"
	@echo "  make build-web     Build and type-check the frontend"
	@echo "  make test-openapi  Lint the OpenAPI contract and run route coverage tests"
	@echo "  make lint-openapi  Lint docs/openapi.json with Redocly"
	@echo "  make generate-openapi Generate web TypeScript schemas from OpenAPI"
	@echo "  make check-openapi Verify committed generated OpenAPI schemas are current"
	@echo "  make preview-openapi Build and serve local OpenAPI documentation on localhost"

check: test-openapi check-openapi test-server lint-web test-web build-web

test: test-server test-web

test-server: check-server-test-tools prepare-webdist-placeholder
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go test -count=1 -v -tags "$(TEST_TAGS)" ./...

test-tmdb-integration: check-go-tools
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go test -count=1 -v -tags "$(TEST_TAGS) integration" ./cmd/internal/tmdb/

test-web: check-web-tools
	@cd $(WEB_DIR) && bun run test

lint-web: check-web-tools
	@cd $(WEB_DIR) && bun run lint

build-web: check-web-tools
	@cd $(WEB_DIR) && bun run build

test-openapi: lint-openapi check-server-test-tools prepare-webdist-placeholder
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 go test -tags "$(TEST_TAGS)" ./cmd/api -run TestOpenAPIDocumentsRegisteredAPIRoutes -count=1

lint-openapi: check-web-tools
	@cd $(WEB_DIR) && bun run lint:openapi

generate-openapi: check-web-tools
	@cd $(WEB_DIR) && bun run generate:openapi

check-openapi: check-web-tools
	@cd $(WEB_DIR) && bun run check:openapi

preview-openapi: check-web-tools
	@cd $(WEB_DIR) && bun run preview:openapi

check-go-tools:
	@command -v go >/dev/null || (echo "go is required"; exit 1)
	@command -v cc >/dev/null || command -v clang >/dev/null || command -v gcc >/dev/null || (echo "a C compiler is required for CGO/sqlite builds"; exit 1)

check-web-tools:
	@command -v bun >/dev/null || (echo "bun is required"; exit 1)

check-sqlc-tools:
	@command -v sqlc >/dev/null || (echo "sqlc is required"; exit 1)

check-media-tools:
	@command -v ffmpeg >/dev/null || (echo "ffmpeg is required on PATH"; exit 1)
	@command -v ffprobe >/dev/null || (echo "ffprobe is required on PATH"; exit 1)

check-dev-tools: check-go-tools check-sqlc-tools check-media-tools

check-build-tools: check-go-tools check-sqlc-tools check-web-tools

check-server-test-tools: check-go-tools check-media-tools

check-platform:
	@case "$(PLATFORM)" in \
		linux-amd64|darwin-arm64) ;; \
		*) echo "Unsupported release platform $(PLATFORM). Supported: linux-amd64 darwin-arm64."; exit 1 ;; \
	esac

check-media-payloads: check-platform
	@test -f "$(FFMPEG_PAYLOAD)" || (echo "Missing embedded ffmpeg payload: $(FFMPEG_PAYLOAD)"; exit 1)
	@test -f "$(FFPROBE_PAYLOAD)" || (echo "Missing embedded ffprobe payload: $(FFPROBE_PAYLOAD)"; exit 1)

generate:
	@cd $(SERVER_DIR)/sqlc && sqlc generate

prepare-webdist-placeholder:
	@mkdir -p $(WEB_EMBED_DIR)
	@touch $(WEB_EMBED_DIR)/.keep

prepare-test-webdist: prepare-webdist-placeholder

prepare-web:
	@echo "Building web app..."
	@cd $(WEB_DIR) && bun run build
	@echo "Embedding web app..."
	@rm -rf $(WEB_EMBED_DIR)
	@mkdir -p $(WEB_EMBED_DIR)
	@cp -R $(WEB_DIST)/. $(WEB_EMBED_DIR)/
