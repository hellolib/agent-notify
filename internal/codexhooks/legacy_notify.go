package codexhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	p, err := parseLegacyNotifyPayload(raw)
	if err != nil {
		return notify.Message{}, err
	}
	return legacyNotifyMessage(p), nil
}

func parseLegacyNotifyPayload(raw string) (legacyNotifyPayload, error) {
	var p legacyNotifyPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return legacyNotifyPayload{}, fmt.Errorf("decode Codex notify payload: %w", err)
	}
	if p.Type != "agent-turn-complete" {
		return legacyNotifyPayload{}, fmt.Errorf("unsupported Codex notify event: %s", p.Type)
	}
	return p, nil
}

func legacyNotifyMessage(p legacyNotifyPayload) notify.Message {
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
	}
}

func HandleLegacyNotify(ctx context.Context, cfg config.Config, statePath, logPath, raw string) error {
	p, err := parseLegacyNotifyPayload(raw)
	if err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("skip event: %v", err))
	}
	msg := legacyNotifyMessage(p)

	if isVSCodeNotifyClient(p.Client) {
		codexHome, homeErr := codexHomeDir()
		if homeErr != nil {
			_ = state.AppendLog(logPath, fmt.Sprintf(
				"Codex notify completion verification unavailable client=%s thread=%s turn=%s err=%v",
				p.Client, p.ThreadID, p.TurnID, homeErr,
			))
		} else {
			completion, verifyErr := waitForLegacyNotifyCompletion(
				ctx,
				codexHome,
				p,
				legacyNotifyCompletionWait,
			)
			if verifyErr != nil {
				_ = state.AppendLog(logPath, fmt.Sprintf(
					"Codex notify completion verification failed client=%s thread=%s turn=%s err=%v",
					p.Client, p.ThreadID, p.TurnID, verifyErr,
				))
			} else {
				switch completion.State {
				case legacyTurnRunning:
					return state.AppendLog(logPath, fmt.Sprintf(
						"skip premature Codex notify client=%s thread=%s turn=%s transcript=%s",
						p.Client, p.ThreadID, p.TurnID, completion.TranscriptPath,
					))
				case legacyTurnComplete:
					if hint := truncateMessage(strings.TrimSpace(completion.LastAgentMessage), 200); hint != "" {
						msg.Body = hint
					}
				}
			}
		}
	}
	return agenthooks.Dispatch(ctx, cfg, statePath, logPath, msg)
}

func codexHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Clean(home), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}
