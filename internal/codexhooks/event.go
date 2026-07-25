package codexhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Codex hooks 通过 stdin 投递的事件 JSON。
// 字段与 Codex 官方 hook schema 对齐，未使用的字段也保留以便排查。
type payload struct {
	HookEventName        string          `json:"hook_event_name"`
	SessionID            string          `json:"session_id"`
	CWD                  string          `json:"cwd"`
	Model                string          `json:"model"`
	PermissionMode       string          `json:"permission_mode"`
	TurnID               string          `json:"turn_id"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	ToolUseID            string          `json:"tool_use_id"`
	StopHookActive       bool            `json:"stop_hook_active"`
	LastAssistantMessage string          `json:"last_assistant_message"`
}

// requestUserInputArgs mirrors the arguments of Codex's request_user_input
// tool.  Raw messages are used for questions and autoResolutionMs so we can
// distinguish an omitted field from a malformed value and fail closed without
// ever starting a notification for a partially decoded prompt.
type requestUserInputArgs struct {
	Questions        json.RawMessage `json:"questions"`
	AutoResolutionMs json.RawMessage `json:"autoResolutionMs"`
}

type requestUserInputQuestion struct {
	ID       string                   `json:"id"`
	Header   string                   `json:"header"`
	Question string                   `json:"question"`
	IsOther  bool                     `json:"isOther"`
	IsSecret bool                     `json:"isSecret"`
	Options  []requestUserInputOption `json:"options"`
}

type requestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func ParseMessage(data []byte) (notify.Message, error) {
	var p payload
	if err := json.Unmarshal(data, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.HookEventName {
	case "SessionStart":
		// 仅供 Linux 点击聚焦捕获窗口用；Dispatch 会拦截、不发通知。
		return notify.Message{
			Agent:     "codex",
			Event:     "session_start",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "PermissionRequest":
		return notify.Message{
			Agent:     "codex",
			Event:     "permission_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "permission_required"),
			Body:      fmt.Sprintf("工具: %s\n操作需要您的授权许可", fallbackToolName(p.ToolName)),
		}, nil
	case "PreToolUse", "pre_tool_use":
		// The matcher in hooks.json already limits this hook to
		// request_user_input. Keep the check here as a second line of defence:
		// users may invoke the handler directly, and Codex can add other
		// PreToolUse tools over time. Ignored events return an empty message so
		// callers can safely no-op without treating them as hook failures.
		if p.ToolName != "request_user_input" {
			return notify.Message{}, nil
		}
		return parseRequestUserInput(p)
	case "Stop":
		// Codex Stop means that one agent turn stopped. It also fires when a
		// planning turn finishes and Codex is waiting for the user to start
		// execution, so it cannot reliably represent task completion. Treat legacy
		// Stop subscriptions as a no-op; new installations no longer subscribe.
		return notify.Message{}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

func parseRequestUserInput(p payload) (notify.Message, error) {
	if len(p.ToolInput) == 0 || string(p.ToolInput) == "null" {
		return notify.Message{}, fmt.Errorf("request_user_input tool_input is missing")
	}

	var args requestUserInputArgs
	if err := json.Unmarshal(p.ToolInput, &args); err != nil {
		return notify.Message{}, fmt.Errorf("decode request_user_input tool_input: %w", err)
	}
	if len(args.Questions) == 0 || string(args.Questions) == "null" {
		return notify.Message{}, fmt.Errorf("request_user_input questions are missing")
	}

	var rawQuestions []requestUserInputQuestion
	if err := json.Unmarshal(args.Questions, &rawQuestions); err != nil {
		return notify.Message{}, fmt.Errorf("decode request_user_input questions: %w", err)
	}
	if len(rawQuestions) == 0 {
		return notify.Message{}, fmt.Errorf("request_user_input questions are missing")
	}

	questions := make([]notify.Question, 0, len(rawQuestions))
	for _, question := range rawQuestions {
		var options []notify.QuestionOption
		if question.Options != nil {
			options = make([]notify.QuestionOption, 0, len(question.Options))
			for _, option := range question.Options {
				options = append(options, notify.QuestionOption{
					Label:       option.Label,
					Description: option.Description,
				})
			}
		}
		questions = append(questions, notify.Question{
			ID:       question.ID,
			Header:   question.Header,
			Question: question.Question,
			Options:  options,
			IsOther:  question.IsOther,
			IsSecret: question.IsSecret,
		})
	}

	autoResolutionMs, err := parseAutoResolutionMs(args.AutoResolutionMs)
	if err != nil {
		return notify.Message{}, err
	}

	return notify.Message{
		Agent:            "codex",
		Event:            "input_required",
		SessionID:        p.SessionID,
		DedupeID:         p.ToolUseID,
		Workspace:        p.CWD,
		Title:            notify.FormatTitle("codex", "input_required"),
		Body:             requestUserInputBody(questions, autoResolutionMs),
		Questions:        questions,
		AutoResolutionMs: autoResolutionMs,
	}, nil
}

func parseAutoResolutionMs(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}

	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("decode request_user_input autoResolutionMs: %w", err)
	}
	if value > uint64(^uint64(0)>>1) {
		return 0, fmt.Errorf("request_user_input autoResolutionMs overflows int64")
	}
	return int64(value), nil
}

func requestUserInputBody(questions []notify.Question, autoResolutionMs int64) string {
	var b strings.Builder
	b.WriteString("Codex 正在等待您的输入")
	for i, question := range questions {
		b.WriteString(fmt.Sprintf("\n%d. ", i+1))
		if question.Header != "" {
			b.WriteString(question.Header)
			b.WriteString(": ")
		}
		if question.Question != "" {
			b.WriteString(question.Question)
		} else {
			b.WriteString("（未提供问题文本）")
		}
		if question.IsSecret {
			b.WriteString(" [敏感输入]")
		}
		for _, option := range question.Options {
			b.WriteString("\n   - ")
			b.WriteString(option.Label)
			if option.Description != "" {
				b.WriteString(": ")
				b.WriteString(option.Description)
			}
		}
		if question.IsOther {
			b.WriteString("\n   - 其他（自由输入）")
		}
	}
	if autoResolutionMs > 0 {
		b.WriteString(fmt.Sprintf("\n自动处理时间：%d 毫秒", autoResolutionMs))
	}
	b.WriteString("\n请回到 Codex 终端提交答案")
	return b.String()
}

func fallbackToolName(name string) string {
	if name == "" {
		return "未知工具"
	}
	return name
}
