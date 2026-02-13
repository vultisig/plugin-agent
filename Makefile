.PHONY: help build build-agent build-worker test swagger clean install-deps

help:
	@echo "Available targets:"
	@echo "  build         - Build both agent and worker"
	@echo "  build-agent   - Build agent server"
	@echo "  build-worker  - Build worker"
	@echo "  test          - Run tests"
	@echo "  swagger       - Generate Swagger documentation"
	@echo "  sqlc          - Generate sqlc code"
	@echo "  clean         - Clean build artifacts"
	@echo "  install-deps  - Install development dependencies"

build: build-agent build-worker

build-agent:
	@echo "Building agent server..."
	@mkdir -p bin
	go build -o bin/agent ./cmd/agent

build-worker:
	@echo "Building worker..."
	@mkdir -p bin
	go build -o bin/worker ./cmd/worker

test:
	@echo "Running tests..."
	go test -v ./...

swagger:
	@echo "Generating Swagger documentation..."
	@command -v swag >/dev/null 2>&1 || { echo "swag not found. Run 'make install-deps' first."; exit 1; }
	swag init -g cmd/agent/server.go -o docs/swagger --parseDependency --parseInternal
	@echo "Swagger docs generated in docs/swagger/"

sqlc:
	@echo "Generating sqlc code..."
	sqlc generate

clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf docs/swagger/

install-deps:
	@echo "Installing development dependencies..."
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@echo "Dependencies installed!"

.DEFAULT_GOAL := help