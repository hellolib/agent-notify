// Package droidhooks parses Droid (Factory) hook stdin payloads into notify.Message values.
//
// Droid hooks 文档: https://docs.factory.ai/harness/hooks
// Droid 与 Claude Code 共享相似的 hook stdin JSON 结构（hook_event_name / session_id / cwd），
// 但 Notification 事件额外携带 notification_type 字段（permission_prompt / idle_prompt /
// auth_success / elicitation_dialog）。仅 permission_prompt 和 idle_prompt 映射为通知，
// 其它类型（auth_success / elicitation_dialog）静默丢弃。
package droidhooks

import (
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Droid hooks 通过 stdin 投递的事件 JSON。
type payload struct {
	HookEventName    string `json:"hook_event_name"`
	SessionID        string `json:"session_id"`
	CWD              string `json:"cwd"`
	Message          string `json:"message"`
	NotificationType string `json:"notification_type"`
}

func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.HookEventName {
	case "SessionStart":
		// 仅供点击聚焦捕获窗口用；Dispatch 会拦截、不发通知。
		return notify.Message{
			Agent:     "droid",
			Event:     "session_start",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("droid", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "Notification":
		return parseNotification(p)
	case "Stop":
		return notify.Message{
			Agent:     "droid",
			Event:     "run_completed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("droid", "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

// parseNotification 根据 notification_type 分发通知类型。
// Droid Notification 事件携带 notification_type:
//   - permission_prompt → permission_required
//   - idle_prompt → input_required
//   - auth_success / elicitation_dialog → 静默丢弃（返回 error）
func parseNotification(p payload) (notify.Message, error) {
	notifType := strings.ToLower(strings.TrimSpace(p.NotificationType))

	switch notifType {
	case "permission_prompt":
		return notify.Message{
			Agent:     "droid",
			Event:     "permission_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("droid", "permission_required"),
			Body:      permissionBody(p.Message),
		}, nil
	case "idle_prompt":
		hint := extractInputHint(p.Message)
		return notify.Message{
			Agent:     "droid",
			Event:     "input_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("droid", "input_required"),
			Body:      fmt.Sprintf("提示: %s", hint),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported notification type: %s", p.NotificationType)
	}
}

func permissionBody(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg != "" {
		return msg
	}
	return "操作需要您的授权许可"
}

// extractInputHint 从 Notification message 中提取简洁的等待输入提示。
func extractInputHint(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return "等待您的操作"
	}
	prefixes := []string{
		"droid is waiting for your input",
		"waiting for your input",
		"waiting for input",
		"needs input",
	}
	lower := strings.ToLower(msg)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			rest := strings.TrimSpace(msg[len(prefix):])
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				return rest
			}
		}
	}
	return common.TruncateRunes(msg, 100)
}
