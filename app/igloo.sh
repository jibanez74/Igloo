#!/bin/bash

# Igloo Server Management Script
# Usage: ./igloo.sh {start|stop|restart|status}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="igloo-server"
BINARY_PATH="${SCRIPT_DIR}/${BINARY_NAME}"
PID_FILE="${SCRIPT_DIR}/.igloo.pid"
LOG_FILE="${SCRIPT_DIR}/logs/igloo.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Ensure logs directory exists
mkdir -p "${SCRIPT_DIR}/logs"

# Function to check if sqlc generated files exist
check_sqlc_files() {
    if [ ! -d "${SCRIPT_DIR}/cmd/internal/database" ]; then
        echo -e "${RED}Error: sqlc generated files not found at cmd/internal/database${NC}"
        echo "The generated database files must exist for the build to succeed."
        echo "Please ensure sqlc generated files are committed to git or pre-generated."
        exit 1
    fi
    
    # Check for at least one generated file
    if [ -z "$(ls -A ${SCRIPT_DIR}/cmd/internal/database/*.go 2>/dev/null)" ]; then
        echo -e "${RED}Error: No generated .go files found in cmd/internal/database${NC}"
        echo "Please ensure sqlc generated files are present."
        exit 1
    fi
}

# Function to sync schema (copy without running sqlc)
sync_schema() {
    if [ ! -f "${SCRIPT_DIR}/sqlc/schema.sql" ]; then
        echo -e "${RED}Error: sqlc/schema.sql not found${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Syncing schema...${NC}"
    cp "${SCRIPT_DIR}/sqlc/schema.sql" "${SCRIPT_DIR}/cmd/api/schema.sql"
}

# Function to build the application (without sqlc generate)
build_app() {
    echo -e "${GREEN}Building Igloo application (without sqlc generate)...${NC}"
    
    cd "${SCRIPT_DIR}"
    
    # Check if sqlc generated files exist
    check_sqlc_files
    
    # Sync schema
    sync_schema
    
    # Build frontend
    echo -e "${GREEN}Building frontend...${NC}"
    if [ ! -f "${SCRIPT_DIR}/cmd/web/package.json" ]; then
        echo -e "${RED}Error: cmd/web/package.json not found${NC}"
        exit 1
    fi
    
    
    # Build Go binary
    echo -e "${GREEN}Building Go binary...${NC}"
    cd "${SCRIPT_DIR}"
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: go is not installed or not in PATH${NC}"
        exit 1
    fi
    
    export CGO_ENABLED=1
    go build -ldflags="-s -w" -o "${BINARY_PATH}" ./cmd/api
    if [ $? -ne 0 ]; then
        echo -e "${RED}Go build failed${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Build completed successfully!${NC}"
}

# Function to check if binary exists
check_binary() {
    if [ ! -f "${BINARY_PATH}" ]; then
        echo -e "${YELLOW}Binary not found. Building application...${NC}"
        build_app
        if [ $? -ne 0 ]; then
            exit 1
        fi
    fi
}

# Function to check if server is running
is_running() {
    if [ -f "${PID_FILE}" ]; then
        PID=$(cat "${PID_FILE}")
        if ps -p "${PID}" > /dev/null 2>&1; then
            return 0
        else
            # PID file exists but process is dead, clean it up
            rm -f "${PID_FILE}"
            return 1
        fi
    fi
    return 1
}

# Function to start the server
start_server() {
    check_binary
    
    if is_running; then
        PID=$(cat "${PID_FILE}")
        echo -e "${YELLOW}Server is already running (PID: ${PID})${NC}"
        return 1
    fi
    
    echo -e "${GREEN}Starting Igloo server...${NC}"
    
    # Check if .env file exists
    if [ ! -f "${SCRIPT_DIR}/.env" ]; then
        echo -e "${YELLOW}Warning: .env file not found. Server may not start correctly.${NC}"
    fi
    
    # Start the server in the background
    cd "${SCRIPT_DIR}"
    nohup "${BINARY_PATH}" > "${LOG_FILE}" 2>&1 &
    PID=$!
    
    # Save PID to file
    echo "${PID}" > "${PID_FILE}"
    
    # Wait a moment to check if it started successfully
    sleep 1
    
    if is_running; then
        echo -e "${GREEN}Server started successfully (PID: ${PID})${NC}"
        echo "Logs are being written to: ${LOG_FILE}"
        echo "To view logs in real-time: tail -f ${LOG_FILE}"
        return 0
    else
        echo -e "${RED}Server failed to start. Check logs: ${LOG_FILE}${NC}"
        rm -f "${PID_FILE}"
        return 1
    fi
}

# Function to stop the server
stop_server() {
    if ! is_running; then
        echo -e "${YELLOW}Server is not running${NC}"
        return 1
    fi
    
    PID=$(cat "${PID_FILE}")
    echo -e "${YELLOW}Stopping Igloo server (PID: ${PID})...${NC}"
    
    # Try graceful shutdown first
    kill "${PID}" 2>/dev/null
    
    # Wait up to 10 seconds for graceful shutdown
    for i in {1..10}; do
        if ! ps -p "${PID}" > /dev/null 2>&1; then
            echo -e "${GREEN}Server stopped gracefully${NC}"
            rm -f "${PID_FILE}"
            return 0
        fi
        sleep 1
    done
    
    # Force kill if still running
    if ps -p "${PID}" > /dev/null 2>&1; then
        echo -e "${YELLOW}Server did not stop gracefully, forcing shutdown...${NC}"
        kill -9 "${PID}" 2>/dev/null
        sleep 1
        
        if ! ps -p "${PID}" > /dev/null 2>&1; then
            echo -e "${GREEN}Server stopped${NC}"
            rm -f "${PID_FILE}"
            return 0
        else
            echo -e "${RED}Failed to stop server${NC}"
            return 1
        fi
    fi
    
    rm -f "${PID_FILE}"
    return 0
}

# Function to restart the server
restart_server() {
    stop_server
    sleep 2
    start_server
}

# Function to show server status
show_status() {
    if is_running; then
        PID=$(cat "${PID_FILE}")
        echo -e "${GREEN}Server is running (PID: ${PID})${NC}"
        
        # Show process info
        ps -p "${PID}" -o pid,ppid,user,%cpu,%mem,etime,command 2>/dev/null || true
        
        # Show last few log lines
        if [ -f "${LOG_FILE}" ]; then
            echo ""
            echo "Last 5 log lines:"
            tail -n 5 "${LOG_FILE}" 2>/dev/null || echo "No log entries yet"
        fi
        return 0
    else
        echo -e "${RED}Server is not running${NC}"
        return 1
    fi
}

# Main command handler
case "${1}" in
    start)
        # Build if needed, then start
        check_binary
        start_server
        ;;
    stop)
        stop_server
        ;;
    restart)
        stop_server
        sleep 2
        check_binary
        start_server
        ;;
    status)
        show_status
        ;;
    build)
        build_app
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|build}"
        echo ""
        echo "Commands:"
        echo "  start   - Build (if needed) and start the Igloo server"
        echo "  stop    - Stop the Igloo server"
        echo "  restart - Restart the Igloo server (rebuilds if needed)"
        echo "  status  - Show server status and recent logs"
        echo "  build   - Build the application without starting"
        exit 1
        ;;
esac

exit $?
