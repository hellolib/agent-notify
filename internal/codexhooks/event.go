package codexhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Codex hooks 通过 stdin 投递的事件 JSON。
// 字段与 Codex 官方 hook schema 对齐，未使用的字段也保留以便排查。
type payload struct {
	HookEventName        string          `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	CWD                  string          `json:"cwd"`
	Model                string          `json:"model"`
	PermissionMode       string          `json:"permission_mode"`
	TurnID               string          `json:"turn_id"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`       // 容错:形态依工具而异(issue #32)
	StopHookActive       json.RawMessage `json:"stop_hook_active"` // 容错:接受 bool 或 "true"/"false"
	LastAssistantMessage string          `json:"last_assistant_message"`
}

func ParseMessage(data []byte) (notify.Message, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.HookEventName {
	case "SessionStart":
		// 仅供 Linux 点击聚焦捕获窗口用；Dispatch 会拦截、不发通知。
		return notify.Message{
			Agent:     "codex",
			Event:     "session_start",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "PermissionRequest":
		return notify.Message{
			Agent:     "codex",
			Event:     "permission_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "permission_required"),
			Body:      fmt.Sprintf("工具: %s\n操作需要您的授权许可", fallbackToolName(p.ToolName)),
		}, nil
	case "Stop":
		body := notify.DefaultBody("run_completed")
		if hint := truncateMessage(strings.TrimSpace(p.LastAssistantMessage), 200); hint != "" {
			body = hint
		}
		return notify.Message{
			Agent:     "codex",
			Event:     "run_completed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "run_completed"),
			Body:      body,
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

func fallbackToolName(name string) string {
	if name == "" {
		return "未知工具"
	}
	return name
}

// truncateMessage 按 rune 截断,CJK 不产生半个字符(issue #33)。
func truncateMessage(msg string, limit int) string {
	return common.TruncateRunes(msg, limit)
}
