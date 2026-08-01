package notify

import (
	"context"
	"strings"
	"time"
)

type windowsToastRequest struct {
	Title string
	Body  string
	// Agent 是触发通知的 agent 名（claude_code/codex/zcode/grok/droid 等），
	// 用于经 icon.go 的 AgentLogoPath 解析 per-agent logo 图标路径。
	Agent        string
	ClickToFocus bool
	FocusDebug   bool
	LogPath      string
	// FocusCapture 是 SessionStart 缓存的窗口快照 JSON（winfocus {"hwnd","title"}）；
	// 命中且复核通过时用它拼 anfocus:，否则退回进程树兜底。macOS / Windows 发送端分别
	// 消费自己的快照格式。
	FocusCapture string
}

type windowsToastFunc func(ctx context.Context, req windowsToastRequest) error

type WindowsSender struct {
	push         windowsToastFunc
	clickToFocus bool
	focusDebug   bool
}

func NewWindowsSender(_ Runner, clickToFocus, focusDebug bool) *WindowsSender {
	return &WindowsSender{push: defaultWindowsToastPush, clickToFocus: clickToFocus, focusDebug: focusDebug}
}

func NewWindowsSenderWithPusher(push windowsToastFunc, clickToFocus, focusDebug bool) *WindowsSender {
	return &WindowsSender{push: push, clickToFocus: clickToFocus, focusDebug: focusDebug}
}

func (s *WindowsSender) Name() string { return "system" }

func (s *WindowsSender) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.push(ctx, windowsToastRequest{
		Title:        msg.Title,
		Body:         s.formatBody(msg),
		Agent:        msg.Agent,
		ClickToFocus: s.clickToFocus,
		FocusDebug:   s.focusDebug,
		LogPath:      focusHelperLogPath(s.focusDebug),
		FocusCapture: msg.FocusCapture,
	})
}

func (s *WindowsSender) formatBody(msg Message) string {
	timestamp := time.Now().Format("15:04:05")
	parts := make([]string, 0, 3)
	// Prefer a shortened path (last two segments) so CJK project folders remain
	// readable and long drive paths do not dominate the toast body.
	if msg.Workspace != "" {
		parts = append(parts, shortenWorkspace(msg.Workspace))
	}
	if msg.Body != "" {
		parts = append(parts, msg.Body)
	}
	parts = append(parts, timestamp)
	return strings.Join(parts, "\n")
}
