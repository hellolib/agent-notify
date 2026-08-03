# Toast

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

English | [简体中文](README.zh-CN.md)

Toast is a small Go library for sending desktop notifications from native
applications and lightweight tools. It supports macOS, Windows, and JavaScript
WASM, with helper utilities for Windows toast click-to-focus workflows.

## Features

- Simple `toast.Push` API with functional options
- macOS notifications through `osascript` or Objective-C
- Windows toast notifications through Windows Runtime APIs
- Windows protocol activation for clickable toast actions
- Optional Windows focus helper for bringing the originating terminal or app to front
- JavaScript/WASM notification support for browser environments

## Install

```bash
go get github.com/hellolib/toast
```

## Quick Start

```go
package main

import "github.com/hellolib/toast"

func main() {
    _ = toast.Push("Build finished", toast.WithTitle("Agent Notify"))
}
```

## Platform Examples

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

For Objective-C delivery:

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

To add an image:

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

## Windows Click-To-Focus

Windows toast clicks are delivered through activation. For ordinary Go command
line tools, the practical approach is:

1. Ship a small GUI-subsystem helper executable.
2. Register a custom URL protocol that launches the helper.
3. Send the toast with `WithActivationType("protocol")`.
4. Pass the protocol URI through `WithActivationArguments`.

This repository provides both the library API and a helper command for that
flow. The helper is a separate executable so toast clicks do not flash a console
window.

### Target Selection

`PrepareFocusActivation` resolves the target window before the toast is sent. It
walks up from the supplied PID, finds a visible top-level window, and embeds that
window handle in the activation URI. When the toast is clicked, the helper first
focuses that exact window handle. If the handle is no longer valid, it falls back
to the PID-based process-tree lookup.

This avoids focusing the wrong window in shells launched from apps with multiple
top-level windows, such as VS Code, Windows Terminal, or other Electron-based
hosts.

### Library Usage

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

`PrepareFocusActivation` checks explicit helper candidates first. If none are
provided or found, it looks next to the current executable for conventional
names:

- `toast-focus-helper.exe`
- `toast-focus-helper-arm64.exe`
- `<app>-focus-helper.exe`
- `<app>-helper.exe`

### Demo Commands

`cmd/toast-focus` is a runnable demo that sends a clickable toast. It expects the
helper binary to be in the same directory.

```bash
make build
```

Artifacts are written to `dist/`:

- `toast-focus.exe`
- `toast-focus-helper.exe`
- `toast-focus-arm64.exe`
- `toast-focus-helper-arm64.exe`

For applications that only need the helper binaries:

```bash
make build-helpers
```

## Make Targets

```bash
make test                 # Run Go tests
make build                # Build all Windows demo/helper binaries
make build-helpers        # Build only Windows focus helpers
make build-windows-amd64  # Build Windows amd64 demo/helper
make build-windows-arm64  # Build Windows arm64 demo/helper
make clean                # Remove dist/
```

## API Overview

Common options:

- `toast.Push(message, opts...)`
- `toast.WithTitle(title)`
- `toast.WithMessage(message)`
- `toast.WithAudio(audio)`

macOS options:

- `toast.WithSubtitle(subtitle)`
- `toast.WithObjectiveC()`

Windows options:

- `toast.WithAppID(appID)`
- `toast.WithIcon(path)`
- `toast.WithIconRaw(bytes)`
- `toast.WithActivationType(kind)`
- `toast.WithActivationArguments(uri)`
- `toast.WithProtocolAction(label, uri)`
- `toast.WithLongDuration()`
- `toast.WithShortDuration()`

Windows focus helpers:

- `toast.FindFocusHelper(candidates...)`
- `toast.RegisterFocusProtocol(helperPath, protocol...)`
- `toast.PrepareFocusActivation(pid, helperCandidates...)`
- `toast.FocusActivationArguments(pid, protocol...)`

`FocusActivation.Window` is the resolved Windows `HWND` used for precise
click-to-focus. It may be zero when no suitable window is found before sending
the toast.

JavaScript/WASM options:

- `toast.WithIcon(url)`
- `toast.WithImage(url)`
- `toast.WithRequireInteraction(true)`
- `toast.WithOnClick(fn)`
- `toast.WithOnShow(fn)`
- `toast.WithOnClose(fn)`
- `toast.WithOnError(fn)`

## License

MIT
