BINARY_NAME=igloo_server
DSN="host=localhost port=5432 user=postgres password=password dbname=igloo sslmode=disable timezone=UTC connect_timeout=5"
REDIS_HOST="localhost"
REDIS_PORT="6379"
REDIS_PASSWORD="redisPassword"
REDIS_DB="0"
PORT=":8080"
URL_SIGN_SECRET="urlSuperSecret"
JWT_SECRET="tokenSuperSecret"
COOKIE_DOMAIN="localhost"
COOKIE_PATH="/"
COOKIE_NAME="ocs-portal"
FFMPEG_LOCATION="/bin/ffmpeg"
FFPROBE_LOCATION="/bin/ffprobe"

## build: Build binary
build:
	@echo "Building..."
	env CGO_ENABLED=0  go build -ldflags="-s -w" -o ${BINARY_NAME} ./cmd
	@echo "Built!"

## run: builds and runs the application
run: build
	@echo "Starting..."
	@env FFPROBE_LOCATION=${FFPROBE_LOCATION} FFMPEG_LOCATION=${FFMPEG_LOCATION} PORT=${PORT} COOKIE_DOMAIN=${COOKIE_DOMAIN} COOKIE_NAME=${COOKIE_NAME} COOKIE_PATH=${COOKIE_PATH} JWT_SECRET=${JWT_SECRET} URL_SIGN_SECRET=${URL_SIGN_SECRET} DSN=${DSN} REDIS_HOST=${REDIS_HOST} REDIS_PORT=${REDIS_PORT} REDIS_DB=${REDIS_DB} ./${BINARY_NAME} &
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
