package codexhooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("read stdin error: %v", err))
	}

	msg, err := ParseMessage(data)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	// PermissionRequest 且远程审批启用时，走阻塞等待 + stdout 决策流程
	if msg.Event == "permission_required" && cfg.RemoteApproval.Enabled {
		decision, err := runRemoteApproval(ctx, cfg, statePath, logPath, msg, data)
		if err != nil {
			return err
		}
		if decision != "" {
			// 输出决策 JSON 到 stdout，Codex 解析后直接放行/拒绝
			fmt.Fprintln(os.Stdout, decision)
		}
		return nil
	}

	// 非审批事件：保存详情到 pending 文件（status=info），供详情按钮回调加载
	if msg.RequestID != "" && msg.Detail != "" {
		infoReq := state.PendingRequest{
			RequestID: msg.RequestID,
			SessionID: msg.SessionID,
			ToolName:  "info",
			Workspace: msg.Workspace,
			Body:      msg.Body,
			Detail:    msg.Detail,
			Status:    "info",
			CreatedAt: time.Now(),
		}
		_ = state.SavePending(statePath, infoReq)
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
