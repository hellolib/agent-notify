package zcodehooks

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 ZCode hooks 通过 stdin 投递的事件 JSON。
// ZCode 与 Claude Code 的 hook 协议高度兼容，但会同时下发驼峰与下划线
// 两套字段名（hookEventName / hook_event_name），这里两者都解析。
// 未使用的字段也保留以便排查。
type payload struct {
	HookEventName      string          `json:"hook_event_name"` // 下划线风格（优先）
	HookEventNameCamel string          `json:"hookEventName"`   // 驼峰风格（兜底）
	SessionID          string          `json:"session_id"`
	SessionIDCamel     string          `json:"sessionId"`
	CWD                string          `json:"cwd"`
	Mode               string          `json:"mode"`   // 例如 yolo / plan
	Source             string          `json:"source"` // 例如 resume / new
	Model              string          `json:"model"`
	PermissionMode     string          `json:"permission_mode"`
	TurnID             string          `json:"turn_id"`
	ToolName           string          `json:"tool_name"`
	StopHookActive     json.RawMessage `json:"stop_hook_active"` // 容错:接受 bool 或 "true"/"false"
	Message            string          `json:"message"`          // Notification 原因（ZCode 当前无此事件，保留）
}

// eventOf 兼容下划线与驼峰两套字段名。
func (p payload) eventOf() string {
	if p.HookEventName != "" {
		return p.HookEventName
	}
	return p.HookEventNameCamel
}

// sessionOf 兼容下划线与驼峰两套字段名。
func (p payload) sessionOf() string {
	if p.SessionID != "" {
		return p.SessionID
	}
	return p.SessionIDCamel
}

func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	event := p.eventOf()
	switch event {
	case "SessionStart":
		return notify.Message{
			Agent:     "zcode",
			Event:     "session_start",
			SessionID: p.sessionOf(),
			Workspace: p.CWD,
			Title:     notify.FormatTitle("zcode", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "PermissionRequest":
		return notify.Message{
			Agent:     "zcode",
			Event:     "permission_required",
			SessionID: p.sessionOf(),
			Workspace: p.CWD,
			Title:     notify.FormatTitle("zcode", "permission_required"),
			Body:      fmt.Sprintf("工具: %s\n操作需要您的授权许可", fallbackToolName(p.ToolName)),
		}, nil
	case "PostToolUseFailure":
		tool := fallbackToolName(p.ToolName)
		return notify.Message{
			Agent:     "zcode",
			Event:     "run_failed",
			SessionID: p.sessionOf(),
			Workspace: p.CWD,
			Title:     notify.FormatTitle("zcode", "run_failed"),
			Body:      fmt.Sprintf("工具 %s 执行失败", tool),
		}, nil
	case "Stop":
		return notify.Message{
			Agent:     "zcode",
			Event:     "run_completed",
			SessionID: p.sessionOf(),
			Workspace: p.CWD,
			Title:     notify.FormatTitle("zcode", "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", event)
	}
}

func fallbackToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知工具"
	}
	return name
}
