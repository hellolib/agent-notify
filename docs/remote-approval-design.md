# 飞书远程审批设计方案（Remote Approval Design）

> 状态：设计草案  
> 目标：用户在飞书消息卡片上点击"允许 / 拒绝"按钮，远程完成 Codex 权限审批，无需回到终端操作。

## 1. 问题分析

### 1.1 现状

```
Codex PermissionRequest
  → handle-codex-hook（发通知后立即退出）
  → 飞书收到卡片（有按钮）
  → 用户点击按钮 → 飞书回调目标服务 → "回调服务当前未在线"
  → Codex 终端仍显示权限提示，用户必须回终端手动操作
```

### 1.2 核心矛盾

| 矛盾 | 说明 |
|------|------|
| hook 即退 | 当前 `handle-codex-hook` 发完通知就退出，不等待用户响应 |
| 无回调服务 | 没有服务端接收飞书按钮回调 |
| 网络隔离 | 本机在公司内网 + DNS 污染，飞书服务器无法直接访问本机 |
| hook 能力未知 | 不确定 Codex hook 能否阻塞等待、能否通过 stdout 返回审批决定 |

## 2. 总体架构

```
┌─────────────────────────────────────────────────────────────┐
│  Codex CLI                                                   │
│    │                                                         │
│    │ PermissionRequest (stdin: hook payload)                 │
│    ▼                                                         │
│  handle-codex-hook (agent-notify)                            │
│    │                                                         │
│    ├─① 生成 request_id，写 pending 请求文件                    │
│    ├─② 发飞书卡片（按钮 value 带 request_id）                 │
│    ├─③ 阻塞等待响应（轮询文件 / Unix socket，超时 120s）       │
│    │                                                         │
│    │  ┌──────────────────────────────────────────────┐       │
│    │  │ 用户在飞书点击"允许"                          │       │
│    │  │     │                                        │       │
│    │  │     ▼                                        │       │
│    │  │ 飞书服务器 POST 回调                          │       │
│    │  │     │                                        │       │
│    │  │     ▼                                        │       │
│    │  │ Cloud Relay（公网 HTTP 服务）                 │       │
│    │  │     │ WebSocket 长连接（由本地 serve 发起）    │       │
│    │  │     ▼                                        │       │
│    │  │ agent-notify serve（本地回调服务）             │       │
│    │  │     │ 写 pending 请求文件 / 发 Unix socket     │       │
│    │  │     ▼                                        │       │
│    │  └──────┘                                       │       │
│    │                                                         │
│    ├─④ 收到响应 → 输出决策 / 注入终端按键                     │
│    └─⑤ 超时 → 退出，Codex 回退到正常终端提示                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## 3. 核心组件设计

### 3.1 Pending Request Store（待审批请求存储）

**位置**：`~/.agent-notify/pending-requests/`

**结构**：每个请求一个 JSON 文件，文件名为 `<request_id>.json`

```json
{
  "request_id": "a3b1c2d4-...",
  "session_id": "sess-codex-1",
  "tool_name": "Bash",
  "tool_input": {"command": "git status"},
  "status": "pending",
  "action": "",
  "created_at": "2026-07-27T16:00:00+08:00",
  "resolved_at": ""
}
```

**状态流转**：`pending` → `approved` / `denied` / `timeout`

**清理**：启动时扫描，删除超过 10 分钟的 pending 请求。

### 3.2 Callback Server（回调服务端）

新增 `agent-notify serve` 子命令，启动本地 HTTP 服务：

```
agent-notify serve --listen 127.0.0.1:7896
```

**职责**：
- 接收 Cloud Relay 推送的飞书回调
- 解析 `request_id` + `action`（allow / allow_session / reject）
- 更新 pending 请求文件
- 通知等待中的 hook 进程（写文件 + Unix socket 信号）

**回调处理流程**：
```
POST /feishu/callback
  → 校验来源（飞书签名 / token）
  → 解析 card.action.value: {"action": "allow", "request_id": "a3b1c2d4-..."}
  → 查找 pending 请求文件
  → 更新 status + action + resolved_at
  → 返回 200
