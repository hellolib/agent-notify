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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

const (
	agentID = "antigravity"
	// maxTranscriptTailBytes 限制读取 transcript.jsonl 尾部的最大字节数，
	// 避免超长会话日志在 hook 进程里占用过多内存或拖慢响应。
	maxTranscriptTailBytes = 256 * 1024
)

// payload 描述 Antigravity hooks 通过 stdin 投递的事件 JSON。
type payload struct {
	HookEventName      string    `json:"hookEventName"`
	ConversationID     string    `json:"conversationId"`
	CWD                string    `json:"cwd"`
	WorkspacePaths     []string  `json:"workspacePaths"`
	TranscriptPath     string    `json:"transcriptPath"`
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
		Body:      completedBody(p),
	}, nil
}

// completedBody 为 Stop 完成事件生成尽量具体的通知正文：
//  1. 优先从 transcriptPath 读取本轮最后一条 PLANNER_RESPONSE 的可见回复内容（与 Codex
//     的 last_assistant_message 体验一致，按 rune 截断至 200 字）；
//  2. 若读不到本轮回复，退回展示 lastUserInput 摘要；
//  3. 都没有时退回通用默认文案。
func completedBody(p payload) string {
	if reply := extractLastAssistantMessage(p.TranscriptPath); reply != "" {
		return common.TruncateRunes(reply, 200)
	}
	if prompt := strings.TrimSpace(p.LastUserInput); prompt != "" {
		return fmt.Sprintf("任务已完成：%s", common.TruncateRunes(prompt, 120))
	}
	return notify.DefaultBody("run_completed")
}

type transcriptEntry struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// extractLastAssistantMessage 从 Antigravity transcript.jsonl 尾部反向扫描，
// 提取当前轮次（最近一次 USER_INPUT 之后）最后一条含非空 content 的 PLANNER_RESPONSE。
func extractLastAssistantMessage(path string) string {
	path = expandHomePath(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ""
	}

	offset := int64(0)
	if info.Size() > maxTranscriptTailBytes {
		offset = info.Size() - maxTranscriptTailBytes
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return ""
		}
	}

	buf, err := io.ReadAll(io.LimitReader(f, maxTranscriptTailBytes))
	if err != nil || len(buf) == 0 {
		return ""
	}
	// 如果是从文件中间开始读的，第一段可能是不完整的半行，直接丢弃。
	if offset > 0 {
		if idx := bytes.IndexByte(buf, '\n'); idx >= 0 {
			buf = buf[idx+1:]
		} else {
			return ""
		}
	}

	lines := bytes.Split(buf, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type == "USER_INPUT" {
			// 已回溯到本轮用户输入边界，不再跨轮复用历史回复。
			return ""
		}
		if entry.Type == "PLANNER_RESPONSE" {
			if text := strings.TrimSpace(entry.Content); text != "" {
				return text
			}
		}
	}
	return ""
}

func expandHomePath(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if p[1] == '/' || p[1] == '\\' {
		return filepath.Join(home, p[2:])
	}
	return p
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
