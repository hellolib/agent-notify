<div align="center">

# Agent Notify

<p align="center"><b>Notifies you when your agent needs you</b>

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/hellolib/agent-notify.svg)](https://github.com/hellolib/agent-notify/releases)

<p align="center"><b>English</b> | <a href="README.zh-CN.md">简体中文</a></p>

</div>

## Overview

Agent Notify hooks into the lifecycle events of AI coding agents (Claude Code, Codex, ZCode, Grok, etc.) and pushes them to your phone and desktop. Get notified the moment your agent needs permission, is waiting for input, finishes a task, or fails — so you never have to babysit a running agent.

Supported delivery channels: **OS-native system notifications**, **Feishu/Lark**, **WeChat Work (企业微信)**, **DingTalk (钉钉)**, **Bark (iOS)**, and **ntfy**.

<p align="center">
  <img src="assist/demo.gif" alt="Agent Notify demo" width="700">
</p>

## Quick Start

```bash
npx agent-notify
```



## Features

### Supported Channels

| Channel | Description | Setup   |
|:--------|------|---------|
| 🖥️ System Notification | Native notifications on macOS, Linux, and Windows | Default |
| <img src="assist/logo/feishu.png" width="24" align="absmiddle"> Feishu / Lark | One-click QR-code binding; push via Feishu bot messages | QR scan |
| <img src="assist/logo/qiyeweixin.png" width="24" align="absmiddle"> WeChat Work | Push notifications via a WeChat Work group bot webhook | Webhook |
| <img src="assist/logo/dingding.png" width="24" align="absmiddle"> DingTalk | Push notifications via a DingTalk group bot webhook | Webhook |
| <img src="assist/logo/bark.png" width="24" align="absmiddle"> Bark | Push to iOS devices via a Bark webhook URL | Webhook |
| <img src="assist/logo/ntfy.png" width="24" align="absmiddle"> ntfy | Push via ntfy.sh or self-hosted ntfy server | Topic |
| <img src="assist/logo/slack.png" width="24" align="absmiddle"> Slack | Push via Slack Incoming Webhook | Webhook |
| <img src="assist/logo/discord.png" width="24" align="absmiddle"> Discord | Push via Discord channel webhook | 🚧 Webhook |
| <img src="assist/logo/telegram.png" width="24" align="absmiddle"> Telegram | Push via Telegram Bot API | 🚧 Bot token |

### Supported Events

| Event | Description | Claude Code | Codex | ZCode | Grok |
|------|------|:---:|:---:|:---:|:----:|
| `permission_required` | Agent needs authorization (e.g. to run a command) | ✅ | ✅ | ✅ |  ✅  |
| `input_required` | Agent is waiting for user input | ✅ | — | — |  ✅  |
| `run_completed` | Task finished | ✅ | ✅ | ✅ |  ✅  |
| `run_failed` | Task failed | ✅ | — | ✅ |  ✅  |

Notes:

- Claude Code subscribes via hooks in `~/.claude/settings.json`: `PermissionRequest`, `Notification`, `Stop`, `PostToolUseFailure`, and `SessionStart`.
- Codex subscribes via `~/.codex/hooks.json`: `PermissionRequest` and `Stop` (mapped to `permission_required` / `run_completed`), plus `SessionStart`. `input_required` and `run_failed` have no corresponding Codex hook yet, so they are not supported.
- ZCode subscribes via `~/.zcode/cli/config.json`: `SessionStart`, `PermissionRequest`, `PostToolUseFailure`, and `Stop`, mapped to `permission_required`, `run_failed`, and `run_completed`. ZCode has no `Notification` event (so no `input_required`), and its hook schema is strict — an unknown event name will cause the whole hooks config to be silently dropped.
- Grok subscribes via `~/.grok/hooks/agent-notify.json`: `SessionStart`, `Notification`, `Stop`, `StopFailure`, and `PostToolUseFailure`. There is no dedicated `PermissionRequest` event; `Notification`s with permission/approval semantics map to `permission_required` (marked *), others map to `input_required`. `StopFailure` / `PostToolUseFailure` map to `run_failed`.
- **`SessionStart` does not produce a notification.** It is subscribed on every agent solely to capture the terminal window at session start, which powers Linux window-level [Click-to-Focus](#click-to-focus). On macOS/Windows the SessionStart hook is a no-op.

### Supported Platforms

| Platform | Architecture | Status |
|:---:|:---:|:---:|
| macOS | amd64 / arm64 | ✅ |
| Linux | amd64 / arm64 | ✅ |
| Windows | amd64 / arm64 | ✅ |

### Click-to-Focus

System notifications are clickable — clicking one brings you back to the terminal / window where the agent is running. Behavior differs by platform:

- **macOS** — App-level by default (activates the agent's terminal/IDE app). For window-level focus (return to the exact window even when several are open), set `AGENT_NOTIFY_FOCUS_PRECISION=window` in your login shell environment (e.g. `~/.zshrc`); this uses a bundled helper and requires Accessibility permission. Unset stays app-level.
- **Linux (X11)** — Window-level. The exact terminal window is captured at session start (via the `SessionStart` hook) and re-focused on click, so it distinguishes sibling windows of single-process terminals (deepin-terminal, GNOME Terminal, etc.). Native Wayland windows can't be targeted.
- **Windows** — Returns to the terminal window via a bundled helper.

> **`AGENT_NOTIFY_FOCUS_PRECISION`** accepts `window` (window-level) or `app` (app-level — the default). Values are case-insensitive and whitespace-trimmed; anything unset or unrecognized falls back to `app`. This variable **only affects macOS** — Linux is always window-level, and Windows uses its own helper.

Click-to-focus is enabled by default for the System channel; the target app/window is detected automatically from the hook's environment and process tree.




## Configuration

On first run, the launcher downloads the platform-specific binary matching the current npm package version from GitHub Releases and installs it to:

- macOS / Linux: `~/.agent-notify/agent-notify`
- Windows: `~/.agent-notify/agent-notify.exe`

On every subsequent run it compares the local binary against the version pinned by the npm package and reinstalls whenever they differ — in both directions, so `npx agent-notify@0.13.0` really does run 0.13.0. The launcher never persistently modifies `PATH` — it always executes via an absolute path.

### Behind a proxy

The downloader honours `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` and `NO_PROXY` (lowercase forms take precedence, as usual). When none of those are set it falls back to npm's own proxy settings from `.npmrc`, which is how most corporate setups are configured:

```bash
export HTTPS_PROXY=http://user:pass@proxy.corp:8080
export NO_PROXY=github.com                          # or exclude GitHub from the proxy

npm config set https-proxy http://proxy.corp:8080   # read as a fallback
```

Download failures name the proxy that was used and what to try next, rather than stalling until the timeout.

> **Note**: Codex integrates through the official hooks system in `~/.codex/hooks.json` and currently subscribes only to `PermissionRequest` and `Stop`. After first install, run `/hooks` inside Codex to complete the trust review.
>
> **Grok**: Writes `~/.grok/hooks/agent-notify.json`. Global hooks are always trusted; project hooks (`.grok/hooks/`) require `/hooks-trust` or `--trust`. After install, run `/hooks` (or `Ctrl+L`) inside Grok to confirm they loaded.


> You don't need to edit config files by hand — this section is for reference only.

Agent Notify's own config lives at `~/.agent-notify/config.yaml`. **New installs start with all agents and channels disabled** — run `npx agent-notify` (setup wizard) once to enable the agents and channels you want. This avoids showing unconfigured agents as ready in view/doctor after a partial setup. Existing config files are left unchanged.

Agent integration config locations:

- Claude Code: `~/.claude/settings.json` (writes hooks → command `agent-notify handle-claude-hook`)
- Codex: `~/.codex/hooks.json` (writes hooks → command `agent-notify handle-codex-hook`; run `/hooks` inside Codex to complete trust)
- ZCode: `~/.zcode/cli/config.json` (writes `hooks.events.<Event>` + `hooks.enabled` → command `agent-notify handle-zcode-hook`; restart ZCode for the config to take effect)
- Grok: `~/.grok/hooks/agent-notify.json` (writes hooks → command `agent-notify handle-grok-hook`; project scope uses `.grok/hooks/agent-notify.json`)

### WeChat Work Bot Binding Tip

1. **Create a single-person notification group**: start a group chat in WeChat Work (pull in a few colleagues). After it's created, **do not post anything**, then remove the others — the group becomes your personal notification channel.
2. **Add a bot**: "Group Settings" → "Message Push" → "Add" → "Custom Message Push", name it and save.
3. **Get the webhook URL**: copy the generated URL, which looks like `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`.
4. **Bind it**: run `npx agent-notify`, enable the WeChat Work channel in the setup wizard, and paste the webhook URL.
> Older WeChat Work versions: "Group Settings" → "Group Bots" → "Add Bot" → "New Bot", name it and save.


<p align="center">
  <img src="assist/workflow.png" alt="Workflow diagram" />
</p>

## Screenshots

| | |
|:---:|:---:|
| <img src="assist/launch-setting.png" alt="Setup" width="75%"> | <img src="assist/feishu-bind.png" alt="Feishu binding" width="75%"> |
| **Setup** | **Feishu Binding** |
| <img src="assist/feishu-notify-phone.png" alt="Feishu notification" width="55%"> | <img src="assist/wecom-notify.jpg" alt="WeChat Work notification" width="55%"> |
| **Feishu Notification** | **WeChat Work Notification** |
| <img src="assist/system-notify.png" alt="System notification" width="55%"> | |
| **System Notification** | |


## Acknowledgments

Thanks for the support and feedback from the friends at [LINUX DO](https://linux.do/).