```

### 3.3 Cloud Relay（公网中继服务）

**为什么需要**：本机在公司内网 + DNS 污染，飞书服务器无法直接访问本机。

**方案**：在公网 VPS 上部署一个轻量 relay 服务。

**工作方式**：
1. 本地 `agent-notify serve` 启动时，通过现有代理（10.10.73.85:7894）向 Cloud Relay 发起 WebSocket 长连接
2. 飞书回调 URL 配置为 `https://relay.example.com/feishu/callback`
3. 用户点击按钮 → 飞书 POST → Cloud Relay → 通过 WebSocket 推送给本地 serve → 本地处理

**Cloud Relay 最小实现**（单文件 Go / Node.js）：
```
WebSocket 长连接管理 + HTTP 回调转发
```

**替代方案**：如果不想维护 VPS，可用 frp / cloudflare tunnel 等内网穿透工具，
通过现有代理隧道暴露本地 serve 端口。但需确认代理是否支持反向隧道。

### 3.4 Hook 修改（handle-codex-hook）

**修改 `internal/codexhooks/handler.go` 的 Handle 函数**：

```
PermissionRequest 事件时：
  1. 生成 request_id (UUID)
  2. 写 pending 请求文件
  3. 调用 Dispatch 发送飞书通知（按钮 value 带 request_id）
  4. 阻塞等待：
     a. 轮询 pending 请求文件（每 500ms），或
     b. 监听 Unix socket ~/.agent-notify/pending-requests/<request_id>.sock
  5. 收到响应：
     a. 如果 Codex 支持 hook stdout 返回决策 → 输出 JSON 决策
     b. 如果不支持 → 通过 TTY 注入按键（见 3.5）
  6. 超时（默认 120s）：退出，Codex 回退正常终端提示
```

**关键参数（config.yaml 新增）**：
```yaml
remote_approval:
  enabled: true
  timeout_seconds: 120
  relay_url: "https://relay.example.com"
  relay_token: "xxx"
```

### 3.5 决策返回机制（需验证）

**方案 A：hook stdout 返回决策（优先）**

如果 Codex 的 PermissionRequest hook 支持 stdout 返回 JSON 决策：
```json
{"decision": "allow"}
```
则 hook 收到飞书回调后直接输出，Codex 自动执行。这是最干净的方案。

**验证方法**：在 hook 中输出测试 JSON，观察 Codex 行为。

**方案 B：TTY 按键注入（兜底）**

如果 hook 无法返回决策，但 hook 阻塞时 Codex 等待 hook 退出后再显示终端提示：
1. hook 发通知后阻塞等待
2. 收到审批 → hook 退出
3. Codex 显示终端提示
4. 另一个进程（serve）向终端 TTY 注入按键（如 y/Enter 表示允许）

**方案 C：完全不阻塞（最简）**

hook 发通知后立即退出，Codex 正常显示终端提示。
serve 收到回调后向终端 TTY 注入按键。
优点：不依赖 hook 阻塞能力。缺点：如果用户已在终端操作可能冲突。

## 4. 飞书卡片按钮修改

### 4.1 按钮值携带 request_id

```go
// buildCard 中按钮的 value 字段
"value": map[string]any{
    "action":      a.Value,       // "allow" / "allow_session" / "reject"
    "request_id":  msg.RequestID, // 新增字段
}
```

### 4.2 Message 新增字段

```go
type Message struct {
    // ... 现有字段 ...
    RequestID string // 远程审批请求 ID，用于关联飞书回调
}
```

## 5. 网络拓扑

```
                        ┌─── 公网 ───┐
                        │            │
┌────────┐    HTTPS    │  ┌──────┐  │    WebSocket(出站，穿代理)
│ 飞书服务器 │ ────────→ │  │ Cloud │  │  ┌────────┐
└────────┘  回调        │  │ Relay │  │  │ 本地     │
                        │  └──────┘  │  │ serve   │
                        └────────────┘  └───┬────┘
                                            │ 文件/socket
                                            ▼
                                        ┌────────┐
                                        │ hook   │
                                        │ (阻塞) │
                                        └────────┘
```

**出站连接**（本地 → Cloud Relay）走现有代理，无需公网入站。

## 6. 实现步骤

