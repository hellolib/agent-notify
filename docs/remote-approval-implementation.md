# 飞书远程审批 Codex 实现方案

**目标**：用户在飞书消息卡片点击"允许/拒绝"按钮，远程完成 Codex 权限审批，无需回终端手动操作。

## 整体流程

```
Codex PermissionRequest
  → handle-codex-hook (agent-notify)
     ├─ 生成 request_id，写 pending 请求文件
     ├─ 发飞书卡片（按钮 value 带 request_id）
     ├─ 启动 inject-wait 后台子进程（继承控制终端，轮询 pending 文件）
     └─ 立即返回空 → Codex 回退终端审批菜单（TUI 正常弹出）
            ↓
  用户在飞书点击按钮 → 飞书 WebSocket 长连接 → serve 收到回调
     ├─ 更新 pending 请求文件状态 (approved/denied)
     └─ inject-wait 轮询到决策 → TIOCSTI 注入按键到终端 (y/p/ESC)
```

## 核心组件

### 配置层 — `internal/config/config.go`

- 新增 `RemoteApprovalConfig` 结构：`enabled`、`wait_seconds`、`timeout_seconds`(590) 等
- YAML key 为 `remote_approval`，默认关闭

### 通知卡片按钮 — `internal/notify/message.go` + `feishu.go` + `codexhooks/event.go`

- `Message` 新增 `Actions []Action` 和 `RequestID` 字段
- `feishu.go` 的 `buildCard` 将 Actions 渲染为飞书交互按钮，value 含 `action` + `request_id`
- `permissionActions(mode)`：default 模式 3 按钮（允许 / 允许不再询问 / 拒绝），其它模式 2 按钮

### 待审批请求存储 — `internal/state/pending.go`（新增）

- 文件式存储于 `~/.agent-notify/pending-requests/<id>.json`
- 状态流转 `pending → approved/denied/timeout`
- 提供 Save/Load/Resolve/CleanExpired/ListPending 等方法

### Hook 入口分发 — `internal/codexhooks/handler.go`

- 当 `permission_required` 且 `remote_approval.enabled` 时，走 `runRemoteApproval` 分支

### 远程审批逻辑 — `internal/codexhooks/remote_approval.go`（新增）

- `runRemoteApproval`：非阻塞模式，生成 request_id → 写 pending → 发卡片 → 启动 inject-wait 子进程 → 返回空让 Codex 回退菜单

### 回调服务 — `internal/cli/serve.go`（新增 `serve` 子命令）

- 飞书 WebSocket 长连接（出站连接，无需公网 IP）接收 `CardActionTrigger` 回调
- 本地 HTTP `/feishu/callback` 作备用测试入口
- 回调 → 解析 action/request_id → `ResolvePending` 更新状态

### 按键注入 — `internal/codexhooks/tiocsti_*.go` + `internal/cli/inject_wait.go`

- `inject-wait` 隐式子命令：后台轮询 pending 文件（500ms），收到决策后经 TIOCSTI ioctl 向 `/dev/tty` 注入按键
- 按键映射：`allow→y`、`allow_prefix→p`、`reject→ESC(0x1b)`
- TIOCSTI 实现 Linux/非 Linux 分平台文件，仅 Linux 可用

### 测试辅助 — `Makefile`

- 新增 `dev-serve`、`dev-test-hook`、`dev-approve`、`dev-reject`、`dev-approval-on/off` 等 target，本地可完整模拟审批闭环
- vendor 目录补充了 `gorilla/websocket`、`gogo/protobuf` 等飞书 SDK 依赖

## 关键设计取舍

- **非阻塞而非阻塞等待**：hook 不阻塞等待 stdout 决策，而是回退 TUI 菜单 + 后台注入按键，规避了 hook 能力不确定性
- **WebSocket 长连接**：本地 serve 主动出站连接飞书，绕开内网/DNS 污染下的公网回调难题，无需公网中继
- **TIOCSTI 注入**：inject-wait 继承 hook 的控制终端（不 setsid），因此可打开 `/dev/tty` 向 Codex TUI 注入按键
