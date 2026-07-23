package notify

import "context"

// QuestionOption is one of the choices offered by an agent's user-input
// prompt.  It intentionally mirrors Codex's request_user_input schema while
// remaining agent-agnostic so notification channels can render it without
// having to understand hook payloads.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Question describes one question in an input-required notification.
//
// ID, IsOther and IsSecret are kept even for channels that do not currently
// support submitting an answer.  Preserving them lets those channels make a
// useful visual distinction and leaves room for a future answer callback.
type Question struct {
	ID       string           `json:"id,omitempty"`
	Header   string           `json:"header,omitempty"`
	Question string           `json:"question"`
	Options  []QuestionOption `json:"options,omitempty"`
	IsOther  bool             `json:"isOther,omitempty"`
	IsSecret bool             `json:"isSecret,omitempty"`
}

type Message struct {
	Agent     string
	Event     string
	SessionID string
	// DedupeID optionally identifies a single event within a session.  Codex
	// sets it to request_user_input's tool_use_id so two prompts in the same
	// session are delivered independently while duplicate hook invocations for
	// one prompt remain deduplicated.
	DedupeID  string
	Workspace string
	Title     string
	Body      string
	Questions []Question
	// AutoResolutionMs is the optional request_user_input timeout supplied by
	// Codex.  It is kept on the message so channels that support a countdown
	// can render it without having to parse the hook payload again.  A zero
	// value means that Codex did not request automatic resolution.
	AutoResolutionMs int64
	SourceApp        SourceApp
	// FocusWindowID 是 Linux 点击聚焦的目标 X11 窗口 ID（十进制字符串），
	// 由 dispatch 依据 SessionStart 缓存填充；为空则回退按进程树定位。仅 Linux 使用。
	FocusWindowID string
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
