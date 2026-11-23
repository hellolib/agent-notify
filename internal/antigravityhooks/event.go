// Package antigravityhooks parses Google Antigravity hook stdin payloads into
// notify.Message values and manages Antigravity's hooks.json configuration.
//
// Antigravity hooks 文档: https://antigravity.google/docs/hooks
// Antigravity 的 hook 协议与 Claude Code 有两点关键差异：
//  1. 配置文件（~/.gemini/config/hooks.json 或 .agents/hooks.json）顶层按
//     具名 hook 键组织，如 {"agent-notify": {"SessionStart": [...], "PreToolUse": [...], "Stop": [...]}}；
//     其中 SessionStart / Stop 是扁平的 handler 数组，PreToolUse 是带 matcher 的分组数组。
//  2. stdin JSON 字段使用 camelCase（conversationId / workspacePaths / hookEventName /
//     toolCall / terminationReason），且 stdout 必须返回合法 JSON（PreToolUse 和 Stop
//     要求返回带 decision 的对象，否则会被宿主视为 hook 失败）。
package antigravityhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

const agentID = "antigravity"

// payload 描述 Antigravity hooks 通过 stdin 投递的事件 JSON。
type payload struct {
	HookEventName      string    `json:"hookEventName"`
	ConversationID     string    `json:"conversationId"`
	CWD                string    `json:"cwd"`
	WorkspacePaths     []string  `json:"workspacePaths"`
	LastUserInput      string    `json:"lastUserInput"`
	ToolCall           *toolCall `json:"toolCall"`
	StepIdx            *int      `json:"stepIdx"`
	ExecutionNum       *int      `json:"executionNum"`
	TerminationReason  string    `json:"terminationReason"`
	Error              string    `json:"error"`
	FullyIdle          *bool     `json:"fullyIdle"`
	ContinuationActive bool      `json:"continuationActive"`
}

type toolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type askPermissionArgs struct {
	Action string `json:"Action"`
	Target string `json:"Target"`
	Reason string `json:"Reason"`
	// 兼容可能的小写键名。
	ActionLower string `json:"action"`
	TargetLower string `json:"target"`
	ReasonLower string `json:"reason"`
}

type askQuestionArgs struct {
	Questions []struct {
		Question string `json:"question"`
	} `json:"questions"`
}

// ParseMessage 将 Antigravity hook stdin JSON 解析为归一化的 notify.Message。
func ParseMessage(stdin io.Reader) (notify.Message, error) {
	_, msg, err := parseHook(stdin)
	return msg, err
}

// parseHook 解析 stdin JSON，同时返回识别到的 Antigravity hook 事件名，
// 供 Handle 按事件协议输出对应的 JSON 响应（即使事件被静默跳过）。
func parseHook(stdin io.Reader) (string, notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return "", notify.Message{}, err
	}

	eventName := resolveEventName(p)
	sessionID := strings.TrimSpace(p.ConversationID)
	workspace := resolveWorkspace(p)

	switch eventName {
	case "SessionStart":
		// 仅供点击聚焦捕获窗口用；agenthooks.Dispatch 会拦截、不发通知。
		return eventName, notify.Message{
			Agent:     agentID,
			Event:     "session_start",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agentID, "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil

	case "PreToolUse":
		msg, err := parsePreToolUse(p, sessionID, workspace)
		return eventName, msg, err

	case "Stop":
		msg, err := parseStop(p, sessionID, workspace)
		return eventName, msg, err

	default:
		return eventName, notify.Message{}, fmt.Errorf("unsupported antigravity hook event: %s", eventName)
	}
}

// ResponseJSON 返回 Antigravity 对该 hook 事件期望的 stdout JSON 响应。
// PreToolUse 和 Stop 合约要求返回 decision 字段；"allow" 对 PreToolUse 表示放行给
// 下一个 hook 或默认权限策略，对 Stop 表示允许正常终止（仅 "continue" 会阻止停止）。
func ResponseJSON(eventName string) string {
	switch eventName {
	case "PreToolUse", "Stop":
		return `{"decision":"allow"}` + "\n"
	default:
		return "{}\n"
	}
}

// resolveEventName 优先取 payload 自带的 hookEventName；若旧版宿主未传该字段，
// 则按各事件独有字段（toolCall / terminationReason / fullyIdle / executionNum）推断。
func resolveEventName(p payload) string {
	if name := strings.TrimSpace(p.HookEventName); name != "" {
		return name
	}
	if p.ToolCall != nil {
		return "PreToolUse"
	}
	if strings.TrimSpace(p.TerminationReason) != "" || p.FullyIdle != nil || p.ExecutionNum != nil {
		return "Stop"
	}
	if strings.TrimSpace(p.ConversationID) != "" {
		return "SessionStart"
	}
	return ""
}