| 步骤 | 内容 | 依赖 |
|------|------|------|
| 1 | Message 加 RequestID 字段 | 无 |
| 2 | buildCard 按钮 value 带 request_id | 步骤 1 |
| 3 | Pending Request Store 实现 | 无 |
| 4 | Hook 阻塞等待逻辑（文件轮询 + 超时） | 步骤 3 |
| 5 | agent-notify serve 子命令（回调处理） | 步骤 3 |
| 6 | config.yaml 加 remote_approval 配置 | 无 |
| 7 | Cloud Relay 部署 | 步骤 5 |
| 8 | 验证 Codex hook stdout 决策能力 | 无 |
| 9 | 根据验证结果选择方案 A/B/C | 步骤 8 |
| 10 | 飞书应用配置回调 URL | 步骤 7 |

## 7. 风险与降级

| 风险 | 降级方案 |
|------|----------|
| Codex hook 不支持阻塞 | 用方案 C（TTY 注入），hook 不阻塞 |
| Codex hook 不支持 stdout 决策 | 用方案 B/C（TTY 注入） |
| Cloud Relay 不可用 | hook 超时后退出，回退终端手动操作 |
| 用户不响应 | 超时（默认 120s）后退出，Codex 正常提示 |
| serve 未启动 | hook 检测到 serve 不在线时跳过阻塞，立即退出 |
| 网络中断 | 同超时降级 |

## 8. Codex hook 能力确认（源码验证）

通过阅读 Codex 源码（`codex-rs/hooks/`）已确认：

### 8.1 hook 同步阻塞（✅ 支持）

`codex-rs/hooks/src/engine/command_runner.rs:104`：
```rust
match timeout(timeout_duration, child.wait_with_output()).await {
```
- hook 进程不退出，Codex **一直阻塞等待**（async wait）
- 默认超时 **600 秒**（10 分钟），PermissionRequest 走标准默认值
  （`normalize_command_hook` 中非 SessionEnd 事件 `unwrap_or(600)`）
- 超时后 kill hook 进程，结果标记 `timeout`

### 8.2 stdout 返回决策（✅ 支持）

hook stdout 输出 JSON，Codex 解析 `hookSpecificOutput.decision`。

决策 JSON 格式（`codex-rs/hooks/src/schema.rs:185-222`）：
```json
{
  "hookSpecificOutput": {
    "hookEventName": "PermissionRequest",
    "decision": {
      "behavior": "allow",
      "message": "可选，deny 时的拒绝原因"
    }
  }
}
```

- `behavior: "allow"` → Codex 直接放行（无需用户在终端确认）
- `behavior: "deny"` + `message` → Codex 拒绝执行并显示原因
- 不输出 JSON / decision 为 null → 无决策，Codex 继续正常审批流程

### 8.3 退出码语义（exit_code）

`codex-rs/hooks/src/events/permission_request.rs:248-270`：
- exit 0 + JSON decision → 使用 stdout 决策
- exit 2 + stderr 消息 → deny（stderr 作为拒绝原因）
- 其它 exit code → 标记 Failed，无决策

### 8.4 结论：采用方案 A

hook 阻塞等待飞书回调 → 收到后 stdout 输出决策 JSON → exit 0。
**无需 TTY 按键注入（方案 B/C 废弃）**，这是最干净的实现路径。

## 9. 确定的实现方案

### 9.1 hook 行为

```
PermissionRequest 事件:
  1. 生成 request_id
  2. 写 pending 请求文件
  3. 发飞书通知（按钮带 request_id）
  4. 阻塞轮询 pending 文件（500ms 间隔，超时 590s，留 10s 余量）
  5. 收到响应:
     - allow → stdout JSON {behavior: allow} → exit 0
     - deny  → stdout JSON {behavior: deny, message} → exit 0
  6. 超时:
     - 不输出 decision → exit 0 → Codex 回退正常审批流程
  7. serve 未启动:
     - 检测到无 pending 目录/serve 不在线 → 跳过阻塞，立即 exit 0
```

### 9.2 按钮与 request_id

飞书卡片按钮 value:
```json
{"action": "allow", "request_id": "a3b1c2d4-..."}
```

## 10. 待确认决策点

1. **Cloud Relay 方案**：自建 VPS / frp 隧道 / 其它？
2. **默认超时**：590s（Codex 600s 限制下留余量）是否合适？
3. **安全**：回调是否需要签名验证？request_id 是否需要加密？
4. **多 Agent**：是否只支持 Codex，还是也要支持 Claude Code / ZCode / Grok？
