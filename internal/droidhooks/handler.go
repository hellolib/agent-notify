package droidhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// Handle 读取 Droid 通过 stdin 投递的 hook 事件并分发通知。
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	msg, err := ParseMessage(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
