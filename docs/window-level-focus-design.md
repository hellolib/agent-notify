# 系统通知窗口级聚焦设计（Window-Level Click-to-Focus）

> 状态：设计定稿；**macOS 窗口级**已通过「高级可选」产品路径落地（默认 App 级，用户可开启窗口级）  
> 范围：macOS / Linux / Windows 均支持窗口级；macOS 通过预编译 Swift helper 实现  
> 关联：`docs/superpowers/specs/2026-07-18-mac-focus-precision-design.md`（macOS 聚焦精度 spec）  
> 独立验证原型（归档，不合入）：`../mac-window-focus-demo/`、`../mac-cgs-focus-demo/`

## 1. 目标

用户点击 OS 系统通知后，应尽量回到 **触发该通知的宿主顶层窗口**（终端窗口 / IDE 项目窗口），而不是只把整个应用带到前台、或随机落到同应用的另一个窗口。

典型场景：

| 场景 | 期望 |
|------|------|
| macOS Terminal 开了 3 个窗口，其中一个里的 agent 弹通知 | 点通知回到 **那个窗口** |
| VS Code 开了 3 个项目窗口 | 点通知回到 **对应项目窗口** |
| JetBrains 开了多个项目窗口 | 点通知回到 **对应项目窗口** |

**非目标（明确不做）：**

- 终端内部 tab / pane / split 级定位  
- 依赖 iTerm session API、Kitty remote control、WezTerm CLI 等宿主私有协议做标签页切换  
- 承诺「永远 100% 精确」——同 PID 多窗歧义时允许降级并上报精度  

## 2. 统一模式

三平台窗口级能力本质同一模式，**不是**通知中心事后猜来源：

```text
发通知瞬间（hook 进程仍在宿主进程树内）
  → 解析并固化「触发源」对应的系统级窗口句柄
  → 写入通知点击载荷

用户点击通知
  → 用已保存的句柄激活窗口
  → 句柄失效则降级（App 级 / 静默失败），不重新乱猜
```

| 平台 | 产品路径 | 说明 |
|------|----------|------|
| macOS | **高级可选**：默认 App 级 `open -b`；开启 `focus_precision: window` 后通过 `mac-focus-helper` 实现窗口级 | 两阶段：发送时 `--capture` 获取窗口信息，点击时用保存的句柄定位并 raise |
| Linux/X11 | X11 Window ID + EWMH（待消歧加固） | 继续 |
| Windows | HWND + `anfocus:` helper（待消歧加固） | 继续 |

## 3. 精度分层（只保留两级）

| 级别 | 含义 | 用户体感 |
|------|------|----------|
| **App 级** | 只激活整个应用 | 多窗口时可能落到「随便一个」前台窗 |
| **窗口级** | 激活某个顶层 OS 窗口 | 多项目 / 多终端窗口时回到正确那扇窗 |

> 不做 Tab 级。

```go
type Precision int
const ( PrecisionApp Precision = iota; PrecisionWindow )

type LocateResult struct {
    Level     Precision
    Handle    string  // mac 不用 / X11 id / Windows HWND
    OwnerPID  int
    AppHint   string  // BundleID / 主进程名，降级用
    Ambiguous bool
    Reason    string  // doctor / 日志
}
```

## 4. 「单进程 = 单顶层窗口」前提

PID 反查只有在「该 PID 恰好对应 1 个顶层窗口」时才够用：

```text
1. 拿候选 PID（触发 hook 的进程树祖先，非“当前前台窗”）
2. 枚举该 PID 下所有可见顶层窗
3. 恰好 1 个 → PrecisionWindow，高置信
4. 多个 → 二次消歧
5. 消歧后仍多个 → Ambiguous + best-effort，或降级 App
6. 0 个 → 沿父进程继续 / 降级 App
```

多窗消歧顺序：环境窗口提示（如 Linux `WINDOWID`）→ tty → 工作区/cwd 与窗口标题 → 最近激活 → 降级。**禁止**在消歧失败时假装高精度。

## 5. 分平台设计要点

### 5.1 macOS — 高级可选窗口级（默认 App 级）

