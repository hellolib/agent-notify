package config

import "time"

// 远程审批相关常量。
const (
	// DefaultTimeoutSec 默认审批等待超时（3天），覆盖 inject-wait / inject-daemon / ensureInjectDaemon。
	DefaultTimeoutSec = 259200

	// PollInterval inject-wait 轮询 pending 文件的间隔。
	PollInterval = 500 * time.Millisecond

	// DaemonPollInterval inject-daemon 轮询注入队列的间隔。
	DaemonPollInterval = 300 * time.Millisecond

	// FallbackDelay 注入 p/a 后等待 TUI 响应再注入兜底 y 的延迟。
	FallbackDelay = 150 * time.Millisecond

	// BackspaceDelay 注入 y 后等待再注入 backspace 的延迟。
	BackspaceDelay = 50 * time.Millisecond

	// InjectByteDelay 逐字节注入文本时的间隔。
	InjectByteDelay = 2 * time.Millisecond

	// InjectEnterDelay 注入文本后到注入回车之间的延迟。
	InjectEnterDelay = 5 * time.Millisecond

	// DefaultListenAddr serve 默认监听地址。
	DefaultListenAddr = "127.0.0.1:7896"
)
