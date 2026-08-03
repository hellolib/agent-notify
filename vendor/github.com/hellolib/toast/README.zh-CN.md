# Toast

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[English](README.md) | 简体中文

Toast 是一个用于发送桌面通知的 Go 小型库，适合原生命令行工具、桌面辅助程序和轻量应用。它支持 macOS、Windows 和 JavaScript WASM，并提供 Windows 通知点击后聚焦原窗口的辅助能力。

## 功能特性

- 简洁的 `toast.Push` API 和函数式选项
- macOS 支持 `osascript` 和 Objective-C 通知
- Windows 使用 Windows Runtime Toast Notification API
- Windows 支持通过 URL Protocol 处理通知点击
- 可选的 Windows focus helper，用于点击通知后聚焦原终端或应用窗口
- JavaScript/WASM 环境下支持浏览器通知

## 安装

```bash
go get github.com/hellolib/toast
```

## 快速开始

```go
package main

import "github.com/hellolib/toast"

func main() {
    _ = toast.Push("Build finished", toast.WithTitle("Agent Notify"))
}
```

## 平台示例

### macOS

```go
package main

import "github.com/hellolib/toast"

func main() {
    _ = toast.Push("Permission required",
        toast.WithTitle("Agent Notify"),
        toast.WithSubtitle("15:04:05"),
        toast.WithAudio(toast.Submarine),
    )
}
```

使用 Objective-C 方式发送：

```go
_ = toast.Push("Task completed",
    toast.WithTitle("Agent Notify"),
    toast.WithObjectiveC(),
)
```

### Windows

```go
package main

import "github.com/hellolib/toast"

func main() {
    _ = toast.Push("Task completed",
        toast.WithAppID("agent-notify"),
        toast.WithTitle("Agent Notify"),
        toast.WithAudio(toast.Default),
        toast.WithLongDuration(),
    )
}
```

添加图标：

```go
_ = toast.Push("Task completed",
    toast.WithTitle("Agent Notify"),
    toast.WithIcon(`C:\path\to\icon.png`),
)
```

### JavaScript / WASM

```go
package main

import (
    "fmt"

    "github.com/hellolib/toast"
)

func main() {
    _ = toast.Push("Saved",
        toast.WithTitle("WASM App"),
        toast.WithOnClick(func(event interface{}) {
            fmt.Println("clicked")
        }),
        toast.WithOnClose(func() {
            fmt.Println("closed")
        }),
    )
}
```

## Windows 点击聚焦

Windows toast 的点击行为通过 activation 触发。对普通 Go 命令行工具来说，比较稳定的方案是：

1. 随应用分发一个 GUI 子系统的 helper 可执行文件。
2. 注册一个自定义 URL Protocol，让系统在点击通知时启动 helper。
3. 发送通知时使用 `WithActivationType("protocol")`。
4. 通过 `WithActivationArguments` 传入协议 URI。

本仓库同时提供库 API 和 helper 命令。helper 是独立可执行文件，因此点击通知时不会闪出控制台窗口。

### 目标窗口选择

`PrepareFocusActivation` 会在发送 toast 之前先解析目标窗口。它会从传入的 PID 开始沿父进程树向上查找可见的顶层窗口，并把窗口句柄写入 activation URI。点击通知时，helper 会优先聚焦这个精确的窗口句柄；如果句柄已经失效，再回退到基于 PID 的进程树查找。

这个策略可以避免在 VS Code、Windows Terminal 或其他 Electron 宿主里打开 shell 时，点击通知却聚焦到错误窗口的问题。

### 库用法

```go
package main

import (
    "fmt"
    "os"

    "github.com/hellolib/toast"
)

func main() {
    focus, err := toast.PrepareFocusActivation(
        os.Getppid(),
        `C:\path\to\toast-focus-helper.exe`,
    )
    if err != nil {
        panic(err)
    }

    _ = toast.Push("Click to focus the current terminal",
        toast.WithAppID("agent-notify"),
        toast.WithTitle("Agent Notify"),
        toast.WithMessage(fmt.Sprintf("helper: %s\nhwnd: 0x%x", focus.Helper, focus.Window)),
        toast.WithActivationType("protocol"),
        toast.WithActivationArguments(focus.Arguments),
    )
}
```

`PrepareFocusActivation` 会优先检查传入的 helper 候选路径。如果没有传入路径，或路径不存在，它会在当前可执行文件同目录下查找这些约定名称：

- `toast-focus-helper.exe`
- `toast-focus-helper-arm64.exe`
- `<app>-focus-helper.exe`
- `<app>-helper.exe`

### 示例命令

`cmd/toast-focus` 是一个可运行示例，用于发送可点击聚焦的通知。helper 需要和它放在同一个目录。

```bash
make build
```

产物会生成到 `dist/`：

- `toast-focus.exe`
- `toast-focus-helper.exe`
- `toast-focus-arm64.exe`
- `toast-focus-helper-arm64.exe`

如果上层应用只需要内置 helper：

```bash
make build-helpers
```

## Make 目标

```bash
make test                 # 运行 Go 测试
make build                # 构建全部 Windows demo/helper 二进制
make build-helpers        # 只构建 Windows focus helper
make build-windows-amd64  # 构建 Windows amd64 demo/helper
make build-windows-arm64  # 构建 Windows arm64 demo/helper
make clean                # 删除 dist/
```

## API 概览

通用选项：

- `toast.Push(message, opts...)`
- `toast.WithTitle(title)`
- `toast.WithMessage(message)`
- `toast.WithAudio(audio)`

macOS 选项：

- `toast.WithSubtitle(subtitle)`
- `toast.WithObjectiveC()`

Windows 选项：

- `toast.WithAppID(appID)`
- `toast.WithIcon(path)`
- `toast.WithIconRaw(bytes)`
- `toast.WithActivationType(kind)`
- `toast.WithActivationArguments(uri)`
- `toast.WithProtocolAction(label, uri)`
- `toast.WithLongDuration()`
- `toast.WithShortDuration()`

Windows 聚焦辅助：

- `toast.FindFocusHelper(candidates...)`
- `toast.RegisterFocusProtocol(helperPath, protocol...)`
- `toast.PrepareFocusActivation(pid, helperCandidates...)`
- `toast.FocusActivationArguments(pid, protocol...)`

`FocusActivation.Window` 是解析到的 Windows `HWND`，用于更精确地点击聚焦。如果发送通知前没有找到合适窗口，它可能为 0。

JavaScript/WASM 选项：

- `toast.WithIcon(url)`
- `toast.WithImage(url)`
- `toast.WithRequireInteraction(true)`
- `toast.WithOnClick(fn)`
- `toast.WithOnShow(fn)`
- `toast.WithOnClose(fn)`
- `toast.WithOnError(fn)`

## 许可证

MIT