#### 5.1.1 产品路径

| 项 | 结论 |
|----|------|
| 默认行为 | `terminal-notifier -execute "open -b <BundleID>"`（App 级），零门槛 |
| 高级可选 | 用户通过「高级功能 → 工作区聚焦精度」菜单开启 `focus_precision: window` |
| 实现方式 | 预编译 Swift helper `mac-focus-helper`（仿 Windows `toast-focus-helper.exe` 模式） |
| 失败策略 | **静默降级到 App 级**，不影响通知送达 |
| 配置范围 | 每个 agent 独立配置，一次设置同时写入 Claude / Codex / ZCode / Grok |

#### 5.1.2 架构（两阶段窗口定位）

```text
发通知时（Go 进程，纯 Go，无 cgo）：
  mac-focus-helper --capture → JSON: {window_id, owner_pid, x, y, w, h, title}
  → 拼入 -execute 载荷

用户点击通知时（terminal-notifier 触发 -execute）：
  mac-focus-helper --owner-pid <pid> --bundle <bid> --window-id <id> --bounds <x,y,w,h>
  → CGWindowList 定位 → AX raise → 失败则 open -b <bid>
```

| 组件 | 职责 |
|------|------|
| `internal/notify/macos.go` | 按 precision 分支 `-execute` 载荷；`captureWindowInfoFromHelper` 获取窗口快照 |
| `thirdparty/helper/mac/mac-focus-helper-{arm64,amd64}` | 预编译 Swift 二进制：`--capture` 获取当前窗口信息，`--owner-pid` 定位并 raise |
| `internal/cli/menu.go` | 高级菜单：聚焦精度选择（默认选中当前值） |
| `internal/app/doctor/` | darwin 诊断：显示当前精度 + helper 可用性 |

#### 5.1.3 配置

```yaml
# 每个 agent 的 channels.system 下
system:
  enabled: true
  click_to_focus: true
  focus_precision: app    # app | window；默认 app
```

- 缺省 / 空 / 未知值 → `app`
- `click_to_focus: false` 时忽略 `focus_precision`
- 非 darwin 上 `window` 按 `app` 处理

#### 5.1.4 降级矩阵

| 条件 | 行为 |
|------|------|
| `focus_precision: app` 或非 darwin | `open -b <BundleID>` |
| `window` 但 helper 不存在 | `open -b`（静默降级） |
| `window` 但 `--capture` 失败 | 回退到 `--owner-pid` 进程树 walk |
| `window` 但点击时无 AX 权限 | helper 内部 `open -b` |
| 无 BundleID 且无法 window | 无 `-execute`，纯展示 |

通知发送本身**不因**聚焦降级而失败。

#### 5.1.5 双授权说明（诚实文案）

macOS TCC 对辅助功能按**进程树归属**算账：在 VS Code / GoLand 等终端中，授权列表可能出现宿主 IDE + terminal-notifier 两条。这是 `-execute`(NSTask fork) 与 AX 调用的机制级耦合，非签名可解。

因此 `focus_precision: window` 定位为**默认关闭的高级选项**，菜单中以灰色说明文字提示此成本。

**为何 Windows 没这问题**：Windows toast `activationType=protocol` + `anfocus:` 协议，点击时**系统按协议拉起独立 helper 进程**（非 toast 发起方的 NSTask 子进程），fork 链被切断，故无「宿主+通知器双授权」。mac 要复刻需重做通知客户端，非签名 terminal-notifier 所能解决。

#### 5.1.5 明确不做（macOS）

- Accessibility / AXRaise 窗口级聚焦  
- 私有 CGS / SkyLight 窗口级（SIP 开不可用）  
- `agent-notify mac-focus` 子命令  
- 重签/公证 terminal-notifier 试图消除授权（无效）  
- 自建签名通知 App + LaunchAgent helper（ROI 不成立）  
- 引导用户开启辅助功能以换窗口级  
- tab / pane / session 级  
- 承诺多项目窗口精确回跳（mac 上产品不提供）

### 5.2 Linux/X11（加固现有窗口级）

**现状优点：**

