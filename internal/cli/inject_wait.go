package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/hellolib/agent-notify/internal/codexhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
	"github.com/spf13/cobra"
)

// newInjectWaitCmd 创建 inject-wait 子命令。
// 由 hook 进程在退出前启动为后台进程（不 setsid），继承 Codex 的控制终端。
// 轮询 pending 文件，收到 serve 更新的决策后，通过 TIOCSTI 注入按键到终端。
func newInjectWaitCmd(ctx context.Context) *cobra.Command {
	var requestID string
	var timeoutSec int

	cmd := &cobra.Command{
		Use:    "inject-wait",
		Short:  "Internal: wait for remote approval and inject keystroke via TIOCSTI",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath, err := config.StatePath()
			if err != nil {
				return err
			}
			return runInjectWait(ctx, statePath, requestID, timeoutSec)
		},
	}
	cmd.Flags().StringVar(&requestID, "request-id", "", "pending request ID to poll")
	cmd.Flags().IntVar(&timeoutSec, "timeout", config.DefaultTimeoutSec, "max wait seconds")
	_ = cmd.MarkFlagRequired("request-id")
	return cmd
}

// runInjectWait 轮询 pending 文件，收到决策后注入 TIOCSTI 按键。
func runInjectWait(ctx context.Context, statePath, requestID string, timeoutSec int) error {
	logPath, _ := config.LogPath()
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 超时 request_id=%s", requestID))
				return nil
			}
			req, err := state.LoadPending(statePath, requestID)
			if err != nil || req.Status == "pending" {
				continue
			}
			// 请求已被新请求失效 (expired) 或超时 (timeout)，正常退出不注入
			if req.Status == "expired" || req.Status == "timeout" {
				_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 请求已 %s，退出 request_id=%s", req.Status, requestID))
				return nil
			}
			// 收到决策 (approved/denied)，注入按键
			key, ok := keyForApprovalAction(req.Action)
			if !ok {
				_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 未知 action=%s status=%s", req.Action, req.Status))
				return nil
			}
			// 注入按键到控制终端（/dev/tty）
			if err := codexhooks.InjectKeystrokeToTTY(key); err != nil {
				_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 注入按键失败: %v", err))
				return err
			}
			_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 按键注入成功 action=%s key=0x%02x", req.Action, key))
			// 兜底：p/a 可能被 TUI 忽略（2 按钮场景），再注入 y 确保审批通过。
			// 3 按钮时 p/a 已生效菜单关闭，y 会进入输入框，立即注入 backspace 删除。
			// 2 按钮时 y 命中"允许"菜单关闭，backspace 在菜单中被忽略（无害）。
			if req.Action == "allow_prefix" || req.Action == "allow_session" {
				time.Sleep(config.FallbackDelay)
				if err := codexhooks.InjectKeystrokeToTTY('y'); err != nil {
					_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: 兜底 y 注入失败: %v", err))
				} else {
					_ = state.AppendLog(logPath, "inject-wait: 兜底 y 已注入")
					// 立即注入 backspace 删除可能残留在输入框中的 y
					time.Sleep(config.BackspaceDelay)
					if err := codexhooks.InjectKeystrokeToTTY(0x7f); err != nil {
						_ = state.AppendLog(logPath, fmt.Sprintf("inject-wait: backspace 注入失败: %v", err))
					} else {
						_ = state.AppendLog(logPath, "inject-wait: backspace 已注入")
					}
				}
			}
			// 不删除 pending 文件，由 serve 或定时清理负责（避免飞书回调延迟到达时报"已过期"）

			return nil
		}
	}
}

// keyForApprovalAction 将飞书按钮 action 映射为 TIOCSTI 按键字节。
func keyForApprovalAction(action string) (byte, bool) {
	switch action {
	case "allow":
		return 'y', true
	case "allow_prefix":
		return 'p', true
	case "allow_session":
		return 'a', true
	case "reject":
		return 0x1b, true // ESC
	default:
		return 0, false
	}
}
