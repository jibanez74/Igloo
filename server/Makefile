BINARY_NAME=igloo-server
WEB_DIR=../web
WEB_DIST=$(WEB_DIR)/dist
WEB_EMBED_DIR=cmd/api/webdist
DIST_DIR=dist
GO_TAGS=sqlite_fts5
DEV_TEST_TAGS=externalbin sqlite_fts5
LDFLAGS=-s -w
GOOS_CURRENT=$(shell go env GOOS)
GOARCH_CURRENT=$(shell go env GOARCH)
PAYLOAD_SUFFIX=$(GOOS_CURRENT)_$(GOARCH_CURRENT)
PLATFORM=$(GOOS_CURRENT)-$(GOARCH_CURRENT)
PACKAGE_ROOT=$(BINARY_NAME)-$(PLATFORM)
PACKAGE_DIR=$(DIST_DIR)/$(PACKAGE_ROOT)
PACKAGE_ARCHIVE=$(DIST_DIR)/$(PACKAGE_ROOT).tar.gz
FFMPEG_PAYLOAD=cmd/internal/ffmpeg/ffmpeg_$(PAYLOAD_SUFFIX)
FFPROBE_PAYLOAD=cmd/internal/ffprobe/ffprobe_$(PAYLOAD_SUFFIX)
SUPPORTED_PLATFORMS=linux-amd64 darwin-arm64

.PHONY: sync-schema generate build-web prepare-web prepare-test-webdist check-package-platform check-media-payloads check-external-bins dev build package package-linux-amd64 package-darwin-arm64 run clean start stop restart test test-ci

## sync-schema: Copy schema from sqlc to cmd/api for embedding.
sync-schema:
	@echo "Syncing schema..."
	@cp sqlc/schema.sql cmd/api/schema.sql
	@echo "Schema synced."

## generate: Run sqlc generate and sync schema.
generate: sync-schema
	@echo "Generating sqlc code..."
	@cd sqlc && sqlc generate
	@echo "Generated."

## build-web: Build the web frontend.
build-web:
	@echo "Building web frontend..."
	@cd $(WEB_DIR) && bun run build
	@echo "Web frontend built to $(WEB_DIST)."

## prepare-web: Copy the frontend bundle into the Go embed directory.
prepare-web: build-web
	@echo "Copying frontend bundle to $(WEB_EMBED_DIR)..."
	@rm -rf $(WEB_EMBED_DIR)
	@mkdir -p $(WEB_EMBED_DIR)
	@cp -R $(WEB_DIST)/. $(WEB_EMBED_DIR)/
	@echo "Frontend bundle ready for embedding."

## prepare-test-webdist: Create the embed directory needed for backend tests.
prepare-test-webdist:
	@mkdir -p $(WEB_EMBED_DIR)
	@touch $(WEB_EMBED_DIR)/.keep

check-package-platform:
	@if ! printf '%s\n' $(SUPPORTED_PLATFORMS) | grep -qx '$(PLATFORM)'; then \
		echo "Unsupported release platform $(PLATFORM). Supported: $(SUPPORTED_PLATFORMS)."; \
		exit 1; \
	fi

check-media-payloads: check-package-platform
	@test -f "$(FFMPEG_PAYLOAD)" || (echo "Missing embedded ffmpeg payload: server/$(FFMPEG_PAYLOAD)"; exit 1)
	@test -f "$(FFPROBE_PAYLOAD)" || (echo "Missing embedded ffprobe payload: server/$(FFPROBE_PAYLOAD)"; exit 1)

check-external-bins:
	@command -v ffmpeg >/dev/null || (echo "ffmpeg is required on PATH for externalbin tests/dev"; exit 1)
	@command -v ffprobe >/dev/null || (echo "ffprobe is required on PATH for externalbin tests/dev"; exit 1)

## dev: Run Go server in development mode. Run "cd ../web && bun run dev" separately.
dev: generate prepare-test-webdist check-external-bins
	@echo "Starting development server (backend only)..."
	@echo "Note: Run frontend with 'cd $(WEB_DIR) && bun run dev' (Vite on port 3000)."
	@env CGO_ENABLED=1 VITE_DEV_SERVER=http://localhost:3000 go run -tags "$(DEV_TEST_TAGS)" ./cmd/api

## build: Build a complete binary for the current supported platform.
build: generate prepare-web check-media-payloads
	@echo "Building $(BINARY_NAME) for $(PLATFORM) with embedded web assets and media tools..."
	@env CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/api
	@echo "Built server/$(BINARY_NAME)."

## package: Create dist/$(PACKAGE_ARCHIVE) for the current supported platform.
package: build
	@echo "Packaging $(PACKAGE_ARCHIVE)..."
	@rm -rf $(PACKAGE_DIR)
	@mkdir -p $(PACKAGE_DIR)
	@cp $(BINARY_NAME) $(PACKAGE_DIR)/
	@cp ../.env.example $(PACKAGE_DIR)/.env.example
	@cp release/README.md $(PACKAGE_DIR)/README.md
	@cd $(DIST_DIR) && tar -czf $(PACKAGE_ROOT).tar.gz $(PACKAGE_ROOT)
	@echo "Packaged server/$(PACKAGE_ARCHIVE)."

package-linux-amd64:
	@test "$(PLATFORM)" = "linux-amd64" || (echo "package-linux-amd64 must run on Linux AMD64; current platform is $(PLATFORM)."; exit 1)
	@$(MAKE) package

package-darwin-arm64:
	@test "$(PLATFORM)" = "darwin-arm64" || (echo "package-darwin-arm64 must run on macOS ARM64; current platform is $(PLATFORM)."; exit 1)
	@$(MAKE) package

## run: Build and run the complete binary in the foreground.
run: build
	@echo "Starting $(BINARY_NAME)..."
	@./$(BINARY_NAME)

## clean: Remove generated binaries, packages, and frontend build output.
clean:
	@echo "Cleaning..."
	@go clean
	@rm -f $(BINARY_NAME)
	@rm -rf $(DIST_DIR) $(WEB_DIST) $(WEB_EMBED_DIR)
	@echo "Cleaned."

start: run

stop:
	@echo "Stopping..."
	@-pkill -SIGTERM -f "./$(BINARY_NAME)"
	@echo "Stopped."

restart: stop start

## test: Run backend tests using host ffmpeg/ffprobe.
test: prepare-test-webdist check-external-bins
	go test -tags "$(DEV_TEST_TAGS)" -v ./...

## test-ci: Run the deterministic backend suite used by GitHub Actions.
test-ci: prepare-test-webdist check-external-bins
	go test -count=1 -v -tags "$(DEV_TEST_TAGS)" ./...
