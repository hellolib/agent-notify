package notify

import "context"

type Message struct {
	Agent     string
	Event     string
	SessionID string
	Workspace string
	Title     string
	Body      string
	SourceApp SourceApp
	// ActivationURI 是点击系统通知时应优先打开的应用协议 URI。它用于已有稳定
	// deep link 的宿主应用；为空时各平台继续使用窗口捕获和进程树定位。
	ActivationURI string
	// FocusWindowID 是 Linux 点击聚焦的目标 X11 窗口 ID（十进制字符串），
	// 由 dispatch 依据 SessionStart 缓存填充；为空则回退按进程树定位。仅 Linux 使用。
	FocusWindowID string
	// FocusCapture 是 macOS / Windows 在 SessionStart 时刻抓取的窗口快照（各平台自己的
	// 原始 JSON），由 dispatch 依据 session 缓存填充。非空时优先用它构造点击载荷，避免
	// 发通知时用户已切走窗口导致抓错（send-time 抓取会取到已漂移的当前焦点窗）。仅 macOS /
	// Windows 使用。
	FocusCapture string
}

// SourceApp 描述触发事件的宿主应用（终端 / IDE），用于系统通知点击后跳转聚焦。
type SourceApp struct {
	BundleID         string // macOS bundle identifier，激活目标（主信号解析结果）
	TermProgram      string // TERM_PROGRAM 原始值，诊断/扩展用
	TerminalEmulator string // TERMINAL_EMULATOR 原始值，诊断/扩展用
}

type Sender interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}
