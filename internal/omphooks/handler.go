package omphooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

// Handle reads one normalized OMP event from stdin and dispatches it through
// the same notification pipeline as all other supported agents.
func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	msg, err := ParseMessage(stdin)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}

	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
