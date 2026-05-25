BINARY_NAME := igloo-server

SERVER_DIR := server
WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist
WEB_EMBED_DIR := $(SERVER_DIR)/cmd/api/webdist

DIST_DIR := $(SERVER_DIR)/dist
SERVER_BINARY := dist/$(BINARY_NAME)
BINARY_PATH := $(SERVER_DIR)/$(SERVER_BINARY)
PID_FILE := $(DIST_DIR)/$(BINARY_NAME).pid
LOG_FILE := $(DIST_DIR)/$(BINARY_NAME).log

GO_TAGS := sqlite_fts5
DEV_TAGS := externalbin sqlite_fts5
LDFLAGS := -s -w

GOOS_CURRENT := $(shell go env GOOS 2>/dev/null)
GOARCH_CURRENT := $(shell go env GOARCH 2>/dev/null)
PLATFORM := $(GOOS_CURRENT)-$(GOARCH_CURRENT)
PAYLOAD_SUFFIX := $(GOOS_CURRENT)_$(GOARCH_CURRENT)
FFMPEG_PAYLOAD := $(SERVER_DIR)/cmd/internal/ffmpeg/ffmpeg_$(PAYLOAD_SUFFIX)
FFPROBE_PAYLOAD := $(SERVER_DIR)/cmd/internal/ffprobe/ffprobe_$(PAYLOAD_SUFFIX)

.PHONY: dev build start stop clean
.PHONY: check-dev-tools check-build-tools check-platform check-media-payloads sync-schema generate prepare-test-webdist prepare-web

dev: check-dev-tools generate prepare-test-webdist
	@echo "Starting development server..."
	@echo "Start the web client separately with: cd $(WEB_DIR) && bun run dev"
	@cd $(SERVER_DIR) && env CGO_ENABLED=1 VITE_DEV_SERVER=http://localhost:3000 go run -tags "$(DEV_TAGS)" ./cmd/api

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

check-dev-tools:
	@command -v go >/dev/null || (echo "go is required"; exit 1)
	@command -v sqlc >/dev/null || (echo "sqlc is required"; exit 1)
	@command -v ffmpeg >/dev/null || (echo "ffmpeg is required on PATH for make dev"; exit 1)
	@command -v ffprobe >/dev/null || (echo "ffprobe is required on PATH for make dev"; exit 1)

check-build-tools:
	@command -v go >/dev/null || (echo "go is required"; exit 1)
	@command -v sqlc >/dev/null || (echo "sqlc is required"; exit 1)
	@command -v bun >/dev/null || (echo "bun is required"; exit 1)

check-platform:
	@case "$(PLATFORM)" in \
		linux-amd64|darwin-arm64) ;; \
		*) echo "Unsupported release platform $(PLATFORM). Supported: linux-amd64 darwin-arm64."; exit 1 ;; \
	esac

check-media-payloads: check-platform
	@test -f "$(FFMPEG_PAYLOAD)" || (echo "Missing embedded ffmpeg payload: $(FFMPEG_PAYLOAD)"; exit 1)
	@test -f "$(FFPROBE_PAYLOAD)" || (echo "Missing embedded ffprobe payload: $(FFPROBE_PAYLOAD)"; exit 1)

sync-schema:
	@cp $(SERVER_DIR)/sqlc/schema.sql $(SERVER_DIR)/cmd/api/schema.sql

generate: sync-schema
	@cd $(SERVER_DIR)/sqlc && sqlc generate

prepare-test-webdist:
	@mkdir -p $(WEB_EMBED_DIR)
	@touch $(WEB_EMBED_DIR)/.keep

prepare-web:
	@echo "Building web app..."
	@cd $(WEB_DIR) && bun run build
	@echo "Embedding web app..."
	@rm -rf $(WEB_EMBED_DIR)
	@mkdir -p $(WEB_EMBED_DIR)
	@cp -R $(WEB_DIST)/. $(WEB_EMBED_DIR)/
