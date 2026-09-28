# DeepSeek Harness (DSH) 接入开发计划

> 状态：**计划（未开工）**；本文只描述方案与任务拆解，不含实现
> 范围：让 `agent-notify` 支持 DeepSeek Harness 作为第 8 个 agent（与 Claude Code / Codex / ZCode / Grok / Droid / OpenCode / OMP 并列）
> 关联：`docs/window-level-focus-design.md`（同为 docs/ 下的设计文档，本文沿用其「状态 / 范围 / 关联」抬头）
> 调研基线：`deepseek-harness`（本机 checkout `/Users/liming/deepseek-harness/`，版本 `0.1.7-alpha.2`）+ `agent-notify` @ `6ba04bf`

---

## 0. 先把问题定义清楚

「添加 deepseek-harness 支持」在本仓库语境下只有一个合理解读：

**把 DSH 当作又一个被通知的 agent**——DSH 跑任务时，用户在终端/桌面收到「等待授权 / 等待输入 / 运行完成 / 运行失败」通知。这与既有 7 个 agent 的接入方式同构，**不是**把 DeepSeek 模型接进某个 harness，也不是引入一个新的事件总线。

理由（读代码得出，非推断）：

- 本仓库 `go.mod` 中**没有任何 LLM/SDK 依赖**，全部是 cobra / survey / yaml / 平台通知库，仓库不调用模型。
- `internal/agentintegrations/integration.go:6-28` 定义了唯一的接入抽象 `Integration`（`Name/DetectInstalled/SettingsPath/Install/Uninstall/IsHookInstalled`），7 个 agent 各实现一份，形状完全一致。
- `internal/notify/message.go:5-21` 的 `Message` 只有 `Agent/Event/SessionID/Workspace/Title/Body/SourceApp/Focus*`，没有任何模型相关字段。

因此本次是**新增一个 agent 集成**，复用既有抽象，不需要「改造现有抽象」。

---

## 1. 调研结论：DSH 的接入面在哪

DSH 是「all-plugin Cordis agent harness」（`AGENTS.md:3`）。它对外提供**两条**可挂接的路径，本计划分别称为路线 A / 路线 B。

### 1.1 路线 A：复用 DSH 自带的 Claude Code hook bridge

DSH 内置 `@deepseek-ai/dsh-hooks-claude-code`（`packages/hooks/hooks-claude-code/`），能直接跑「未修改的 Claude Code `hooks.json` 命令式 hook」。挂载方式（该包 README「Smallest working setup」）：

```yaml
- name: '@deepseek-ai/dsh-hooks-claude-code'
  config:
    configPath: ./.claude/hooks.json
```

**关键限制（决定路线选择）：它只认 CC 的 7 个事件，且未列出的事件被静默丢弃。**

`packages/hooks/hooks-claude-code/src/config.ts:11-19` 的白名单：

```ts
const CLAUDE_EVENTS = [
  'SessionStart', 'UserPromptSubmit', 'PreToolUse',
  'PostToolUse', 'Stop', 'SubagentStart', 'SubagentStop',
] as const
```

而 `agent-notify` 写给 CC 的事件是 5 个（`internal/claudehooks/settings.go:17-23`）：`SessionStart / PermissionRequest / Notification / Stop / PostToolUseFailure`。

**交集只有 `SessionStart` 和 `Stop`**：

| agent-notify 写入 CC 的事件 | 归一化事件 | DSH CC bridge 是否支持 |
|---|---|---|
| `SessionStart` | `session_start`（仅聚焦捕获） | ✅ 支持 |
| `PermissionRequest` | `permission_required` | ❌ **静默丢弃** |
| `Notification` | `input_required` | ❌ **静默丢弃** |
| `Stop` | `run_completed` | ✅ 支持 |
| `PostToolUseFailure` | `run_failed` | ❌ **静默丢弃**（bridge 只有 `PostToolUse`） |

`parseClaudeCodeConfig` 只遍历白名单（`config.ts:86`），不在表里的事件连 group 都不会被 parse，**不报错、不告警**——用户会以为装好了，实际只有 2/4 事件生效。

