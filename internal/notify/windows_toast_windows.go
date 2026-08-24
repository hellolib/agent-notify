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

	// per-agent AppUserModelID：toast 的 attribution row 图标绑定 AppUserModelID
	// （由 toast 库注册为该 AppID 的 IconUri），不同 agent 要显示不同 head 栏图标
	// 就必须用不同 AppID。已知 agent 用可读名（兼作 AUMID 与 DisplayName，head 栏
	// 显示 agent 名）；未知 agent 回退 "agent-notify"。
	appID := "agent-notify"
	if name := appDisplayName(req.Agent); name != "" && name != req.Agent {
		appID = name
	}

	opts := []toast.NotificationOption{
		toast.WithAppID(appID),
		toast.WithTitle(req.Title),
		toast.WithMessage(req.Body),
		toast.WithAudio(toast.Default),
		toast.WithLongDuration(),
	}

	// per-agent attribution 图标：找到 logo 才注入（toast 库将其注册为该 AppID
	// 的 IconUri，显示在 head 栏）；找不到静默回退 toast 库默认终端图标。
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
		} else if args, reason, ok := focusArgumentsFromWorkspace(req); ok {
			opts = append(opts,
				toast.WithActivationType("protocol"),
				toast.WithActivationArguments(args),
			)
			if req.FocusDebug && req.LogPath != "" {
				appendFocusSendDiag(req.LogPath, "[send] used workspace title: "+reason+" args="+args+"\n")
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

// focusArgumentsFromWorkspace 处理 Codex IDE/app-server 没有 SessionStart 缓存的场景。
// 多个 VS Code 窗口共用 Code.exe 时，用 notify payload 的 cwd 唯一匹配窗口标题。
func focusArgumentsFromWorkspace(req windowsToastRequest) (args, reason string, ok bool) {
	chain := toast.EnumerateAncestorWindows(uint32(os.Getppid()))
	hwnd, _, reason := selectWorkspaceWindow(chain, req.Workspace)
	if hwnd == 0 {
		return "", reason, false
	}
	act, err := toast.FocusActivationForWindow(os.Getppid(), hwnd, req.LogPath)
	if err != nil {
		return "", err.Error(), false
	}
	return act.Arguments, reason, true
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
