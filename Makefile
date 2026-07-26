.PHONY: build test run clean install lint fmt vet help tag npm-publish release check-release-version

# Binary name
BINARY_NAME=agent-notify

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Version info
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X github.com/hellolib/agent-notify/internal/cli.Version=$(VERSION)"

all: clean test build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

## build-all: Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/$(BINARY_NAME)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/$(BINARY_NAME)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/$(BINARY_NAME)
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/$(BINARY_NAME)

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## run: Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	$(GOCMD) run ./cmd/$(BINARY_NAME)

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	$(GOCLEAN)

## install: Install the binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install ./cmd/$(BINARY_NAME)

## lint: Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, please install it" && exit 1)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "Tidy go modules..."
	$(GOMOD) tidy

## mod-download: Download go modules
mod-download:
	@echo "Downloading go modules..."
	$(GOMOD) download

## doctor: Run doctor command
doctor: build
	@echo "Running doctor..."
	./$(BUILD_DIR)/$(BINARY_NAME) doctor

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

# Release parameters
NPX_DIR=npx

# VERSION 顶部有 `?=` 默认值(git describe),所以 `ifndef VERSION` 永远不成立——
# 不传 VERSION 时它会静默变成 "v0.14.3-15-gabc1234-dirty" 之类,照样打 tag、
# 推远端、触发 release workflow,发出一个垃圾 release。因此改为校验格式:
# 只接受 vX.Y.Z(可带 -rc1 之类后缀),git describe 的产物一律拒绝。
check-release-version:
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$$' || { \
		echo "Error: VERSION must look like v1.2.3 (got: '$(VERSION)')."; \
		echo "       Usage: make $(MAKECMDGOALS) VERSION=v0.1.0"; \
		exit 1; \
	}

## tag: Create and push a git tag (usage: make tag VERSION=v0.1.0)
tag: check-release-version
	@echo "Creating tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "Tag $(VERSION) created and pushed to remote"

## npm-publish: Manually publish npm package (fallback; CI normally does this)
npm-publish: check-release-version
	@echo "Publishing to npm..."
	@echo "NOTE: release.yml publishes automatically after the GitHub Release is"
	@echo "      uploaded. Only run this by hand if that job failed."
	@NPM_VERSION=$$(echo $(VERSION) | sed 's/^v//'); \
	cd $(NPX_DIR) && npm version $$NPM_VERSION --no-git-tag-version --allow-same-version && npm publish --access public
	@git checkout $(NPX_DIR)/package.json $(NPX_DIR)/package-lock.json 2>/dev/null || true
	@echo "Published $(VERSION) to npm"

## release: Push the release tag; CI builds, publishes the GitHub Release, then npm (usage: make release VERSION=v0.1.0)
release: check-release-version
	@echo "Starting release $(VERSION)..."
	$(MAKE) tag VERSION=$(VERSION)
	@echo ""
	@echo "Tag pushed. release.yml now:"
	@echo "  1. builds the 6-target matrix"
	@echo "  2. publishes the GitHub Release + SHA256SUMS"
	@echo "  3. publishes to npm (only after step 2 succeeds)"
	@echo ""
	@echo "Watch it with: gh run watch"
	@echo "npm publish is intentionally NOT run from here: doing so raced the"
	@echo "release build and left first-time npx users downloading a 404."
