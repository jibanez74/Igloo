BINARY_NAME=igloo-server

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

## build: Build binary for current platform (generates sqlc code and syncs schema first)
build: generate
	@echo "Building for current platform..."
	env CGO_ENABLED=1 go build -ldflags="-s -w" -o ${BINARY_NAME} ./cmd/api
	@echo "Built!"

## build-mac: Build binary for macOS ARM64
build-mac: generate
	@echo "Building for macOS ARM64..."
	env CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ${BINARY_NAME}-darwin-arm64 ./cmd/api
	@echo "Built ${BINARY_NAME}-darwin-arm64!"

## build-linux: Build binary for Linux AMD64
build-linux: generate
	@echo "Building for Linux AMD64..."
	env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ${BINARY_NAME}-linux-amd64 ./cmd/api
	@echo "Built ${BINARY_NAME}-linux-amd64!"

## build-all: Build binaries for all supported platforms
build-all: build-mac build-linux
	@echo "All platform builds complete!"

## run: builds and runs the application
run: build
	@echo "Starting..."
	@env ./${BINARY_NAME} &
	@echo "Started!"

## clean: runs go clean and deletes binaries
clean:
	@echo "Cleaning..."
	@go clean
	@rm ${BINARY_NAME}
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