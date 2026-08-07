package opencodehooks

import (
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 opencode 插件通过 stdin 投递的事件 JSON。
// opencode 插件收到的事件结构为 {id, type, properties}，
// 插件把整个 event 对象透传给 handle-opencode-hook 的 stdin。
// properties 内公共字段为 sessionID 和 timestamp，各事件另有自身字段。
type payload struct {
	Type       string     `json:"type"`
	Properties properties `json:"properties"`
}

type properties struct {
	SessionID  string       `json:"sessionID"`
	Timestamp  int64        `json:"timestamp"`
	Directory  string       `json:"directory"`  // session.created 的 info.directory
	Permission string       `json:"permission"` // permission.asked
	Patterns   []string     `json:"patterns"`   // permission.asked
	Status     statusPayload `json:"status"`    // session.status
	Error      errorPayload  `json:"error"`      // session.error
}

type statusPayload struct {
	Type string `json:"type"` // busy / idle
}

type errorPayload struct {
	Message string `json:"message"`
}

func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.Type {
	case "session.created":
		return notify.Message{
			Agent:     "opencode",
			Event:     "session_start",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "permission.asked":
		return notify.Message{
			Agent:     "opencode",
			Event:     "permission_required",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "permission_required"),
			Body:      permissionBody(p.Properties.Permission),
		}, nil
	case "session.status":
		// 仅 idle 状态转发（插件已过滤 busy），表示会话空闲等待用户输入。
		if p.Properties.Status.Type != "idle" {
			return notify.Message{}, fmt.Errorf("skip session.status type=%s", p.Properties.Status.Type)
		}
		return notify.Message{
			Agent:     "opencode",
			Event:     "input_required",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "input_required"),
			Body:      "等待您的输入",
		}, nil
	case "session.idle":
		return notify.Message{
			Agent:     "opencode",
			Event:     "run_completed",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	case "session.error":
		errMsg := strings.TrimSpace(p.Properties.Error.Message)
		if errMsg == "" {
			errMsg = "运行出错"
		}
		return notify.Message{
			Agent:     "opencode",
			Event:     "run_failed",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "run_failed"),
			Body:      fmt.Sprintf("错误: %s", common.TruncateRunes(errMsg, 200)),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported opencode event: %s", p.Type)
	}
}

// permissionBody 构造授权等待通知的正文。
// permission 字段可能来自 OpenCode 的原始请求，长度不可控，截断到 200 runes。
func permissionBody(permission string) string {
	perm := strings.TrimSpace(permission)
	if perm != "" {
		return fmt.Sprintf("权限: %s\n操作需要您的授权许可", common.TruncateRunes(perm, 200))
	}
	return "操作需要您的授权许可"
}