func resolveWorkspace(p payload) string {
	if cwd := strings.TrimSpace(p.CWD); cwd != "" {
		return cwd
	}
	for _, wp := range p.WorkspacePaths {
		if trimmed := strings.TrimSpace(wp); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// parsePreToolUse 将阻塞等待用户交互的工具调用映射为通知事件：
//   - ask_permission → permission_required
//   - ask_question → input_required
//
// 其它工具静默跳过，避免对普通工具调用产生噪声通知。
func parsePreToolUse(p payload, sessionID, workspace string) (notify.Message, error) {
	if p.ToolCall == nil {
		return notify.Message{}, fmt.Errorf("skip PreToolUse without toolCall")
	}
	toolName := strings.TrimSpace(p.ToolCall.Name)
	switch toolName {
	case "ask_permission":
		return notify.Message{
			Agent:     agentID,
			Event:     "permission_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agentID, "permission_required"),
			Body:      permissionBody(p.ToolCall.Args),
		}, nil

	case "ask_question":
		return notify.Message{
			Agent:     agentID,
			Event:     "input_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agentID, "input_required"),
			Body:      questionBody(p.ToolCall.Args),
		}, nil

	default:
		return notify.Message{}, fmt.Errorf("skip PreToolUse tool: %s", toolName)
	}
}

func parseStop(p payload, sessionID, workspace string) (notify.Message, error) {
	reason := strings.TrimSpace(p.TerminationReason)
	errMsg := strings.TrimSpace(p.Error)

	// 用户主动取消且无错误时静默跳过，避免刚按 Ctrl+C/Esc 就收到一条“任务已完成”。
	if strings.EqualFold(reason, "USER_CANCELED") && errMsg == "" {
		return notify.Message{}, fmt.Errorf("skip Stop: %s", reason)
	}

	if isFailureStop(reason, errMsg) {
		body := "运行失败"
		if errMsg != "" {
			body = fmt.Sprintf("运行失败：%s", common.TruncateRunes(errMsg, 120))
		} else if reason != "" {
			body = fmt.Sprintf("运行失败（%s）", reason)
		}
		return notify.Message{
			Agent:     agentID,
			Event:     "run_failed",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agentID, "run_failed"),
			Body:      body,
		}, nil
	}

	return notify.Message{
		Agent:     agentID,
		Event:     "run_completed",
		SessionID: sessionID,
		Workspace: workspace,
		Title:     notify.FormatTitle(agentID, "run_completed"),
		Body:      notify.DefaultBody("run_completed"),
	}, nil
}

func isFailureStop(reason, errMsg string) bool {
	if errMsg != "" {
		return true
	}
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "ERROR",
		"MAX_INVOCATIONS",
		"MAX_FORCED_INVOCATIONS",
		"MAX_TOKEN_BUDGET_EXCEEDED",
		"MAX_STEPS_EXCEEDED",
		"TERMINAL_CUSTOM_HOOK":
		return true
	default:
		return false
	}
}

func permissionBody(raw json.RawMessage) string {
	var args askPermissionArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &args)
	}
	action := firstNonEmpty(args.Action, args.ActionLower)
	target := firstNonEmpty(args.Target, args.TargetLower)
	reason := firstNonEmpty(args.Reason, args.ReasonLower)

	var parts []string
	switch {
	case action != "" && target != "":
		parts = append(parts, fmt.Sprintf("请求权限: %s %s", action, target))
	case action != "":
		parts = append(parts, fmt.Sprintf("请求权限: %s", action))
	case target != "":
		parts = append(parts, fmt.Sprintf("请求权限: %s", target))
	}
	if reason != "" {
		parts = append(parts, fmt.Sprintf("原因: %s", reason))
	}
	if len(parts) == 0 {
		return "操作需要您的授权许可"
	}
	return common.TruncateRunes(strings.Join(parts, "\n"), 200)
}

func questionBody(raw json.RawMessage) string {
	var args askQuestionArgs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &args)
	}
	for _, q := range args.Questions {
		if text := strings.TrimSpace(q.Question); text != "" {
			return fmt.Sprintf("等待您的回答：%s", common.TruncateRunes(text, 120))
		}
	}
	return "等待您的输入"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