**结论**：路线 A 可以「白送」`session_start` + `run_completed`，但拿不到 `permission_required` / `input_required` / `run_failed`，而**「等待授权」恰恰是这类工具最核心的通知场景**。路线 A 不足以作为交付方案，最多作为降级/兼容说明。

### 1.2 路线 B：写一个原生 DSH 插件（推荐）

DSH 的官方立场明确写在 `.agents/notes/implemented/feature/2026-06-30-interception-extension-points.md`：

> **"native hooks" are not a package** — a native hook is just an ordinary Cordis plugin subscribing to the canonical lifecycle events.
> Anything a bridge can do, a plain plugin can do directly — more powerfully (no serialization boundary, full `ctx`, typed returns).

原生插件能直接订阅到 CC bridge 覆盖不到的**授权与提问**事件，从而拿到全部 4 个通知事件 + 聚焦捕获。已核实的事件签名（`packages/core/agent/src/runtime-types.ts`、`packages/interaction/*/src/types.ts`）：

| DSH 扩展点 | 声明位置 | 类型 | 载荷 | 映射到 |
|---|---|---|---|---|
| `agent/created` | `runtime-types.ts:261` | emit + await | `{ agent, source: 'startup'|'resume'|'clear'|'compact', signal? }` | `session_start` |
| `approval/request` | `user-approval/src/types.ts:85` | **waterfall** | `(req: ApprovalRequestEvent, next) => ApprovalOutcome` | `permission_required` |
| `user-questions/request` | `user-questions/src/types.ts:88` | **waterfall** | `(request: AskUserQuestionRequestEvent, next) => AskUserQuestionAnswer` | `input_required` |
| `agent/turn-stopping` | `runtime-types.ts:381` | serial (awaited) | `{ agent, turn, signal }` | `run_completed` |
| `agent/error` | `runtime-types.ts:393` | emit | `{ agent, turn, step, error }` | `run_failed` |

字段来源：`agent.session.header.id`（session id）与 `agent.session.header.cwd`（工作区），见 bridge 的 `base()`（`hooks-claude-code/src/index.ts:327-336`）。

涉及的关键类型字段：
- `ApprovalRequest`（`user-approval/src/index.ts:111-132`）：`agent` / `toolName` / `callId?` / `reason?` / `signal?`。**不含工具参数**（设计上刻意如此，避免与已流式的 tool call 重复）。
- `AskUserQuestionRequestEvent`（`user-questions/src/types.ts:70-77`）：`questions[]` / `agent?` / `signal?`。

**结论：路线 B 是唯一能拿全事件面的方案，作为主方案。**

---

## 2. 三个必须写进设计的正确性约束

这三条是本计划与「照抄 OMP 接入」的实质差别，写错会**破坏 DSH 本身**而不只是漏通知。

### 2.1 `approval/request` 与 `user-questions/request` 是 waterfall，必须 `return next()`

两者都是瀑布式（`@mode waterfall`）。按 `docs/cordis-primer.md:31` 的定义：

> A listener receives `(...args, next)`. Call `next()` to delegate the possibly wrapped result to the next service; return without `next()` to short-circuit.

如果我们的监听器**不调用 `next()`** 就返回，等于把整条审批链短路：`ApprovalOutcome` 的闭集是 `'allowed-once' | 'rejected' | 'cancelled' | 'unavailable'`（`docs/subsystems/approval.md`），且是**fail-closed**——插件的通知函数会直接把用户的授权流程打坏（工具被拒或永远弹不出来）。

**因此插件里这两个监听器必须是「通知 + 无条件委托」：**

```js
ctx.on('approval/request', (req, next) => {
  try { notify('permission_required', req) } catch {}
  return next()          // ← 不可省略、不可替换
})
```

这与 CC bridge 的 `next()` 纪律一致，也是本接入**唯一的破坏性风险点**，必须由集成测试锁住（见 §5 M2）。

### 2.2 通知必须非阻塞、分离式 spawn

hook 进程绝不能拖慢 agent。既有 OpenCode 插件已给出本仓库认可的模式（`internal/opencodehooks/plugin/opencode.js:39-46`）：