- 发通知前固化 window id（避免点击时 PID 复用）  
- detached `linux-notify-wait` 等 D-Bus `ActionInvoked`  
- `ewmh.ActiveWindowReq` 激活  

**缺口：**

1. `firstWindowForPID`：同 PID 多窗只取第一个 → 引入 §4 消歧  
2. 焦点窃取防护：现代 WM 可能拒绝非用户触发的激活  
   - 优先 `_NET_ACTIVE_WINDOW`，`source indication = 2`（pager/外部工具语义，与 wmctrl 同类）  
   - 探测 WM（`_NET_SUPPORTING_WM_CHECK`）  
   - 可选：i3/sway 走 IPC（`i3-msg '[id="…"] focus'`）  
   - 再失败：`XSetInputFocus` 硬切（可能只获键盘焦点、视觉未置顶）  
3. Wayland：本设计 **不承诺**；单独评估或 doctor 显示不支持  

**依赖：** 继续优先纯 Go（`xgb` / `xgbutil` / godbus），避免强依赖 `xdotool` 二进制。

### 5.3 Windows（加固现有窗口级）

**现状优点：**

- Toast `activationType=protocol` + `anfocus:`  
- 发通知时可带 HWND：`anfocus:<pid>:<hwnd_hex>`  
- helper 优先用保存 HWND，失效再按 PID 找窗  
- **fork 链被协议切断**（系统按 `anfocus:` 拉起独立 helper，非 toast 子进程），无 mac 式双授权  

**缺口：**

1. 同 PID 多 HWND：实现 §4 消歧后再写入 launch  
2. 前台锁：`SetForegroundWindow` 可能失败 → best-effort（helper 已有 AttachThreadInput 等手法）  
3. doctor 继续检测 `toast-focus-helper` 是否可找到  

**不在范围：** Windows Terminal 标签页（同一 HWND 内 UI，OS 不可见）。

## 6. 现状（agent-notify 代码）

| 平台 | 展示 | 点击聚焦 | 实际精度 | 产品结论 | 关键代码 |
|------|------|----------|----------|----------|----------|
| macOS | terminal-notifier → osascript | `-execute "open -b <BundleID>"` | **App 级** | **保持；不做窗口级** | `internal/notify/macos.go`、`sourceapp.go` |
| Linux | D-Bus Notify / notify-send | `linux-notify-wait` + `ActiveWindowReq` | 窗口级尽力（缺消歧） | 继续加固 | `internal/notify/linux.go`、`internal/linuxfocus/` |
| Windows | WinRT Toast (PowerShell) | `anfocus:pid[:hwnd]` + `toast-focus-helper` | 窗口级尽力（缺消歧） | 继续加固 | `internal/notify/windows*.go`、`hellolib/toast` |

配置：

```yaml
channels:
  system:
    enabled: true
    click_to_focus: true   # false 时三平台均不挂激活逻辑
```

## 7. 接口与集成

```go
type WindowLocator interface {
    // Locate 在「发通知瞬间」调用，基于当前进程树与 msg 上下文。
    // 不得依赖“用户点击时”再解析作为唯一路径。
    Locate(ctx context.Context, hint LocateHint) (LocateResult, error)
}
type LocateHint struct {
    StartPID  int
    Workspace string
    AppHint   string // BundleID / TermProgram
}
```

| 组件 | 变化 |
|------|------|
| `DetectSourceApp` | 保留；mac App 级点击需要 BundleID |
| `MacOSSender` | **保持** `open -b`；**不**引入 AX / CGS / mac-focus |
| `LinuxSender` / `linuxfocus` | `ResolveWindowID` 升级为可消歧的 Locate |
| `WindowsSender` / toast | `PrepareFocusActivation` 前先 Locate，多窗写入消歧后的 HWND |
| `click_to_focus: false` | 行为不变 |
| doctor | mac：terminal-notifier；Win：toast-focus-helper；Linux：既有探测 |

**载荷原则：**

- Linux/Windows：发通知时固化触发源窗口句柄；点击优先消费载荷，不靠「当前前台」重猜  
- macOS：只固化 BundleID（App 级）；**不**写 CGWindowID/bounds/AX 载荷  
- 句柄失效（L/W）：校验失败则降级或静默，**不**用过期 PID 乱绑新窗  

