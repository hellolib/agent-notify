<div align="center">

# Agent Notify

<p align="center"><b>别再盯着 AI Agent 跑了</b><br/>Claude Code 需要你、或跑完时,手机立刻叮一声。</p>

[![Go Version](https://img.shields.io/badge/Go-%3E%3D1.25-blue.svg)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/hellolib/agent-notify.svg)](https://github.com/hellolib/agent-notify/releases)

<p align="center"><a href="README.md">English</a> | <b>简体中文</b></p>

</div>

## 项目简介

一个面向 AI Agent 的通知配置工具。支持将 Claude Code、Codex、ZCode (Z.ai)、Grok 等 Agent 的事件通知推送到飞书、企业微信、钉钉、Bark、ntfy 和系统通知。

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

| 事件 | 说明 | Claude Code | Codex | ZCode | Grok |
|------|------|:---:|:---:|:---:|:---:|
| `session_start` | Agent 新会话开始 | — | — | ✅ | ✅ |
| `permission_required` | Agent 需要授权（如执行命令） | ✅ | ✅ | ✅ | ✅* |
| `input_required` | Agent 等待用户输入 | ✅ | — | — | ✅ |
| `run_completed` | 任务执行完成 | ✅ | ✅ | ✅ | ✅ |
| `run_failed` | 任务执行失败 | ✅ | — | ✅ | ✅ |

说明：

- Claude Code 通过 `~/.claude/settings.json` 的 hooks 订阅四个事件（`PermissionRequest`、`Notification`、`Stop`、`PostToolUseFailure`）。
- Codex 通过 `~/.codex/hooks.json` 订阅 `PermissionRequest` 与 `Stop`，分别映射到 `permission_required` 与 `run_completed`。`input_required` 与 `run_failed` Codex 目前没有对应 hook，因此暂不支持。
- ZCode 通过 `~/.zcode/cli/config.json` 订阅 `SessionStart`、`PermissionRequest`、`PostToolUseFailure` 和 `Stop`，分别映射到 `session_start`、`permission_required`、`run_failed` 和 `run_completed`。ZCode 没有 `Notification` 事件（因此不支持 `input_required`），且其 hook 配置格式较为严格——无法识别的事件名称会导致整个 hooks 配置被静默丢弃。
- Grok 通过 `~/.grok/hooks/agent-notify.json` 订阅 `SessionStart`、`Notification`、`Stop`、`StopFailure`、`PostToolUseFailure`。Grok 没有独立的 `PermissionRequest` 事件，带 permission/approval 语义的 `Notification` 会映射为 `permission_required`（表中 *）；其它通知映射为 `input_required`。`StopFailure` / `PostToolUseFailure` 映射为 `run_failed`。

### 支持的平台

| 平台 | 架构 | 状态 |
|:---:|:---:|:---:|
| macOS | amd64 / arm64 | ✅ |
| Linux | amd64 / arm64 | ✅ |
| Windows | amd64 | ✅ |

## 快速开始

```bash
npx agent-notify
```

首次运行会从 GitHub Releases 下载当前 npm 包版本对应平台的二进制文件，并安装到：

- macOS / Linux: `~/.agent-notify/agent-notify`
- Windows: `~/.agent-notify/agent-notify.exe`

之后每次运行都会检查本地二进制版本：不存在则自动下载，版本落后则自动更新，否则直接运行。launcher 不会持久修改 PATH，始终用绝对路径执行。

> **注意**: Codex 通过 `~/.codex/hooks.json` 接入官方 hooks 系统，目前仅订阅 `PermissionRequest`、`Stop` 两个事件。首次安装后请在 codex 内运行 `/hooks` 完成 trust 审核。
>
> **Grok**: 写入 `~/.grok/hooks/agent-notify.json`。全局 hooks 始终可信；项目级 hooks（`.grok/hooks/`）需在仓库内运行 `/hooks-trust` 或使用 `--trust`。安装后可在 Grok 中运行 `/hooks`（或 `Ctrl+L`）确认已加载。


## 配置说明

> agent-notify 不需要手动处理配置文件，该章节仅是为了说明配置相关信息。

agent-notify 自身配置位于 `~/.agent-notify/config.yaml`。**新安装默认关闭所有 Agent 与通知渠道**——需运行一次 `npx agent-notify`（配置向导）启用你需要的 Agent 与渠道。这样可避免只配置了一个 Agent 后，在「查看配置 / 诊断」里把未配置的 Agent 显示为已就绪。已有配置文件不受影响。

Agent 集成配置位置：

- Claude Code: `~/.claude/settings.json`（写入 hooks → 命令 `agent-notify handle-claude-hook`）
- Codex: `~/.codex/hooks.json`（写入 hooks → 命令 `agent-notify handle-codex-hook`，需在 codex 内运行 `/hooks` 完成 trust）
- ZCode: `~/.zcode/cli/config.json`（写入 `hooks.events.<Event>` + `hooks.enabled` → 命令 `agent-notify handle-zcode-hook`；重启 ZCode 使配置生效）
- Grok: `~/.grok/hooks/agent-notify.json`（写入 hooks → 命令 `agent-notify handle-grok-hook`；项目 scope 为 `.grok/hooks/agent-notify.json`）

### 企业微信机器人绑定小技巧

1. **创建单人通知群**：在企业微信中发起群聊（随便拉几个同事），创建成功后**不要在群里发言**，直接将其他人移出，此时该群将变成你的单人通知群；
2. **添加机器人**：「群设置」->「消息推送」->「添加」-> 「自定义消息推送」，命名并保存；
3. **获取 Webhook 地址**：复制生成的地址，格式类似 `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`；
4. **绑定配置**：运行 `npx agent-notify`，在配置向导中选择启用企业微信渠道，粘贴 Webhook URL 即可；
> 旧版企业微信添加机器人步骤：「群设置」->「群机器人」->「添加机器人」-> 「新建机器人」，命名并保存

### Bark 配置说明

1. **复制 Bark URL**：在 Bark App 内复制测试 URL，例如 `https://api.day.app/<key>/这里改成你自己的推送内容`；
2. **绑定配置**：运行 `npx agent-notify`，进入「消息渠道配置」->「初始化 Bark」，粘贴 Bark URL 即可；
3. **Codex 任务完成通知**：在配置文件 `~/.agent-notify/config.yaml` 中保留 Codex 的 `run_completed` 事件，并启用 `notify.codex.channels.bark`。

Bark URL 会作为本地配置保存，发送时使用 Bark 的 POST JSON 参数 `title` 和 `body`。

## 工作流程

<p align="center">
  <img src="assist/workflow.png" alt="工作流程图" />
</p>

## 效果图

| |                                                              |
|:---:|:------------------------------------------------------------:|
| <img src="assist/launch-setting.png" alt="软件配置" width="75%"> |  <img src="assist/feishu-bind.png" alt="飞书绑定" width="75%">   |
| **软件配置** |                           **飞书绑定**                           |
| <img src="assist/feishu-notify-phone.png" alt="飞书通知" width="50%"> | <img src="assist/wecom-notify.jpg" alt="企业微信通知" width="55%"> |
| **飞书通知** |                          **企业微信通知**                          |
| ![系统通知](assist/system-notify.png) |                                                              |
| **系统通知** |                                                              |

## Friendship Link

Thanks for the support and feedback from the friends at [LINUX DO](https://linux.do/).

