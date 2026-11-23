package antigravityhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// Handle 读取 Antigravity 通过 stdin 投递的 hook 事件，分发通知并向 stdout 写出
// 符合 Antigravity hook 合约的 JSON 响应。
//
// Antigravity 会解析 hook 进程的 stdout JSON：即使事件被跳过或发送失败，也必须写出
// 合法 JSON（且不能阻塞/阻断 Agent 执行）。
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader, stdout io.Writer) error {
	eventName, msg, err := parseHook(stdin)
	if stdout != nil {
		_, _ = io.WriteString(stdout, ResponseJSON(eventName))
	}
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
