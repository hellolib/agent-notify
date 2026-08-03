package codexhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

// runRemoteApproval 处理远程审批流程（非阻塞模式）：
//  1. 生成 request_id
//  2. 写 pending 请求
//  3. 直接发送飞书通知卡片（按钮数量由 event.go 启发式推断）
//  4. 启动 inject-wait 后台子进程（继承控制终端，轮询 pending 文件）
//  5. 立即返回空字符串 → Codex 回退终端审批菜单
//
// inject-wait 子进程收到 serve 更新的决策后，通过 TIOCSTI 注入按键到终端。
func runRemoteApproval(ctx context.Context, cfg config.Config, statePath, logPath string, msg notify.Message, payloadData []byte) (string, error) {
	requestID := msg.RequestID
	if requestID == "" {
		requestID = uuid.NewString()
		msg.RequestID = requestID
	}

	var p payload
	_ = json.Unmarshal(payloadData, &p)

	// 获取控制终端路径，供 serve 注入非审批文本
	ttyPath, _ := FindControllingTTY()

	// 写 pending 请求（历史卡片在用户点击新卡片按钮后由 serve 失效）
	req := state.PendingRequest{
		RequestID: requestID,
		SessionID: msg.SessionID,
		ToolName:  fallbackToolName(p.ToolName),
		Workspace: msg.Workspace,
		Body:      msg.Body,
		Detail:    msg.Detail,
		TtyPath:   ttyPath,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	if err := state.SavePending(statePath, req); err != nil {
		return "", state.AppendLog(logPath, fmt.Sprintf("remote_approval: save pending error: %v", err))
	}

	// 在 Body 末尾追加 requestID（不可见），使每次去重键不同，
	// 避免同一 session 内连续的 permission_required 事件被去重跳过
	msg.Body = msg.Body + "\n#" + requestID[:8]

	// 直接发送飞书卡片
	if err := agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("remote_approval: dispatch error: %v", err))
	} else {
		_ = state.AppendLog(logPath, fmt.Sprintf("remote_approval: 卡片已发送 session=%s buttons=%d", msg.SessionID, len(msg.Actions)))
	}

	// 启动 inject-wait 后台子进程（继承 hook 的控制终端）
	// 它会轮询 pending 文件，收到 serve 决策后通过 TIOCSTI 注入按键
	selfExe, err := os.Executable()
	if err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("remote_approval: os.Executable error: %v", err))
	} else {
		waitCmd := exec.CommandContext(context.Background(), selfExe, "inject-wait",
			"--request-id", requestID,
			"--timeout", fmt.Sprintf("%d", config.DefaultTimeoutSec),
		)
		waitCmd.Stdin = nil
		waitCmd.Stdout = nil
		waitCmd.Stderr = nil
		// 不 setsid：子进程继承 hook 的会话和控制终端
		waitCmd.SysProcAttr = nil

		if err := waitCmd.Start(); err != nil {
			_ = state.AppendLog(logPath, fmt.Sprintf("remote_approval: inject-wait 启动失败: %v", err))
		}
		// 不等待子进程退出，hook 主进程立即返回
		if waitCmd.Process != nil {
			_ = waitCmd.Process.Release()
		}

		// 启动 inject-daemon 后台子进程（继承 hook 的控制终端）
		// 它常驻轮询 inject_queue/<session_id>/ 目录，发现新文本时通过 TIOCSTI 注入到终端
		daemonCmd := exec.CommandContext(context.Background(), selfExe, "inject-daemon",
			"--session-id", msg.SessionID,
			"--timeout", fmt.Sprintf("%d", config.DefaultTimeoutSec),
		)
		daemonCmd.Stdin = nil
		daemonCmd.Stdout = nil
		daemonCmd.Stderr = nil
		daemonCmd.SysProcAttr = nil
		_ = daemonCmd.Start()
		if daemonCmd.Process != nil {
			_ = daemonCmd.Process.Release()
		}
	}

	// 返回空：Codex 回退终端审批菜单，TUI 正常弹出
	return "", nil
}
