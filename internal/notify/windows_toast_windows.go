//go:build windows

package notify

import (
	"context"
	"os"

	"github.com/hellolib/agent-notify/internal/winfocus"
	"github.com/hellolib/toast"
)

// defaultWindowsToastPush renders + shows the toast through the toast library
// (which owns the CJK-safe Base64 PowerShell push and the default terminal
// logo). agent-notify only orchestrates the click-to-focus activation.
func defaultWindowsToastPush(ctx context.Context, req windowsToastRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	opts := []toast.NotificationOption{
		toast.WithAppID("agent-notify"),
		toast.WithTitle(req.Title),
		toast.WithMessage(req.Body),
		toast.WithAudio(toast.Default),
		toast.WithLongDuration(),
	}

	// per-agent logo：找到图标才注入；找不到静默走 toast 库默认图标
	// （文档约定：logo 找不到时静默走系统默认）。
	if iconPath := AgentLogoPath(req.Agent); iconPath != "" {
		opts = append(opts, toast.WithIcon(iconPath))
	}

	if req.ClickToFocus {
		// 优先用 SessionStart 缓存的精确窗口（复核句柄仍可用且标题一致）；命中即写入
		// 带该 HWND 的 anfocus:，点击直接回到那扇窗——这是多 WT 窗口定位准的关键路径。
		if args, ok := focusArgumentsFromCapture(req); ok {
			opts = append(opts,
				toast.WithActivationType("protocol"),
				toast.WithActivationArguments(args),
			)
			if req.FocusDebug && req.LogPath != "" {
				appendFocusSendDiag(req.LogPath, "[send] used SessionStart cache: "+args+"\n")
			}
		} else if act, diag, err := toast.PrepareFocusActivationVerbose(os.Getppid(), req.LogPath); err == nil {
			// 兜底：缓存 miss / 失效 / 指纹不符 → 按进程树选窗（多窗时可能不精确）。
			opts = append(opts,
				toast.WithActivationType("protocol"),
				toast.WithActivationArguments(act.Arguments),
			)
			if req.FocusDebug && req.LogPath != "" {
				appendFocusSendDiag(req.LogPath, diag.String())
			}
		}
	}

	return toast.Push(req.Body, opts...)
}

// focusArgumentsFromCapture 命中 SessionStart 缓存、且缓存句柄经复核仍可用并标题一致时，
// 返回带该 HWND 的 anfocus: 激活参数；miss / 句柄失效 / 指纹不符（防 M2 回收复用）时 ok=false，
// 调用方据此退回进程树兜底。pid 仍传 os.Getppid()，供 helper 在点击时句柄失效的重走兜底。
func focusArgumentsFromCapture(req windowsToastRequest) (string, bool) {
	if req.FocusCapture == "" {
		return "", false
	}
	hwnd, title, ok := winfocus.Decode(req.FocusCapture)
	if !ok || !winfocus.IsUsableAndMatches(hwnd, title) {
		return "", false
	}
	act, err := toast.FocusActivationForWindow(os.Getppid(), hwnd, req.LogPath)
	if err != nil {
		return "", false
	}
	return act.Arguments, true
}

// appendFocusSendDiag 把 send 诊断写入与 helper 同一日志文件；失败一律吞掉。
func appendFocusSendDiag(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text)
}
