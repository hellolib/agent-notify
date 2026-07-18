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
// focusDebug 控制点击聚焦探针日志开关，目前仅 windows 采纳（内嵌 logpath 到 toast URI 并记录 send 诊断），
// macOS/linux 暂忽略。
func NewSystemSender(run Runner, clickToFocus bool, focusPrecision string, focusDebug bool) Sender {
	switch runtime.GOOS {
	case "darwin":
		return NewMacOSSender(run, clickToFocus, focusPrecision)
	case "linux":
		return NewLinuxSender(run, clickToFocus)
	case "windows":
		return NewWindowsSender(run, clickToFocus, focusDebug)
	default:
		return NewUnsupportedSender(runtime.GOOS)
	}
}
