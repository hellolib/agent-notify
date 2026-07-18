package notify

import "runtime"

// NewSystemSender returns the appropriate system notification sender for the current platform.
// - darwin: uses macOS notifications (osascript/terminal-notifier)
// - linux: uses notify-send
// - windows: uses Windows Runtime toast notifications
// - other: returns an explicit unsupported sender
//
// clickToFocus 控制点击通知是否激活宿主应用（macOS/Linux/Windows 生效）。
// focusPrecision 控制激活宿主应用时的聚焦精度（"app" | "window"），目前仅 macOS 采纳，
// linux/windows 暂忽略，留待后续实现。
func NewSystemSender(run Runner, clickToFocus bool, focusPrecision string) Sender {
	switch runtime.GOOS {
	case "darwin":
		return NewMacOSSender(run, clickToFocus, focusPrecision)
	case "linux":
		return NewLinuxSender(run, clickToFocus)
	case "windows":
		return NewWindowsSender(run, clickToFocus)
	default:
		return NewUnsupportedSender(runtime.GOOS)
	}
}
