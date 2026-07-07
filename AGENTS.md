# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Project Overview

agent-notify is a notification tool for AI coding agents (Codex, OpenAI Codex, ZCode). It hooks into agent lifecycle events (permission requests, input needed, task completed/failed) and sends push notifications through multiple channels: Feishu/Lark, WeChat Work, DingTalk, Bark, and OS-native system notifications.

## Common Commands

```bash
make build          # Build binary to bin/agent-notify
make test           # Run all Go tests (go test -v ./...)
make test-coverage  # Generate coverage.html
make lint           # golangci-lint (must be installed)
make fmt            # gofmt -s -w .
make vet            # go vet ./...
make run            # go run ./cmd/agent-notify
make install        # go install to GOPATH/bin
make doctor         # Build + run diagnostics
make build-all      # Cross-compile for all platforms (darwin/linux/windows x amd64/arm64)
make release VERSION=vX.Y.Z  # Git tag + npm publish
```

Single test: `go test -v ./internal/notify/...` (or any package path)

Version is injected via ldflags at build time into `internal/cli.Version`.

## Architecture

### Core Data Flow

1. **Hook trigger** — Agent invokes `agent-notify handle-Codex-hook` or `handle-codex-hook` (configured in `~/.Codex/settings.json` or `~/.codex/hooks.json`)
2. **Event parsing** — `internal/claudehooks/` or `internal/codexhooks/` reads JSON from stdin, normalizes into `notify.Message`
3. **Dispatch** — `internal/agenthooks/dispatch.go` loads config, builds enabled senders, applies deduplication
4. **Send** — `internal/notify/dispatcher.go` iterates senders, each implementing the `Sender` interface

### Key Packages

- `cmd/agent-notify/` — Entry point
- `internal/cli/` — Cobra CLI commands, interactive menu (survey), version info
- `internal/config/` — YAML config at `~/.agent-notify/config.yaml`
- `internal/notify/` — Sender implementations: feishu, wechatwork, dingtalk, bark, system (macOS/Linux/Windows)
- `internal/state/` — Deduplication store (`state.json`) + append-only log
- `internal/agentintegrations/` — `Integration` interface for Codex/Codex/ZCode (detect, install/uninstall hooks)
- `internal/app/` — High-level services: setup wizard, notification tester, doctor diagnostics
- `npx/` — Node.js launcher that downloads/caches the Go binary from GitHub Releases

### Key Interfaces

```go
// internal/notify/ — all senders implement this
type Sender interface {
    Name() string
    Send(ctx context.Context, msg Message) error
}

// internal/agentintegrations/ — Codex/Codex integrations implement this
type Integration interface {
    Name() string
    DetectInstalled() bool
    SettingsPath() string
    Install() error
    Uninstall() error
    IsHookInstalled() bool
}
```

### Supported Events

`session_start`, `permission_required`, `input_required`, `run_completed`, `run_failed` — Codex supports all four standard events; Codex supports `permission_required` and `run_completed`; ZCode supports `session_start`, `permission_required`, `run_completed`, and `run_failed` (no `input_required` — ZCode has no `Notification` event).

ZCode integration notes: config lives at `~/.zcode/cli/config.json`, hooks are nested under `hooks.events.<Event>` (not `hooks.<Event>` like Codex) and require `hooks.enabled: true`. The ZCode hook schema is strict — an unrecognized event name silently invalidates the whole config. stdin payload uses both `hook_event_name` (snake_case) and `hookEventName` (camelCase); the parser accepts either.

### Configuration & State

- Config: `~/.agent-notify/config.yaml` — per-agent event lists, per-channel webhook/enable settings
- State: `~/.agent-notify/state.json` — dedup timestamps
- Logs: `~/.agent-notify/agent-notify.log` — append-only

## Release

Push a `v*` tag to trigger the GitHub Actions release workflow, which cross-compiles for 6 platform/arch combos, UPX-compresses (except macOS arm64 and Windows arm64), and creates a GitHub Release. `make release VERSION=vX.Y.Z` handles tagging and npm publish.

## NPX Launcher

The `npx/` directory is a separate Node.js (>=18) package. It auto-downloads the correct Go binary from GitHub Releases, caches at `~/.agent-notify/agent-notify`, and auto-updates when the npm version is newer. Tests: `cd npx && npm test`.

<!-- headroom:rtk-instructions -->
# RTK (Rust Token Killer) - Token-Optimized Commands

When running shell commands, **always prefix with `rtk`**. This reduces context
usage by 60-90% with zero behavior change. If rtk has no filter for a command,
it passes through unchanged — so it is always safe to use.

## Key Commands
```bash
# Git (59-80% savings)
rtk git status          rtk git diff            rtk git log

# Files & Search (60-75% savings)
rtk ls <path>           rtk read <file>         rtk grep <pattern>
rtk find <pattern>      rtk diff <file>

# Test (90-99% savings) — shows failures only
rtk pytest tests/       rtk cargo test          rtk test <cmd>

# Build & Lint (80-90% savings) — shows errors only
rtk tsc                 rtk lint                rtk cargo build
rtk prettier --check    rtk mypy                rtk ruff check

# Analysis (70-90% savings)
rtk err <cmd>           rtk log <file>          rtk json <file>
rtk summary <cmd>       rtk deps                rtk env

# GitHub (26-87% savings)
rtk gh pr view <n>      rtk gh run list         rtk gh issue list

# Infrastructure (85% savings)
rtk docker ps           rtk kubectl get         rtk docker logs <c>

# Package managers (70-90% savings)
rtk pip list            rtk pnpm install        rtk npm run <script>
```

## Rules
- In command chains, prefix each segment: `rtk git add . && rtk git commit -m "msg"`
- For debugging, use raw command without rtk prefix
- `rtk proxy <cmd>` runs command without filtering but tracks usage
<!-- /headroom:rtk-instructions -->