```js
const child = spawn(BINARY, ["handle-opencode-hook"], {
  stdio: ["pipe", "ignore", "ignore"],
  detached: true,
})
child.stdin.write(JSON.stringify(payload))
child.stdin.end()
child.on("error", () => {})
child.unref()
```

DSH 侧同理：`spawn(detached) + unref()`，**不 await、不冒泡异常**。特别注意 `agent/created` 是 **await 的**（`agent/created` 的返回会被创建流程等待，且 `agent.creation ... rejects on failure`），所以该监听器里的任何抛出都可能**污染会话启动**。全部监听器体都包 `try/catch`。

### 2.3 必须过滤子 agent，否则 `session_start` 会刷屏

`agent/created` 对**每个 Agent** 都触发，包括子 agent——`packages/subagent/tool-subagent/src/index.ts:703` 正是靠它给子 agent 装工具的。若不区分根/子，一次任务里每个 subagent 都会写一条 `session_start`，既刷通知（虽然 `session_start` 不发通知）又**污染聚焦窗口缓存**（`state.FocusStore` 按 session 键，写入大量无用条目）。

需要在插件侧判定「根 agent」，只对根 agent 发出 `session_start`。**根/子的判定字段需要在 M0 中确认**（`Agent` 的可区分标识，如 parent/root 引用），这是 M0 的交付物之一，不能靠猜。

---

## 3. 接口约定（插件 ↔ Go 二进制）

沿用既有 agent 的「stdin 一行 JSON」契约，不引入新协议。

### 3.1 事件载荷（插件 → `agent-notify handle-dsh-hook`）

```json
{
  "hook_event_name": "ApprovalRequest",
  "session_id": "sess_xxx",
  "cwd": "/path/to/workspace",
  "tool_name": "bash",
  "reason": "optional human-readable why",
  "error": "optional, only for RunFailed"
}
```

设计取舍：**字段名沿用 CC 方言**（`hook_event_name` / `session_id` / `cwd` / `tool_name` / `tool_response`），而不是新造 DSH 方言。理由：`internal/claudehooks/event.go:13-22` 的 parser 已经把这些字段连同「类型意外的容错」处理好了（`tool_response` 用 `json.RawMessage` 容错，见 issue #32 注释），复用可少写一套解析与测试。`hook_event_name` 取值：

| 取值 | 归一化事件 | 说明 |
|---|---|---|
| `SessionStart` | `session_start` | 仅聚焦捕获，不发通知（`agenthooks.Dispatch` 拦截） |
| `ApprovalRequest` | `permission_required` | |
| `UserQuestion` | `input_required` | |
| `Stop` | `run_completed` | |
| `RunFailed` | `run_failed` | |

### 3.2 二进制定位

既有 OpenCode/OMP 的做法是**安装时把绝对路径烘焙进插件文件**（`__AGENT_NOTIFY_BINARY__` 占位符 → `common.ResolveBinaryPath`，见 `internal/omphooks/settings.go:17`（占位符）、`:66`（`Install`）、`:153`（替换）、`:76`（`BakedBinaryPath` 反向解析））。

但 DSH 插件的分发实体是 **npm 包**（见 §4.2），发布时无法烘焙用户机器上的路径。因此约定：插件按以下顺序解析，**与 npx launcher 的落点一致**：

1. 环境变量 `AGENT_NOTIFY_BINARY`（显式覆盖，便于 `make local` 与测试）
2. `~/.agent-notify/agent-notify`（Windows 为 `agent-notify.exe`）——npx launcher 与 `make local` 的共同落点
3. 找不到则静默放弃（**不抛出**，绝不阻塞 agent）

---

## 4. 分发与安装机制

### 4.1 DSH 的插件模型（已核实）

- Profile 目录：`~/.dsh/profiles/<name>/`，含 `package.json` 与 `cordis.patch.yml`（`packages/boot/app-boot/src/profile.ts:1-20`）。
- 插件是 npm 包，manifest 里声明 `"dsh": { "bundle": { "patch": "./cordis.patch.yml" } }`，patch 文件用 `insert:` 插入自己的 entry。
- 官方安装命令（whale widget 的 README 与本机 `~/.dsh/profiles/desktop/package.json` 均验证）：
  ```bash
  dsh plugin --profile web add <package>        # 从 npm 安装
  dsh plugin --profile web add link:<abs-path>  # 本地开发链接
  ```
