package claudehooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hellolib/agent-notify/internal/notify"
)

type payload struct {
	HookEventName string         `json:"hook_event_name"`
	SessionID     string         `json:"session_id"`
	CWD           string         `json:"cwd"`
	Message       string         `json:"message"`
	ToolName      string         `json:"tool_name"`
	ToolResponse  map[string]any `json:"tool_response"`
	ToolInput     map[string]any `json:"tool_input"`
}

func ParseMessage(data []byte) (notify.Message, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.HookEventName {
	case "PermissionRequest":
		return notify.Message{
			Agent:     "claude_code",
			Event:     "permission_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			ToolName:  p.ToolName,
			Title:     notify.FormatTitle("claude_code", "permission_required"),
			Body:      fmt.Sprintf("工具: %s\n操作需要您的授权许可", p.ToolName),
		}, nil
	case "Notification":
		if isInputRequiredNotification(p.Message) {
			// Extract a cleaner hint from the message
			hint := extractInputHint(p.Message)
			return notify.Message{
				Agent:     "claude_code",
				Event:     "input_required",
				SessionID: p.SessionID,
				Workspace: p.CWD,
				Title:     notify.FormatTitle("claude_code", "input_required"),
				Body:      fmt.Sprintf("提示: %s", hint),
			}, nil
		}
		return notify.Message{}, fmt.Errorf("unsupported notification message: %s", p.Message)
	case "PostToolUse":
		return notify.Message{
			Agent:     "claude_code",
			Event:     notify.EventToolCompleted,
			SessionID: p.SessionID,
			Workspace: p.CWD,
			ToolName:  p.ToolName,
		}, nil
	case "UserPromptSubmit":
		return notify.Message{
			Agent:     "claude_code",
			Event:     notify.EventInputSubmitted,
			SessionID: p.SessionID,
			Workspace: p.CWD,
		}, nil
	case "Stop":
		return notify.Message{
			Agent:     "claude_code",
			Event:     "run_completed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("claude_code", "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	case "PostToolUseFailure":
		errMsg := extractErrorMessage(p.ToolResponse)
		return notify.Message{
			Agent:     "claude_code",
			Event:     "run_failed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			ToolName:  p.ToolName,
			Title:     notify.FormatTitle("claude_code", "run_failed"),
			Body:      fmt.Sprintf("工具: %s\n错误: %s", p.ToolName, errMsg),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

// extractInputHint extracts a cleaner hint from the notification message
func isInputRequiredNotification(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(msg, "waiting for your input") ||
		strings.Contains(msg, "waiting for input") ||
		strings.HasPrefix(msg, "needs input")
}

func extractInputHint(msg string) string {
	// Try to extract meaningful content after common prefixes
	msg = strings.TrimSpace(msg)

	// Remove common prefixes
	prefixes := []string{
		"claude is waiting for your input",
		"waiting for your input: ",
		"waiting for input: ",
		"needs input: ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(msg), prefix) {
			return strings.TrimSpace(msg[len(prefix):])
		}
	}

	// If message is too long, truncate it
	if len(msg) > 100 {
		return msg[:97] + "..."
	}

	return msg
}

// extractErrorMessage extracts error message from tool response
func extractErrorMessage(response map[string]any) string {
	if response == nil {
		return "未知错误"
	}

	if err, ok := response["error"]; ok {
		if errStr, ok := err.(string); ok && errStr != "" {
			if len(errStr) > 200 {
				return errStr[:197] + "..."
			}
			return errStr
		}
	}

	if err, ok := response["message"]; ok {
		if errStr, ok := err.(string); ok && errStr != "" {
			if len(errStr) > 200 {
				return errStr[:197] + "..."
			}
			return errStr
		}
	}

	return "操作失败"
}
