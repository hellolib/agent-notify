package codexhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

// legacyNotifyPayload 是 Codex 顶层 notify 命令收到的 agent-turn-complete JSON。
// IDE/app-server 客户端未必会执行交互式 hooks trust 流程，因此保留这条官方兼容入口。
type legacyNotifyPayload struct {
	Type                 string   `json:"type"`
	ThreadID             string   `json:"thread-id"`
	TurnID               string   `json:"turn-id"`
	CWD                  string   `json:"cwd"`
	Client               string   `json:"client"`
	InputMessages        []string `json:"input-messages"`
	LastAssistantMessage *string  `json:"last-assistant-message"`
}

func ParseLegacyNotify(raw string) (notify.Message, error) {
	var p legacyNotifyPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return notify.Message{}, fmt.Errorf("decode Codex notify payload: %w", err)
	}
	if p.Type != "agent-turn-complete" {
		return notify.Message{}, fmt.Errorf("unsupported Codex notify event: %s", p.Type)
	}

	body := notify.DefaultBody("run_completed")
	if p.LastAssistantMessage != nil {
		if hint := truncateMessage(strings.TrimSpace(*p.LastAssistantMessage), 200); hint != "" {
			body = hint
		}
	}

	return notify.Message{
		Agent:     "codex",
		Event:     "run_completed",
		SessionID: p.ThreadID,
		Workspace: p.CWD,
		Title:     notify.FormatTitle("codex", "run_completed"),
		Body:      body,
	}, nil
}

func HandleLegacyNotify(ctx context.Context, cfg config.Config, statePath, logPath, raw string) error {
	msg, err := ParseLegacyNotify(raw)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}
	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}
