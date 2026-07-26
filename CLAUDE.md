# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Agent Notify hooks into lifecycle events of AI coding agents (Claude Code, Codex, ZCode, Grok) and pushes notifications to desktop and phone (system notifications, Feishu, WeChat Work, DingTalk, Bark, ntfy, Slack). Distributed as a Go binary via an npx launcher (`npx agent-notify`) that downloads the platform binary from GitHub Releases into `~/.agent-notify/`.

## Commands

```bash
make build            # Build to bin/agent-notify (ldflags inject internal/cli.Version)
make test             # go test -v ./...
go test ./internal/notify -run TestName   # Run a single test
make lint             # golangci-lint run ./...
make fmt              # gofmt -s -w .
make vet              # go vet ./...

cd npx && npm test    # Node launcher tests (node --test)

make release VERSION=v0.x.y   # tag + push (triggers GH Actions release) + npm publish
```

The release workflow (`.github/workflows/release.yml`) builds a 6-target matrix on tag push and bundles the prebuilt helpers from `thirdparty/helper/` into the platform archives (macOS: terminal-notifier.app + mac-focus-helper; Windows: toast-focus-helper.exe).

## Architecture

The binary has two personalities:
1. **Interactive TUI** (`agent-notify` / `init`, `doctor`, `test`) — setup wizard that writes `~/.agent-notify/config.yaml` and installs hooks into each agent's config file.
2. **Hook handlers** (`handle-claude-hook`, `handle-codex-hook`, `handle-zcode-hook`, `handle-grok-hook`) — invoked by the agents themselves; read a hook event JSON from stdin, normalize it, and fan out notifications. These run headless and must never block or prompt.

Event flow: agent fires hook → `internal/cli/handler_*.go` → `internal/<agent>hooks/event.go` parses stdin JSON and maps agent-specific events to normalized events (`permission_required`, `input_required`, `run_completed`, `run_failed`, `session_start`) → `internal/agenthooks.Dispatch` → `internal/notify.Dispatcher.SendAll` fans out to enabled channel `Sender`s with dedupe.

Key packages:

- `internal/cli` — cobra commands, setup wizard menus/prompts (uses survey/v2).
- `internal/claudehooks`, `codexhooks`, `zcodehooks`, `grokhooks` — one package per agent, each with the same shape: `event.go` (stdin JSON → normalized `notify.Message`), `settings.go` (read/write hook registration in that agent's config file: `~/.claude/settings.json`, `~/.codex/hooks.json`, `~/.zcode/cli/config.json`, `~/.grok/hooks/agent-notify.json`), `handler.go` (glue). Adding an agent means adding a package with this shape plus entries in `internal/agentintegrations` and `internal/agenthooks/dispatch.go:buildSenders`.
- `internal/agenthooks` — `Dispatch`: detects source app from inherited env, resolves workspace, handles the `session_start` side-effect event (focus-window capture, never notifies), builds senders from per-agent config, dedupes, sends.
- `internal/notify` — one file per channel implementing `Sender`; `dispatcher.go` dedupes per (agent, session, event, content-hash, sender) via `internal/state`. System notifications are per-OS (`macos.go`, `linux.go`, `windows.go` + build-tagged variants).
- `internal/config` — `~/.agent-notify/config.yaml` schema. New installs start with all agents/channels disabled until the wizard enables them.
- `internal/state` — file-backed dedupe reservations, focus-window cache (keyed by session ID), append-only log.
- `internal/linuxfocus`, `internal/winfocus`, `internal/notify/macos.go` — click-to-focus: clicking a system notification refocuses the originating terminal/IDE window. Linux uses X11/EWMH window IDs; macOS/Windows use bundled helper binaries and window snapshots captured at `session_start`. macOS precision is controlled by `AGENT_NOTIFY_FOCUS_PRECISION` (`app` default / `window`).
- `internal/i18n` — TUI localization (zh-CN default, en-US); notification text formatting lives separately in `internal/notify/format.go` and is not affected by TUI locale.
- `npx/` — Node launcher (CommonJS, no runtime deps beyond `tar`): version-checks and downloads the release binary, then execs it by absolute path. `npm run prepack` syncs the repo README into the package.

## Conventions

- Code comments are predominantly Chinese; match the surrounding file's style.
- ZCode's hook schema is strict — an unknown event name silently drops the entire hooks config, so be careful editing `internal/zcodehooks/settings.go`.
- Hook handlers log failures to the state log file and return nil rather than erroring loudly (an agent's hook must not break the agent).
- `docs/` is gitignored (local design archives) — don't reference it from committed code or docs.
