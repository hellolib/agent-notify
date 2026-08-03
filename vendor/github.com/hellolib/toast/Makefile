GO ?= go
DIST_DIR ?= dist
WINDOWS_GUI_LDFLAGS ?= -H windowsgui

.PHONY: all build build-windows build-windows-amd64 build-windows-arm64 build-helpers build-helper-windows-amd64 build-helper-windows-arm64 test clean help

all: build

build: build-windows

build-windows: build-windows-amd64 build-windows-arm64

build-helpers: build-helper-windows-amd64 build-helper-windows-arm64

build-windows-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -o $(DIST_DIR)/toast-focus-amd64.exe ./cmd/toast-focus
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(WINDOWS_GUI_LDFLAGS)" -o $(DIST_DIR)/toast-focus-helper-amd64.exe ./cmd/toast-focus-helper

build-windows-arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=arm64 $(GO) build -o $(DIST_DIR)/toast-focus-arm64.exe ./cmd/toast-focus
	GOOS=windows GOARCH=arm64 $(GO) build -ldflags="$(WINDOWS_GUI_LDFLAGS)" -o $(DIST_DIR)/toast-focus-helper-arm64.exe ./cmd/toast-focus-helper

build-helper-windows-amd64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(WINDOWS_GUI_LDFLAGS)" -o $(DIST_DIR)/toast-focus-helper-amd64.exe ./cmd/toast-focus-helper

build-helper-windows-arm64:
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=arm64 $(GO) build -ldflags="$(WINDOWS_GUI_LDFLAGS)" -o $(DIST_DIR)/toast-focus-helper-arm64.exe ./cmd/toast-focus-helper

test:
	$(GO) test ./...

clean:
	rm -rf $(DIST_DIR)

help:
	@echo "Targets:"
	@echo "  make build              Build all Windows demo/helper binaries into $(DIST_DIR)/"
	@echo "  make build-windows      Build all Windows demo/helper binaries"
	@echo "  make build-helpers      Build only Windows focus helper binaries"
	@echo "  make build-windows-amd64"
	@echo "  make build-windows-arm64"
	@echo "  make test               Run Go tests"
	@echo "  make clean              Remove $(DIST_DIR)/"