- `dsh plugin` 内部转发给 pnpm 并维护 `dsh.profile.bundles`（`apps/cli/src/plugin.ts:10-24`、`packages/boot/plugin-manager/src/index.ts:640-650`）。

### 4.2 落地形态：独立的 npm 包

新建包 `agent-notify-dsh`（名字待定，见 §7 开放问题 2），结构对齐既有第三方插件 `dsh-whale-widget`：

```
agent-notify-dsh/
├── package.json          # type: module, dsh.bundle.patch → ./cordis.patch.yml
├── cordis.patch.yml      # - insert: [{ id: agent-notify-dsh, name: agent-notify-dsh }]
├── lib/index.js          # Cordis 插件：注册 5 个监听器
└── README.md
```

**为什么不让 Go 二进制直接写插件文件**（像 OpenCode/OMP 那样）：DSH 的 loader 按 **package name** 做 Node 模块解析（`resolveBundleDir('dsh', name, anchor, dir)`，`plugin-manager/src/operations.ts:58-61`），而 profile 的 `dsh.profile.bundles` 也是包名列表。走 npm 包可以让 `dsh plugin add` 自己完成 pnpm 解析、链接与 bundles 记账；自行改写 profile 的 `package.json` + `node_modules` 反而要重新实现 pnpm 与锁。

> **M0 必须验证的降级路径**：若 `cordis.patch.yml` 的 `insert[].name` 接受**绝对路径或 `file:`/`link:` 说明符**，则「Go 二进制写文件 + 绝对路径 entry」也可行，可作为无 npm 发布场景的备选。这一条**尚未验证**，不得在未验证前写进实现。

### 4.3 agent-notify 侧如何安装

`DshIntegration.Install()` 负责：
1. 定位 profile：默认 `web`，用户可选（**必须排除 `desktop`**——`apps/cli/src/args.ts` 的 `rejectElectronProfile` 会直接报错，desktop 由 Electron 应用独占管理）。
2. 执行 `dsh plugin --profile <p> add agent-notify-dsh`（或本地开发用 `link:<path>`）。
3. 失败时给出可操作的提示（`dsh` 不在 PATH、pnpm 缺失、网络问题分情况），并**不写坏任何 DSH 配置**。

`Uninstall()` 对应 `dsh plugin --profile <p> remove agent-notify-dsh`。

**注意：`dsh` 不在 PATH 是本机现状**（`which dsh` → not found；DSH 从 checkout 启动，`node apps/cli/lib/bin.js`）。所以 `DetectInstalled()` 不能只做 `exec.LookPath("dsh")`，需要多信号：PATH → `$DSH_HOME`/`~/.dsh` 目录存在 → checkout 内的 `apps/cli/lib/bin.js`。安装时若走不到 `dsh` 命令，应回退为「打印指引」而不是报错退出（对齐 `setup.*_tip` 的既有提示风格）。

---

## 5. 任务拆解

估算为**净开发时间**（不含评审/等待），标 `?` 的项因存在未验证前提而区间较大。

### M0 · 前置验证（0.5–1 天，无生产代码）

| # | 任务 | 产出 | 验收 |
|---|---|---|---|
| M0-1 | 确认 `Agent` 上区分根/子 agent 的字段 | 结论记录进本文 §2.3 | 能写出「只对根 agent 触发」的判定表达式 |
| M0-2 | 验证 listener 里 `spawn(detached)` 不阻塞 `agent/created` 等待链 | 最小 demo 脚本 | 注入 100ms 延迟不改变 DSH 会话启动耗时 |
| M0-3 | 验证 `return next()` 后审批流正常（不短路） | demo + 记录 | 授权弹窗照常出现，`ApprovalOutcome` 不被篡改 |
| M0-4 | 验证 profile entry `name` 是否接受绝对/相对路径说明符 | 结论，决定 §4.2 是否需要降级路径 | — |
| M0-5 | 确认 `dsh` 在本机与 CI 的可调用形式 | 检测策略写进 §4.3 | `DetectInstalled()` 的三信号策略可落地 |

