# X Spam Detector Makefile

# Build variables
BINARY_NAME=spam-detector
AUTONOMOUS_BINARY=spam-detector-autonomous
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
BUILD_DIR=build
DIST_DIR=dist
COVERAGE_DIR=coverage

# Default target
.PHONY: all
all: clean deps test build build-autonomous

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -rf $(COVERAGE_DIR)

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
.PHONY: test
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Build for current platform
.PHONY: build
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build autonomous version
.PHONY: build-autonomous
build-autonomous:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(AUTONOMOUS_BINARY) cmd/autonomous/main.go

# Build for all platforms
.PHONY: build-all
build-all: clean
	mkdir -p $(DIST_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	
	# Create checksums
	cd $(DIST_DIR) && sha256sum * > checksums.txt

# Run the application
.PHONY: run
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run autonomous mode
.PHONY: run-autonomous
run-autonomous: build-autonomous
	./$(BUILD_DIR)/$(AUTONOMOUS_BINARY)

# Run with sample config
.PHONY: run-dev
run-dev: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml

# Run autonomous with debug
.PHONY: run-autonomous-debug
run-autonomous-debug: build-autonomous
	./$(BUILD_DIR)/$(AUTONOMOUS_BINARY) -log-level debug

# Install to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install $(LDFLAGS) .

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Check for security issues
.PHONY: security
security:
	gosec ./...

# Run all quality checks
.PHONY: quality
quality: fmt lint test security

# Generate documentation
.PHONY: docs
docs:
	godoc -http=:6060

# Create release archive
.PHONY: release
release: build-all
	mkdir -p $(DIST_DIR)/release
	
	# Create archives for each platform
	cd $(DIST_DIR) && \
	tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 && \
	tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 && \
	tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 && \
	tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 && \
	zip release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	
	# Copy additional files
	cp README.md $(DIST_DIR)/release/
	cp config.yaml $(DIST_DIR)/release/
	cp checksums.txt $(DIST_DIR)/release/

# Docker build
.PHONY: docker-build
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

# Docker run
.PHONY: docker-run
docker-run:
	docker run -it --rm $(BINARY_NAME):latest

# Performance profiling
.PHONY: profile-cpu
profile-cpu: build
	./$(BUILD_DIR)/$(BINARY_NAME) -cpuprofile=cpu.prof
	$(GOCMD) tool pprof cpu.prof

.PHONY: profile-mem
profile-mem: build
	./$(BUILD_DIR)/$(BINARY_NAME) -memprofile=mem.prof
	$(GOCMD) tool pprof mem.prof

# Development helpers
.PHONY: dev-setup
dev-setup:
	$(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOCMD) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Watch for changes and rebuild
.PHONY: watch
watch:
	which fswatch > /dev/null || (echo "fswatch not found. Install with: brew install fswatch" && exit 1)
	fswatch -o . | xargs -n1 -I{} make build

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  all              - Clean, deps, test, and build both versions"
	@echo "  build            - Build GUI version"
	@echo "  build-autonomous - Build autonomous version"
	@echo "  build-all        - Build for all platforms"
	@echo ""
	@echo "Run targets:"
	@echo "  run                    - Run GUI version"
	@echo "  run-autonomous         - Run autonomous version"
	@echo "  run-autonomous-debug   - Run autonomous with debug logging"
	@echo "  run-dev                - Run GUI with development config"
	@echo ""
	@echo "Development:"
	@echo "  clean        - Remove build artifacts"
	@echo "  deps         - Download dependencies"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  bench        - Run benchmarks"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  security     - Check for security issues"
	@echo "  quality      - Run all quality checks"
	@echo ""
	@echo "Deployment:"
	@echo "  install      - Install to GOPATH/bin"
	@echo "  release      - Create release archives"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo ""
	@echo "Utilities:"
	@echo "  docs         - Generate documentation"
	@echo "  profile-cpu  - Run CPU profiling"
	@echo "  profile-mem  - Run memory profiling"
	@echo "  dev-setup    - Install development tools"
	@echo "  watch        - Watch for changes and rebuild"
	@echo "  help         - Show this help message"