## 8. 实现分期

### P0 — 精度模型 + Linux/Windows 多窗消歧（优先）

- 引入 `Precision` / `LocateResult`  
- Linux：替换/增强 `firstWindowForPID`，多窗走 §4  
- Windows：同 PID 多 HWND 消歧后再写 `anfocus:`  
- 日志：`level=window|app ambiguous=… reason=…`  
- 单测：单窗唯一、多窗 + cwd 消歧、多窗无法消歧  

**收益：** 修 VS Code / WT / GNOME 等同 PID 多窗误聚焦（非 mac）。

### P1 — macOS：无窗口级合入（已关闭）

- **不做** `internal/macfocus`、`mac-focus` 子命令、AX/CGS、helper  
- 保持现有 App 级 `open -b` + terminal-notifier  
- 文档与 doctor 不承诺 mac 多窗精确回跳  

### P2 — Linux 激活链路加固

- WM 探测 + `_NET_ACTIVE_WINDOW` source=2  
- i3/sway IPC 可选优先  
- 焦点窃取失败时的行为可配置或文档说明  
- Wayland：明确不支持或另开设计  

### 明确不做（本设计生命周期内）

- **macOS 任何形式的窗口级聚焦（AX / 私有 CGS / 自建通知客户端）**  
- Tab / pane / split 级聚焦  
- 为每个终端写 session 级特判  
- 保证绕过所有 OS 前台锁 / 专注模式  
- 要求用户为点击聚焦开启辅助功能（mac）  

## 9. 测试矩阵

| 用例 | 平台 | 期望 | 说明 |
|------|------|------|------|
| mac 点击通知 | mac | **App 级**回宿主 BundleID | 回归现网行为 |
| mac 无 BundleID | mac | 无 `-execute` | 纯展示 |
| 单终端单窗口 | Linux/Win | Window | 基线 |
| 同 App 两窗口，仅一侧触发 | Linux/Win | Window 或 Ambiguous | 核心多窗 |
| VS Code 两项目窗口 | Linux/Win | Window（消歧） | 高价值 |
| `click_to_focus: false` | 全平台 | 无激活 | 回归 |
| 触发窗已关闭再点击 | Linux/Win | 不误聚焦到无关窗 | 校验失效 |
| Linux 无 EWMH/无窗 | Linux | 纯展示 | 降级不 panic |
| Windows 无 helper | Windows | 纯 Toast | 与现状一致 |

## 10. 风险与约束

| 风险 | 缓解 |
|------|------|
| 同 PID 多窗消歧仍失败（L/W） | `Ambiguous` + 日志；不谎称精确 |
| mac 多窗无法精确回跳 | **接受**；产品仅 App 级，文档写明 |
| mac 再评估窗口级 | 三条路已实测否决；除非 Apple 改 TCC/提供免授权窗口激活 API，否则不再评估 |
| WM / Windows 前台锁 | best-effort；文档不写「保证置顶」 |
| Wayland | 范围外；doctor 标明 |
| 句柄绑定过早、窗后来重建（L/W） | 点击时校验；失效则降级 |

## 11. 决策摘要

1. **不做 tab 级。**  
2. **Linux/Windows**：继续「发通知绑触发源窗 → 点击激活」+ 多窗消歧。  
3. **macOS**：**弃用窗口级（AX/CGS/自建通知客户端全否决）**；产品 **仅 App 级** `open -b BundleID`。  
4. **mac 否决根因**：terminal-notifier `-execute` 经 NSTask fork 子进程执行点击命令；窗口级需调 AX，AX 调用挂在 `宿主→terminal-notifier→sh→helper` fork 链上；macOS TCC 按进程树归属 → 宿主与通知器双授权；签名无法切断 fork 链；切断需自建独立通知客户端 + 协议拉起 helper（类 Windows `anfocus:`），ROI 不成立。  
5. **落地顺序**：P0 Linux/Windows 消歧 → P2 Linux 激活加固；**无 mac 窗口级 P1**。  
6. **产品话术**：mac 点击回到对应应用；Linux/Windows 尽量回到对应窗口（best-effort）。