**M0 是 gate**：M0-1/M0-3 任一结论与假设不符，需回到本文修订 §2 与 §3 再开工。

### M1 · 插件本体（1.5–2 天）

| # | 任务 | 文件 |
|---|---|---|
| M1-1 | 建包骨架（`package.json` / `cordis.patch.yml` / `lib/index.js`） | 目录待定，见 §7 开放问题 2（暂定 `packages/dsh-plugin/`） |
| M1-2 | 5 个监听器 + 根 agent 过滤 + 全 `try/catch` | `lib/index.js` |
| M1-3 | 二进制解析三优先级 + 分离式 spawn | `lib/index.js` |
| M1-4 | 双 waterfall 的 `return next()` 纪律 + 单测 | `tests/*.spec.js` |
| M1-5 | 打包与发布脚本（`prepack`、`files` 白名单） | `package.json` |

### M2 · Go 侧接入（2–2.5 天）

严格照 OMP 的接入面铺开（OMP 是本仓库最近一次新增 agent，其触点即清单）：

| # | 任务 | 文件（行号为现有 OMP 对应位置） |
|---|---|---|
| M2-1 | 新建 hooks 包：`event.go` / `handler.go`（`handler.go` 与 `omphooks/handler.go` 同构，约 20 行） | `internal/dshhooks/` |
| M2-2 | 新建 `Integration` 实现 | `internal/agentintegrations/dsh.go` |
| M2-3 | 配置结构：`AgentConfig.DSH` + `NotifyConfig.DSH` + `Default()` 的 `dshEvents` + 加入 `NotifyConfig.All()` | `internal/config/config.go:24-30, 58-64, 73-75, 186+` |
| M2-4 | 显示名 + logo 映射 | `internal/notify/format.go:18-37`、`internal/notify/icon.go:10-19` |
| M2-5 | logo 资源 + 守卫测试的 agent 列表 | `assist/logo/agentlogo/dsh.png`、`internal/notify/icon_test.go:93`（`supportedAgents`） |
| M2-6 | `buildSenders` 分支 | `internal/agenthooks/dispatch.go:142` |
| M2-7 | CLI：`handle-dsh-hook` + `dsh print-hooks/install-hooks` + 注册 | `internal/cli/handler_dsh.go`、`internal/cli/dsh.go`、`internal/cli/root.go:32-53` |
| M2-8 | 向导接线：agent 常量、agent 选项、事件选项、渠道/事件读取、`configureAgent` 分支、`disableAgentNotification` 分支 | `internal/app/setup/flow.go:15-21, 89, 192, 211, 230, 263, 466`、`internal/app/setup/service.go:45, 117, 205, 315, 395` |
| M2-9 | doctor：集成字段、结果字段、渠道枚举、`DiagnosticStatus` | `internal/app/doctor/service.go:34, 46, 91, 166-199, 219, 292` |
| M2-10 | CLI 其余触点：`applyChannelToAgents`、`clean_targets`、配置视图行、`settingsPathForAgent`、`runInitFlow` | `internal/cli/channel_apply.go:22`、`clean_targets.go:37`、`actions.go:409, 435, 81`、`menu.go:27` |

**M2-3 有两个现成的守卫测试**（都在 `internal/config/config_test.go`，无需新写，但必须让它们继续通过）：

1. `TestNotifyConfigAllCoversEveryAgent`（`:406`）——断言 `All()` 的长度等于 `NotifyConfig` 的字段数（`reflect` 比对）。漏加即测试期暴露。`config.go:71-73` 的注释记录了 Droid 接入时漏加的真实 bug：只配 Droid 的用户完全冻结不了渠道。
2. `TestLoadFillsDefaultsForAgentAddedAfterConfigWasWritten`（`:418`）——回归测试「老配置（无该 agent 段）加载后必须拿到 `Default()` 的值而非 Go 零值」。

第 2 条正是 CLAUDE.md 记录的 `click_to_focus` 事故的守卫：`Load` 把 YAML 反序列化**进** `Default()`（而非零值结构体），所以新增 agent 只要在 `Default()` 里给了正确的字段初值就自动生效——**但绝不能走「解析进零值再手工补字段」的老路**。

