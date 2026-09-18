package omphooks

import (
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload is the small, normalized envelope emitted by the OMP extension.
// Keeping the extension-side payload deliberately small avoids coupling the Go
// binary to OMP's full TypeScript event objects.
type payload struct {
	Type         string `json:"type"`
	SessionID    string `json:"session_id"`
	Workspace    string `json:"cwd"`
	ToolName     string `json:"tool_name"`
	Reason       string `json:"reason"`
	Question     string `json:"question"`
	StopReason   string `json:"stop_reason"`
	ErrorMessage string `json:"error_message"`
}

// ParseMessage parses the normalized event emitted by the OMP extension.
func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	agent := "omp"
	sessionID := strings.TrimSpace(p.SessionID)
	workspace := strings.TrimSpace(p.Workspace)

	switch p.Type {
	case "session_start":
		return notify.Message{
			Agent:     agent,
			Event:     "session_start",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "tool_approval_requested":
		body := "操作需要您的授权许可"
		if reason := strings.TrimSpace(p.Reason); reason != "" {
			body = fmt.Sprintf("工具 %s 需要授权\n原因: %s", fallbackToolName(p.ToolName), common.TruncateRunes(reason, 180))
		} else {
			body = fmt.Sprintf("工具 %s 需要授权", fallbackToolName(p.ToolName))
		}
		return notify.Message{
			Agent:     agent,
			Event:     "permission_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "permission_required"),
			Body:      body,
		}, nil
	case "question_asked":
		// OMP 的 ask 工具阻塞会话等待用户回答——归一化为 input_required。
		body := fmt.Sprintf("工具 %s 需要您的回答", fallbackToolName(p.ToolName))
		if q := strings.TrimSpace(p.Question); q != "" {
			body = fmt.Sprintf("等待您的回答：%s", common.TruncateRunes(q, 120))
		}
		return notify.Message{
			Agent:     agent,
			Event:     "input_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "input_required"),
			Body:      body,
		}, nil
	case "tool_execution_end":
		// 单个工具执行失败不是运行失败。运行成败由 session_stop 携带的
		// stop_reason / error_message 判定，这里静默跳过。
		return notify.Message{}, fmt.Errorf("skip OMP tool execution")
	case "session_stop":
		// stop_hook_active / signal_aborted 已在扩展侧过滤，payload 不再携带。
		if isRunFailure(p.StopReason, p.ErrorMessage) {
			body := "运行失败"
			if errMsg := strings.TrimSpace(p.ErrorMessage); errMsg != "" {
				body = fmt.Sprintf("运行失败：%s", common.TruncateRunes(errMsg, 120))
			}
			return notify.Message{
				Agent:     agent,
				Event:     "run_failed",
				SessionID: sessionID,
				Workspace: workspace,
				Title:     notify.FormatTitle(agent, "run_failed"),
				Body:      body,
			}, nil
		}
		return notify.Message{
			Agent:     agent,
			Event:     "run_completed",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported OMP event: %s", p.Type)
	}
}

// isRunFailure 复刻 OMP 自己的通知层对「运行失败」的判定：最后一条助手消息
// stopReason=="error" 必为失败；=="aborted" 且带错误信息也算失败；
// 其余（end_turn 等）视为正常运行完成。
func isRunFailure(stopReason, errorMessage string) bool {
	switch stopReason {
	case "error":
		return true
	case "aborted":
		return strings.TrimSpace(errorMessage) != ""
	}
	return false
}

func fallbackToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知工具"
	}
	return name
}
