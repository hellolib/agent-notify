package dshhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// Handle 读取 DSH 插件投递的一个事件并送入统一的派发管线。
//
// 与其它 agent 的 handler 同构：解析失败只写日志、返回 nil——hook 绝不能
// 让被它观测的 agent 失败。
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	msg, err := ParseMessage(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
