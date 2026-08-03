.PHONY: build test run dev dev-serve dev-test-hook dev-approve dev-reject dev-approval-on dev-approval-off clean install lint fmt vet help tag npm-publish release

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

## dev: Run with local feishu DNS-fix proxy if present (for dev on polluted-DNS machines)
dev:
	@echo "Running $(BINARY_NAME) (dev, with feishu-env if available)..."
	@if [ -f "$$HOME/.agent-notify/feishu-env.sh" ]; then \
		bash -lc 'set --; source "$$HOME/.agent-notify/feishu-env.sh" >/dev/null 2>&1; exec $(GOCMD) run ./cmd/$(BINARY_NAME)'; \
	else \
		$(GOCMD) run ./cmd/$(BINARY_NAME); \
	fi

# ---- 远程审批本地测试 ----

## dev-serve: 启动飞书 WebSocket 长连接审批服务（需飞书代理）
dev-serve:
	@echo "Starting callback server (dev)..."
	@if [ -f "$$HOME/.agent-notify/feishu-env.sh" ]; then \
		bash -lc 'set --; source "$$HOME/.agent-notify/feishu-env.sh" >/dev/null 2>&1; exec $(GOCMD) run ./cmd/$(BINARY_NAME) serve --listen 127.0.0.1:7896'; \
	else \
		$(GOCMD) run ./cmd/$(BINARY_NAME) serve --listen 127.0.0.1:7896; \
	fi

## dev-test-hook: 模拟 Codex PermissionRequest hook（阻塞等待远程审批）
dev-test-hook:
	@echo "Simulating Codex PermissionRequest hook (blocks until approved/rejected/timeout)..."
	@payload='{"hook_event_name":"PermissionRequest","session_id":"test-dev","cwd":"/tmp","tool_name":"Bash","permission_mode":"default","tool_input":{"command":"git status"}}'; \
	if [ -f "$$HOME/.agent-notify/feishu-env.sh" ]; then \
		echo "$$payload" | bash -lc 'set --; source "$$HOME/.agent-notify/feishu-env.sh" >/dev/null 2>&1; exec $(GOCMD) run ./cmd/$(BINARY_NAME) handle-codex-hook'; \
	else \
		echo "$$payload" | $(GOCMD) run ./cmd/$(BINARY_NAME) handle-codex-hook; \
	fi

## dev-approve: 自动审批最新的 pending 请求（模拟飞书点"允许"）
dev-approve:
	@rid=$$(ls -t $$HOME/.agent-notify/pending-requests/*.json 2>/dev/null | head -1); \
	if [ -z "$$rid" ]; then echo "没有 pending 请求，请先在另一个终端运行 make dev-test-hook"; exit 1; fi; \
	rid=$$(basename "$$rid" .json); \
	echo "审批: $$rid (allow)"; \
	curl -s -X POST http://127.0.0.1:7896/feishu/callback \
		-H 'Content-Type: application/json' \
		-d "{\"action\":{\"value\":{\"action\":\"allow\",\"request_id\":\"$$rid\"}}}"

## dev-reject: 自动拒绝最新的 pending 请求（模拟飞书点"拒绝"）
dev-reject:
	@rid=$$(ls -t $$HOME/.agent-notify/pending-requests/*.json 2>/dev/null | head -1); \
	if [ -z "$$rid" ]; then echo "没有 pending 请求，请先在另一个终端运行 make dev-test-hook"; exit 1; fi; \
	rid=$$(basename "$$rid" .json); \
	echo "拒绝: $$rid (reject)"; \
	curl -s -X POST http://127.0.0.1:7896/feishu/callback \
		-H 'Content-Type: application/json' \
		-d "{\"action\":{\"value\":{\"action\":\"reject\",\"request_id\":\"$$rid\"}}}"

## dev-approval-on: 启用远程审批（修改 ~/.agent-notify/config.yaml）
dev-approval-on:
	@cfg="$$HOME/.agent-notify/config.yaml"; \
	if grep -qE '^remote_approval:' "$$cfg"; then \
		sed -i -E '/^remote_approval:/{n;s/enabled:.*/enabled: true/}' "$$cfg"; \
	else \
		printf '\nremote_approval:\n  enabled: true\n  wait_seconds: 30\n' >> "$$cfg"; \
	fi
	@echo "远程审批已启用 (~/.agent-notify/config.yaml)"

## dev-approval-off: 关闭远程审批
dev-approval-off:
	@cfg="$$HOME/.agent-notify/config.yaml"; \
	if grep -qE '^remote_approval:' "$$cfg"; then \
		sed -i -E '/^remote_approval:/{n;s/enabled:.*/enabled: false/}' "$$cfg"; \
	else \
		printf '\nremote_approval:\n  enabled: false\n  wait_seconds: 30\n' >> "$$cfg"; \
	fi
	@echo "远程审批已关闭 (~/.agent-notify/config.yaml)"

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

## tag: Create and push a git tag (usage: make tag VERSION=v0.1.0)
tag:
ifndef VERSION
	@echo "Error: VERSION is required. Usage: make tag VERSION=v0.1.0"
	@exit 1
endif
	@echo "Creating tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "Tag $(VERSION) created and pushed to remote"

## npm-publish: Publish npm package (usage: make npm-publish VERSION=v0.1.0)
npm-publish:
ifndef VERSION
	@echo "Error: VERSION is required. Usage: make npm-publish VERSION=v0.1.0"
	@exit 1
endif
	@echo "Publishing to npm..."
	@NPM_VERSION=$$(echo $(VERSION) | sed 's/^v//'); \
	cd $(NPX_DIR) && npm version $$NPM_VERSION --no-git-tag-version --allow-same-version && npm publish --access public
	@git checkout $(NPX_DIR)/package.json $(NPX_DIR)/package-lock.json 2>/dev/null || true
	@echo "Published $(VERSION) to npm"

## release: Create tag and publish to npm (usage: make release VERSION=v0.1.0)
release:
ifndef VERSION
	@echo "Error: VERSION is required. Usage: make release VERSION=v0.1.0"
	@exit 1
endif
	@echo "Starting release $(VERSION)..."
	$(MAKE) tag VERSION=$(VERSION)
	$(MAKE) npm-publish VERSION=$(VERSION)
	@echo "Release $(VERSION) completed!"
