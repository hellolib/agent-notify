package codexhooks

import (
	"context"
	"fmt"
	"io"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
)

func Handle(ctx context.Context, cfg config.Config, statePath, logPath string, stdin io.Reader) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("read stdin error: %v", err))
		return nil
	}

	msg, err := ParseMessage(data)
	if err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
		return nil
	}
	// A PreToolUse hook can be invoked directly (or by a Codex version with a
	// broader matcher) for tools other than request_user_input. ParseMessage
	// represents those events as an empty message; do not dispatch or emit a
	// misleading "no sender" log entry for them.
	if msg.Event == "" {
		return nil
	}

	// Hook notifications are best-effort. A failed remote/system sender (or a
	// local state/logging problem) must never turn into a failing Codex hook
	// process, because PreToolUse uses the hook's result to decide whether the
	// tool may continue. Dispatch already records sender errors; swallow them here
	// after the best-effort log so Codex can proceed.
	if err := agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("best-effort dispatch failed: %v", err))
	}
	return nil
}