---

## 附录 A：mac 否决依据详表（2026-07-18 实测）

### A.1 AX 路径实测

| 步骤 | 结果 |
|------|------|
| demo `mac-window-focus-demo`（AX + bounds 对齐） | Terminal 双窗技术上可 `result=WINDOW` |
| 在 VSCode 终端跑 notify → 点击 | 辅助功能列表出现 **VSCode + terminal-notifier** 两条 |
| 给 terminal-notifier 重签（ad-hoc + 稳定 identifier） | 列表仍出现两条；**签名无效** |
| 在 GoLand 终端跑 | 同样双授权（GoLand + terminal-notifier） |

**结论**：双授权来自 `-execute` fork 链 + TCC 进程树归属，非签名可解。

### A.2 私有 CGS 路径实测

| 私有符号 | 状态 |
|----------|------|
| `CGSMainConnectionID` / `CGSOrderWindow` / `CGSSetFrontWindow` / `SLPSSetFrontProcessWithOptions` / `SLPSFindProcessByPID` / `SLPSGetWindowOwner` | 全部 `dlsym` 成功 |
| `SLPSSetFrontProcessWithOptions` | err=0（进程前置，应用级） |
| `CGSSetFrontWindow` | err=0 |
| `CGSOrderWindow(Above)` | **err=1000（kCGErrorFailure）** |

环境：macOS 26.5.2，**SIP enabled**。

**结论**：SIP 开时 `CGSOrderWindow` 对他人窗口失败，仅应用级；窗口级需关 SIP（yabai 模式），不可作产品要求。demo 见 `../mac-cgs-focus-demo/`。

### A.3 独立 helper / LaunchAgent 路径

理论可切断 fork 链（点击经协议/launchctl 拉起不属于 terminal-notifier 子进程的独立 helper），实现单授权。但需自建签名通知客户端（`UNUserNotificationCenter`）+ 安装/更新/卸载链路 + 多版本矩阵，对通知工具 ROI 不成立。

### A.4 terminal-notifier 机制（源码反推）

- 通知：废弃 `NSUserNotificationCenter`  
- 点击回调：`userActivatedNotification:` → `executeShellCommand:` → **`NSTask` `/bin/sh -c`**  
- 身份：`FakeBundleIdentifier` 冒充 `com.apple.Terminal`  
- 本机分发版本：**未签名**（`code object is not signed at all`）；重签不改变 fork 链归属  

## 附录 B：关键代码索引

| 路径 | 作用 |
|------|------|
| `internal/notify/sender.go` | `NewSystemSender` 分发 |
| `internal/notify/sourceapp.go` | mac BundleID 等宿主识别 |
| `internal/notify/macos.go` | terminal-notifier / **`open -b`（mac 最终产品路径）** |
| `internal/notify/linux.go` | Linux 发送与 focus starter |
| `internal/linuxfocus/focus_linux.go` | window id 解析、wait、激活 |
| `internal/cli/linux_notify_wait.go` | 隐藏子命令等点击 |
| `internal/notify/windows.go` / `windows_toast_*.go` | Toast XML 与 launch |
| `github.com/hellolib/toast` | `PrepareFocusActivation` / helper 协议 |
| `internal/config/config.go` | `click_to_focus` |
| `internal/agenthooks/dispatch.go` | `DetectSourceApp` + 构造 System Sender |
| `internal/app/doctor/` | 点击聚焦 helper 探测 |

## 附录 C：独立验证原型（归档，不合入）

| 原型 | 位置 | 结论 |
|------|------|------|
| AX + bounds 对齐 | `../mac-window-focus-demo/` (Swift) | 技术可窗口级；**产品否决**（双授权） |
| 私有 CGS | `../mac-cgs-focus-demo/` (Swift) | SIP 开仅应用级；**产品否决** |

二者仅证明「技术可行性」与「授权/稳定性不可接受」，**不作为合入依据**。若未来 Apple 提供免辅助功能的窗口激活公开 API，再开新设计评审。
