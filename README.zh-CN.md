<div align="center">

<img src="assist/brand/bell-only-128.png" alt="Agent Notify" width="90">

# Agent Notify

<p align="center"><b>在 Agent 需要你时通知你</b>

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/hellolib/agent-notify.svg)](https://github.com/hellolib/agent-notify/releases)

<p align="center"><a href="README.md">English</a> | <b>简体中文</b></p>

</div>

## 项目简介

一个面向 AI Agent 的通知配置工具。支持将 Claude Code、Codex、OpenCode、ZCode (Z.ai)、Grok、Droid 等 Agent 的事件通知推送到飞书、企业微信、钉钉、Bark、ntfy 和系统通知。

<p align="center">
  <img src="assist/demo.gif" alt="Agent Notify 演示" width="800">
</p>

## 快速开始

```bash
npx agent-notify
```


## 功能特性
### 支持的通知渠道
|   通知渠道   | 说明 | 绑定方式   |
|:--------|------|--------|
| 🖥️ 系统通知 | 支持 macOS、Linux、Windows 系统通知 | 原生支持   |
| <img src="assist/logo/feishu.png" width="24" align="absmiddle"> 飞书   | 支持一键扫码绑定、支持飞书机器人消息推送 | 二维码扫描  |
| <img src="assist/logo/qiyeweixin.png" width="24" align="absmiddle"> 企业微信  | 支持通过企业微信群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/dingding.png" width="24" align="absmiddle"> 钉钉  | 支持通过钉钉群机器人 Webhook 推送通知消息 | Webhook |
| <img src="assist/logo/bark.png" width="24" align="absmiddle"> Bark  | 支持通过 Bark Webhook URL 推送到 iOS 设备 | Webhook |
| <img src="assist/logo/ntfy.png" width="24" align="absmiddle"> ntfy  | 通过 ntfy.sh 或自托管 ntfy 服务推送 | Topic |
| <img src="assist/logo/slack.png" width="24" align="absmiddle"> Slack | 通过 Slack Incoming Webhook 推送 | Webhook |
| <img src="assist/logo/discord.png" width="24" align="absmiddle"> Discord | 通过 Discord 频道 Webhook 推送 | 🚧 Webhook |
| <img src="assist/logo/telegram.png" width="24" align="absmiddle"> Telegram | 通过 Telegram Bot API 推送 | 🚧 Bot token |

### 支持的事件

| 事件 | Claude Code | Codex | OpenCode | ZCode | Grok | Droid |
|------|:---:|:---:|:---:|:---:|:----:|:---:|
| `permission_required` | ✅ | ✅ | ✅ | ✅ |  ✅  | ✅ |
| `input_required` | ✅ | — | ✅ | — |  ✅  | ✅ |
| `run_completed` | ✅ | ✅ | ✅ | ✅ |  ✅  | ✅ |
| `run_failed` | ✅ | — | ✅ | ✅ |  ✅  | — |

说明：

- Claude Code 通过 `~/.claude/settings.json` 的 hooks 订阅：`PermissionRequest`、`Notification`、`Stop`、`PostToolUseFailure`、`SessionStart`。
- Codex 通过 `~/.codex/hooks.json` 订阅 `PermissionRequest`、`Stop`（映射到 `permission_required` / `run_completed`）以及 `SessionStart`。同时会在 `~/.codex/config.toml` 安装托管的顶层 `notify` 命令，为 VS Code 等可能不执行 hook 或无法完成 trust 流程的 IDE/app-server 客户端提供 `run_completed` 兜底；已有且与 agent-notify 无关的自定义 `notify` 命令会保留。`input_required` 与 `run_failed` Codex 目前没有对应 hook，因此暂不支持。
- OpenCode 使用 JS 插件而非原生 hooks：插件写入 `~/.agent-notify/opencode-plugin.js`（二进制路径烘焙进 JS），路径注册到 `~/.config/opencode/opencode.json`（user）或 `./opencode.json`（project）的 `plugin` 数组。插件订阅 `session.created`→`session_start`、`permission.asked`→`permission_required`、`session.status`(idle)→`input_required`、`session.idle`→`run_completed`、`session.error`→`run_failed`。
- ZCode 通过 `~/.zcode/cli/config.json` 订阅 `SessionStart`、`PermissionRequest`、`PostToolUseFailure`、`Stop`，映射到 `permission_required`、`run_failed`、`run_completed`。ZCode 没有 `Notification` 事件（因此不支持 `input_required`），且其 hook 配置格式较为严格——无法识别的事件名称会导致整个 hooks 配置被静默丢弃。
- Grok 通过 `~/.grok/hooks/agent-notify.json` 订阅 `SessionStart`、`Notification`、`Stop`、`StopFailure`、`PostToolUseFailure`。Grok 没有独立的 `PermissionRequest` 事件，带 permission/approval 语义的 `Notification` 会映射为 `permission_required`（表中 *）；其它通知映射为 `input_required`。`StopFailure` / `PostToolUseFailure` 映射为 `run_failed`。
- Droid 通过 `~/.factory/hooks.json` 订阅 `SessionStart`、`Notification`、`Stop`，映射为 `session_start` / `permission_required`|`input_required` / `run_completed`。Droid 无失败事件，故不支持 `run_failed`。`session_start` 仅用于点击聚焦的窗口捕获，不作为通知事件。
- **`SessionStart` 不产生任何通知。** 它在所有 agent 上被订阅，仅用于捕获会话来源的终端/IDE 窗口，为窗口级点击聚焦提供支持（见下方「点击聚焦」一节）。

### 支持的平台

| 平台 | 架构 | 状态 |
|:---:|:---:|:---:|
| macOS | amd64 / arm64 | ✅ |
| Linux | amd64 / arm64 | ✅ |
| Windows | amd64 / arm64 | ✅ |

### 点击聚焦（Click-to-Focus）

系统通知可点击——点击后会跳回运行 agent 的终端 / 窗口。各平台行为不同：

- **macOS** — 默认应用级（激活 agent 所在的终端/IDE 应用）。若要窗口级（多窗口时也精确跳回那一个），在登录 shell 环境（如 `~/.zshrc`）里设置 `AGENT_NOTIFY_FOCUS_PRECISION=window`；这会用到内置 helper 并需要「辅助功能」权限。不设置则保持应用级。
- **Linux（X11）** — 窗口级。在会话启动时（通过 `SessionStart` hook）捕获精确的终端窗口，点击时跳回，因此能区分单进程多窗口终端（deepin-terminal、GNOME Terminal 等）的兄弟窗口。原生 Wayland 窗口无法定位。
- **Windows** — 通过内置 helper 跳回来源终端/IDE 窗口。优先使用 `SessionStart` 捕获；Codex IDE 通知没有捕获缓存时，会用工作区路径唯一匹配祖先进程的窗口标题，因此可以区分共用同一 `Code.exe` 进程的多个 VS Code 窗口。点击通知不会取消已最大化窗口的最大化状态；最小化窗口会先恢复。

> **`AGENT_NOTIFY_FOCUS_PRECISION`** 接受 `window`（窗口级）或 `app`（应用级，默认值）。取值不区分大小写、会去除首尾空白；未设置或无法识别的值都回退为 `app`。该变量**仅对 macOS 生效**——Linux 始终是窗口级，Windows 用自己的 helper。

系统渠道默认开启点击聚焦；目标应用/窗口会从 hook 环境、会话捕获、工作区与进程树自动识别。

## 安装说明

```bash
npx agent-notify
```

首次运行会从 GitHub Releases 下载当前 npm 包版本对应平台的二进制文件，并安装到：

- macOS / Linux: `~/.agent-notify/agent-notify`
- Windows: `~/.agent-notify/agent-notify.exe`

之后每次运行都会检查本地二进制版本：不存在则自动下载，版本落后则自动更新，否则直接运行。launcher 不会持久修改 PATH，始终用绝对路径执行。

> **注意**: Codex 通过 `~/.codex/hooks.json` 的官方 hooks 处理 `PermissionRequest`、`Stop` 与会话窗口捕获。首次安装后请在 Codex CLI 内运行 `/hooks` 完成 trust 审核。`~/.codex/config.toml` 中的托管 `notify` 兜底可让 VS Code 等 IDE/app-server 客户端无需该 trust 步骤也能收到完成通知。
>
> **Grok**: 写入 `~/.grok/hooks/agent-notify.json`。全局 hooks 始终可信；项目级 hooks（`.grok/hooks/`）需在仓库内运行 `/hooks-trust` 或使用 `--trust`。安装后可在 Grok 中运行 `/hooks`（或 `Ctrl+L`）确认已加载。


## 配置说明

> agent-notify 不需要手动处理配置文件，该章节仅是为了说明配置相关信息。

agent-notify 自身配置位于 `~/.agent-notify/config.yaml`。**新安装默认关闭所有 Agent 与通知渠道**——需运行一次 `npx agent-notify`（配置向导）启用你需要的 Agent 与渠道。这样可避免只配置了一个 Agent 后，在「查看配置 / 诊断」里把未配置的 Agent 显示为已就绪。已有配置文件不受影响。

Agent 集成配置位置：

- Claude Code: `~/.claude/settings.json`（写入 hooks → 命令 `agent-notify handle-claude-hook`）
- Codex: `~/.codex/hooks.json`（hooks → `agent-notify handle-codex-hook`）与 `~/.codex/config.toml`（完成通知兜底 → `agent-notify handle-codex-notify`；Codex CLI 内需运行 `/hooks` 信任 hooks）
- OpenCode: `~/.config/opencode/opencode.json`（写入 `plugin` 数组 → `~/.agent-notify/opencode-plugin.js`，命令 `agent-notify handle-opencode-hook`；项目 scope 为 `./opencode.json`）
- ZCode: `~/.zcode/cli/config.json`（写入 `hooks.events.<Event>` + `hooks.enabled` → 命令 `agent-notify handle-zcode-hook`；重启 ZCode 使配置生效）
- Grok: `~/.grok/hooks/agent-notify.json`（写入 hooks → 命令 `agent-notify handle-grok-hook`；项目 scope 为 `.grok/hooks/agent-notify.json`）
- Droid: `~/.factory/hooks.json`（写入 hooks → 命令 `agent-notify handle-droid-hook`；项目 scope 为 `.factory/hooks.json`）

### 企业微信机器人绑定小技巧

1. **创建单人通知群**：在企业微信中发起群聊（随便拉几个同事），创建成功后**不要在群里发言**，直接将其他人移出，此时该群将变成你的单人通知群；
2. **添加机器人**：「群设置」->「消息推送」->「添加」-> 「自定义消息推送」，命名并保存；
3. **获取 Webhook 地址**：复制生成的地址，格式类似 `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`；
4. **绑定配置**：运行 `npx agent-notify`，在配置向导中选择启用企业微信渠道，粘贴 Webhook URL 即可；
> 旧版企业微信添加机器人步骤：「群设置」->「群机器人」->「添加机器人」-> 「新建机器人」，命名并保存

## 工作流程

<p align="center">
  <img src="assist/workflow.png" alt="工作流程图" />
</p>

## 效果图

| | |
|:---:|:---:|
| <img src="assist/launch-setting.png" alt="软件配置" width="75%"> | <img src="assist/feishu-bind.png" alt="飞书绑定" width="75%"> |
| **软件配置** | **飞书绑定** |
| <img src="assist/feishu-notify-phone.png" alt="飞书通知" width="55%"> | <img src="assist/wecom-notify.jpg" alt="企业微信通知" width="55%"> |
| **飞书通知** | **企业微信通知** |
| <img src="assist/system-notify.png" alt="系统通知" width="55%"> | |
| **系统通知** | |

## 致谢

感谢 [LINUX DO](https://linux.do/) 社区朋友们的支持与反馈。
