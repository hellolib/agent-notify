//go:build windows

package notify

import (
	"context"
	"os"

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
		// No WithIcon → toast falls back to its built-in default terminal icon.
	}

	if req.ClickToFocus {
		if act, diag, err := toast.PrepareFocusActivationVerbose(os.Getppid(), req.LogPath); err == nil {
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

// appendFocusSendDiag 把 send 诊断写入与 helper 同一日志文件；失败一律吞掉。
func appendFocusSendDiag(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text)
}
