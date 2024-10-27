BINARY_NAME="igloo_server"
MODE="dev"
DSN="host=localhost port=5432 user=postgres password=password dbname=igloo sslmode=disable timezone=UTC connect_timeout=5"
REDIS_HOST="127.0.0.1"
REDIS_PORT=6379
PORT=":8080"
COOKIE_NAME="igloo_auth"
COOKIE_DOMAIN="https://swifty.hare-crocodile.ts.net"
JELLYFIN_TOKEN="820260cfd6ad4e929d45c3427122f836"
TMDB_API_KEY="cc32c7ce101a2a0967f83f392fffb873"


## build: Build binary
build:
	@echo "Building..."
	env CGO_ENABLED=0  go build -ldflags="-s -w" -o ${BINARY_NAME} ./cmd
	@echo "Built!"

## run: builds and runs the application
run: build
	@echo "Starting..."
	@env TMDB_API_KEY=${TMDB_API_KEY} COOKIE_DOMAIN=${COOKIE_DOMAIN} COOKIE_NAME=${COOKIE_NAME} PORT=${PORT} MODE=${MODE} DSN=${DSN} REDIS_HOST=${REDI_HOSTS} ./${BINARY_NAME} &
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
