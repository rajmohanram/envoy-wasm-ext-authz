.PHONY: help build build-linux run test test-coverage clean docker-build docker-run format format-check vet lint check dev deploy dev-allow-all dev-strict

help:
	@echo "Available targets:"
	@echo "  make build          - Build authz-service binary"
	@echo "  make build-linux    - Build for Linux ARM64/AMD64"
	@echo "  make run            - Run authz-service locally"
	@echo "  make test           - Run tests with race detection"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make format         - Format code with gofmt"
	@echo "  make format-check   - Check code formatting"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run linter (golangci-lint/staticcheck)"
	@echo "  make check          - Run all checks (format, vet, lint, test)"
	@echo "  make dev            - Full dev cycle (format, vet, lint, test, build)"
	@echo "  make deploy         - Full pipeline: clean, format, vet, test, build, docker-build, restart"
	@echo "  make clean          - Remove binaries and artifacts"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run in Docker container"

build:
	@echo "→ Building authz-service..."
	@mkdir -p bin
	go build -o bin/authz-service ./cmd/server
	@echo "→ Building for Linux ARM64/AMD64..."
	GOOS=linux GOARCH=arm64 go build -o bin/authz-service-arm64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build -o bin/authz-service-amd64 ./cmd/server
	@echo "✓ Build complete: bin/authz-service-arm64 and bin/authz-service-amd64"

run:
	@echo "→ Running authz-service..."
	@echo "Use Ctrl+C to stop"
	@echo ""
	go run ./cmd/server

test:
	@echo "→ Running tests with race detection..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests passed"

test-coverage: test
	@echo "→ Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"
	@echo "→ Coverage summary:"
	@go tool cover -func=coverage.out | tail -1

format:
	@echo "→ Formatting Go code..."
	go fmt ./...
	@echo "✓ Code formatted"

format-check:
	@echo "→ Checking Go code formatting..."
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "✗ Files need formatting" && exit 1)
	@echo "✓ Format check passed"

vet:
	@echo "→ Running go vet..."
	go vet ./...
	@echo "✓ Vet passed"

lint:
	@echo "→ Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	elif command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "⚠ No linter found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)"; \
		echo "→ Running go vet as fallback..."; \
		go vet ./...; \
	fi
	@echo "✓ Lint passed"

check: format-check vet lint test
	@echo "✓ All checks passed"

dev: format vet lint test build
	@echo "✓ Development build complete"

docker-build:
	@echo "→ Building Docker image..."
	docker build -t authz-service .
	@echo "✓ Docker image built: authz-service"

docker-run:
	@echo "→ Running in Docker..."
	docker run -p 443:443 -p 8443:8443 \
		-e LOG_LEVEL=debug \
		-e ENABLE_METRICS=true \
		authz-service

deploy: clean format vet test build docker-build
	@echo "→ Restarting authz-service via docker-compose..."
	cd .. && docker-compose stop authz-service
	cd .. && docker-compose rm -f authz-service
	cd .. && docker-compose up -d authz-service
	@echo "✓ Authz service restarted"
	@sleep 2
	@echo "→ Checking authz-service status..."
	cd .. && docker-compose ps authz-service
	@echo "→ Viewing recent logs..."
	cd .. && docker-compose logs --tail=10 authz-service

clean:
	@echo "→ Cleaning up..."
	rm -rf bin coverage.out coverage.html
	go clean
	@echo "✓ Cleanup complete"

# Development targets
dev-allow-all:
	@echo "→ Running in ALLOW-ALL mode (for testing)..."
	LOG_LEVEL=debug ENABLE_METRICS=true go run ./cmd/server

dev-strict:
	@echo "→ Running in STRICT mode..."
	LOG_LEVEL=info ENABLE_METRICS=true go run ./cmd/server
