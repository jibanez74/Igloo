BINARY_NAME=igloo-server
WEB_DIR=cmd/web
WEB_DIST=web/dist

## sync-schema: Copy schema from sqlc to cmd/api for embedding
sync-schema:
	@echo "Syncing schema..."
	@cp sqlc/schema.sql cmd/api/schema.sql
	@echo "Schema synced!"

## generate: Run sqlc generate and sync schema
generate: sync-schema
	@echo "Generating sqlc code..."
	@cd sqlc && sqlc generate
	@echo "Generated!"

## build-web: Build the web frontend
build-web:
	@echo "Building web frontend..."
	@cd ${WEB_DIR} && bun run build
	@echo "Web frontend built to ${WEB_DIST}!"

## dev: Run Go server in development mode (without building web)
dev: generate
	@echo "Starting development server (backend only)..."
	@echo "Note: Frontend should be run separately with 'cd ${WEB_DIR} && bun run dev'"
	@env CGO_ENABLED=1 go run ./cmd/api

## build: Build binary for current platform (generates sqlc code and syncs schema first)
## Does not build web frontend - use build-web separately if needed
build: generate
	@echo "Building backend for current platform..."
	@env CGO_ENABLED=1 go build -ldflags="-s -w" -o ${BINARY_NAME} ./cmd/api
	@echo "Backend built!"

## build-full: Build both backend and frontend
build-full: generate build-web
	@echo "Building full application..."
	@env CGO_ENABLED=1 go build -ldflags="-s -w" -o ${BINARY_NAME} ./cmd/api
	@echo "Full application built!"

## build-mac: Build binary for macOS ARM64 (backend only)
build-mac: generate
	@echo "Building for macOS ARM64..."
	@env CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-arm64 ./cmd/api
	@echo "Built ${BINARY_NAME}-darwin-arm64!"

## build-linux: Build binary for Linux AMD64 (backend only)
build-linux: generate
	@echo "Building for Linux AMD64..."
	@env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ${BINARY_NAME}-linux-amd64 ./cmd/api
	@echo "Built ${BINARY_NAME}-linux-amd64!"

## run: builds and runs the application (backend only, no web build)
run: build
	@echo "Starting..."
	@env ./${BINARY_NAME} &
	@echo "Started!"

## clean: runs go clean and deletes binaries and web dist
clean:
	@echo "Cleaning..."
	@go clean
	@rm -f ${BINARY_NAME} ${BINARY_NAME}-darwin-arm64 ${BINARY_NAME}-linux-amd64
	@rm -rf ${WEB_DIST}
	@echo "Cleaned!"

## start: an alias to run
start: run

## stop: stops the running application
stop:
	@echo "Stopping..."
	@-pkill -SIGTERM -f "./${BINARY_NAME}"
	@echo "Stopped!"

## restart: stops and starts the application
restart: stop start

## test: runs all tests
test:
	go test -v ./...