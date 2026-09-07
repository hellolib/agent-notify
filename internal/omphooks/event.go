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
	Type           string `json:"type"`
	SessionID      string `json:"session_id"`
	Workspace      string `json:"cwd"`
	ToolName       string `json:"tool_name"`
	Reason         string `json:"reason"`
	IsError        bool   `json:"is_error"`
	StopHookActive bool   `json:"stop_hook_active"`
	SignalAborted  bool   `json:"signal_aborted"`
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
	case "tool_execution_end":
		if !p.IsError {
			return notify.Message{}, fmt.Errorf("skip successful OMP tool execution")
		}
		return notify.Message{
			Agent:     agent,
			Event:     "run_failed",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "run_failed"),
			Body:      fmt.Sprintf("工具 %s 执行失败", fallbackToolName(p.ToolName)),
		}, nil
	case "session_stop":
		if p.StopHookActive {
			return notify.Message{}, fmt.Errorf("skip OMP session stop while stop hook is active")
		}
		if p.SignalAborted {
			return notify.Message{}, fmt.Errorf("skip aborted OMP session stop")
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

func fallbackToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知工具"
	}
	return name
}