### M3 · 测试与文档（1–1.5 天）

| # | 任务 |
|---|---|
| M3-1 | `testdata/dsh-hooks/*.json` 五类事件样本（对齐 `testdata/droid-hooks/` 命名） |
| M3-2 | `event_test.go` 表驱动解析测试 |
| M3-3 | `integration_test.go`：`Name`/`SettingsPath`/`Install`/`DetectInstalled`（对齐既有 `TestClaudeIntegration_*` 风格） |
| M3-4 | 插件侧 waterfall 纪律测试（`return next()` 未被吞）+ 根 agent 过滤测试 |
| M3-5 | doctor 单测 + 向导流程测试 |
| M3-6 | 端到端手测：真机跑 DSH，四类事件各触发一次通知，验证点击聚焦 |
| M3-7 | README / README.zh-CN：agent 表、事件矩阵、安装路径说明；CLAUDE.md 架构段补 `dshhooks` |

### M4 · 发布（0.5 天）

- `assist/logo/agentlogo/*.png` 已被 release workflow 通配打包（`.github/workflows/release.yml:63-78`），**只需 commit 图片**，无需改 workflow。
- 新增 npm 包发布：需在 release 流程里加一步（或独立 repo 独立发版——见 §7 开放问题 2）。

**合计：约 5.5–7.5 净开发日**（不含评审）。

---

## 6. 风险清单

| 风险 | 影响 | 缓解 |
|---|---|---|
| **不调用 `next()` 短路审批链** | 破坏用户授权流程（fail-closed → 工具被拒） | §2.1；M1-4 单测锁死；M0-3 前置验证 |
| 在 `agent/created` 抛异常 | 污染会话启动（该点是 await 的） | §2.2 全 `try/catch` |
| 未过滤子 agent | `session_start` 刷屏、聚焦缓存污染 | §2.3；M0-1 先确认判定字段 |
| 误用路线 A 导致只有 2/4 事件生效且**无任何告警** | 用户以为装好，实际漏掉最关键的授权通知 | §1.1 写成文档；README 明确标注交集限制 |
| `dsh` 不在 PATH / 用户自定义 profile | 安装失败或装错 profile | §4.3 三信号检测 + 明确提示；desktop 排除 |
| 插件与 DSH 版本漂移（`0.1.7-alpha.2` 为 alpha） | 事件签名变更导致静默失效 | 插件对未知事件名静默跳过；README 记录适配版本；把「监听器数量」写进集成测试断言 |
| 自行改写 DSH profile 配置 | 与 pnpm/loader 记账冲突 | §4.2：一律走 `dsh plugin` 命令，不手改 `package.json`/`node_modules` |

---

## 7. 开放问题（需决策后再进入实现）

1. **事件面取舍**：`run_completed` 用 `agent/turn-stopping`（精确对应 CC 的 `Stop` 语义）还是 `agent/status` 的 `idle`？前者语义准确，后者更粗。**建议 `agent/turn-stopping`**，但需在 M0 确认它与「等待授权时不算停止」的边界。
2. **npm 包归属**：作为本仓库子目录发布（如 `packages/dsh-plugin/`，随 `agent-notify` 发版），还是独立仓库独立发版？影响 release 流程与版本耦合，需要拍板。
3. **路线 A 是否也要**：是否额外提供「给已有 CC hooks 的用户复用 bridge」的说明文档？**倾向只写文档、不写代码**。
4. **`session_start` 的必要性**：它不发通知、只服务点击聚焦。DSH 的宿主是终端，`DetectSourceApp()` 已能从继承环境变量拿到终端 bundle id，聚焦捕获的价值与既有 agent 相当——但需确认 DSH 的进程环境是否真的透传 `__CFBundleIdentifier`/`TERM_PROGRAM`（插件用 node `spawn` 直起二进制，应继承，M0 顺带验证）。
5. **`docs/` 与 CLAUDE.md 的表述冲突**：CLAUDE.md 称「`docs/` is gitignored」，但 `docs/window-level-focus-design.md` 与本文实际都被 git 跟踪（`.gitignore` 只忽略 `docs/superpowers`）。建议顺手修正 CLAUDE.md 措辞。
