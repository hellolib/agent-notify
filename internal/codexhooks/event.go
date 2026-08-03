package codexhooks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 Codex hooks 通过 stdin 投递的事件 JSON。
// 字段与 Codex 官方 hook schema 对齐，未使用的字段也保留以便排查。
type payload struct {
	HookEventName        string         `json:"hook_event_name"`
	SessionID            string         `json:"session_id"`
	CWD                  string         `json:"cwd"`
	Model                string         `json:"model"`
	PermissionMode       string         `json:"permission_mode"`
	TurnID               string         `json:"turn_id"`
	ToolName             string         `json:"tool_name"`
	ToolInput            map[string]any `json:"tool_input"`
	StopHookActive       bool           `json:"stop_hook_active"`
	LastAssistantMessage string         `json:"last_assistant_message"`
	AvailableDecisions   []string       `json:"available_decisions"`
	TranscriptPath       string         `json:"transcript_path"`
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
			RequestID: uuid.NewString(),
			Detail:    buildDetail(p),
		}, nil
	case "PermissionRequest":
		return notify.Message{
			Agent:     "codex",
			Event:     "permission_required",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "permission_required"),
			Body:      permissionBody(p),
			Actions:   permissionActionsFromDecisions(p.AvailableDecisions, p.ToolName, p.PermissionMode, extractCommand(p.ToolInput)),
			Detail:    buildDetail(p),
			RequestID: uuid.NewString(),
		}, nil
	case "Stop":
		body := notify.DefaultBody("run_completed")
		if hint := truncateMessage(strings.TrimSpace(p.LastAssistantMessage), 200); hint != "" {
			body = hint
		}
		return notify.Message{
			Agent:     "codex",
			Event:     "run_completed",
			SessionID: p.SessionID,
			Workspace: p.CWD,
			Title:     notify.FormatTitle("codex", "run_completed"),
			Body:      body,
			RequestID: uuid.NewString(),
			Detail:    buildDetail(p),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported hook event: %s", p.HookEventName)
	}
}

// permissionBody 从 tool_input 中提取命令内容，构造审批卡片的 Body。
// 优先展示命令内容；无法提取时回退到默认提示。
func permissionBody(p payload) string {
	toolName := fallbackToolName(p.ToolName)
	cmd := extractCommand(p.ToolInput)
	if cmd != "" {
		return fmt.Sprintf("工具: %s\n命令: %s", toolName, cmd)
	}
	return fmt.Sprintf("工具: %s\n操作需要您的授权许可", toolName)
}

// extractCommand 从 tool_input 中尝试提取 command 字段（Bash/Shell 工具）。
func extractCommand(toolInput map[string]any) string {
	if toolInput == nil {
		return ""
	}
	if cmd, ok := toolInput["command"].(string); ok && cmd != "" {
		return truncateMessage(cmd, 300)
	}
	return ""
}

// permissionActionsFromDecisions 根据 Codex hook payload 中的 available_decisions
// 生成与 TUI 完全匹配的审批按钮。
//
// 决策字符串映射（来自 Codex 的 ReviewDecision::to_opaque_string）：
//   - "approved"                  → 允许（注入 y）
//   - "approved_with_amendment"   → 允许(不再询问)（注入 p）
//   - "approved_for_session"      → 允许(不再询问)（注入 a）
//   - "abort"                     → 拒绝（注入 ESC）
//
// 当 available_decisions 为空时（旧版 Codex），回退到基于 tool_name/mode 的推断。
func permissionActionsFromDecisions(decisions []string, toolName, mode, command string) []notify.Action {
	if len(decisions) > 0 {
		return actionsFromDecisions(decisions)
	}
	return fallbackActions(toolName, mode, command)
}

func actionsFromDecisions(decisions []string) []notify.Action {
	var actions []notify.Action
	for _, d := range decisions {
		switch d {
		case "approved":
			actions = append(actions, notify.Action{Label: "允许", Value: "allow", Style: "primary"})
		case "approved_with_amendment":
			actions = append(actions, notify.Action{Label: "允许(不再询问)", Value: "allow_prefix", Style: "default"})
		case "approved_for_session":
			actions = append(actions, notify.Action{Label: "允许(不再询问)", Value: "allow_session", Style: "default"})
		case "abort":
			actions = append(actions, notify.Action{Label: "拒绝", Value: "reject", Style: "danger"})
		}
	}
	if len(actions) == 0 {
		actions = []notify.Action{
			{Label: "允许", Value: "allow", Style: "primary"},
			{Label: "拒绝", Value: "reject", Style: "danger"},
		}
	}
	return actions
}

// fallbackActions 在 available_decisions 为空时（官方 Codex）推断按钮数量。
//
// 官方 Codex 的 hook payload 不含 available_decisions，需根据命令内容推断 TUI 按钮数。
// 推断逻辑复刻 Codex 的 default_available_decisions:
// execpolicy 修正生成条件:
//   - 命令可被前缀提取 (非复杂解析，即非 heredoc-only)
//   - 命令未匹配现有 allow 规则 (hook 触发即说明未匹配)
//
// 按钮映射:
//   - apply_patch: TUI 固定 3 按钮 → y(allow) + a(allow_session) + esc(reject)
//   - Bash + heredoc-only → 复杂解析，无修正 → 2 按钮 → y(allow) + esc(reject)
//   - Bash 其他 → 有修正 → 3 按钮 → y(allow) + p(allow_prefix) + esc(reject)
//   - 非 default 模式 → 2 按钮
func fallbackActions(toolName, mode, command string) []notify.Action {
	allow := notify.Action{Label: "允许", Value: "allow", Style: "primary"}
	reject := notify.Action{Label: "拒绝", Value: "reject", Style: "danger"}

	if mode != "default" {
		return []notify.Action{allow, reject}
	}

	switch toolName {
	case "apply_patch", "applypatch":
		return []notify.Action{
			allow,
			{Label: "允许(不再询问)", Value: "allow_session", Style: "default"},
			reject,
		}
	case "Bash", "bash":
		// heredoc-only → 复杂解析，无 execpolicy 修正 → TUI 2 按钮
		if isHeredocOnlyCommand(command) {
			return []notify.Action{allow, reject}
		}
		// 简单命令 → 有 execpolicy 修正 → TUI 3 按钮
		return []notify.Action{
			allow,
			{Label: "允许(不再询问)", Value: "allow_prefix", Style: "default"},
			reject,
		}
	default:
		return []notify.Action{allow, reject}
	}
}

// isHeredocOnlyCommand 检测命令是否为"仅含 heredoc 无文件重定向的单条命令"。
// 对应 Codex parse_shell_lc_single_command_prefix 的判定条件:
//  1. 有 heredoc_redirect(<< 或 <<-，不是 <<<)
//  2. 无 file_redirect(> >> <，不含 <<)
//  3. 单条命令(无 && || ; | 分隔符)
//
// 满足以上条件时 used_complex_parsing=true，TUI 不显示 execpolicy 修正按钮(2 按钮)。
func isHeredocOnlyCommand(command string) bool {
	hasHeredoc := false
	hasFileRedirect := false
	hasSeparator := false

	inSingleQuote := false
	inDoubleQuote := false

	runes := []rune(command)
	i := 0
	for i < len(runes) {
		c := runes[i]

		// 跳过引号内的内容
		if inSingleQuote {
			if c == '\'' {
				inSingleQuote = false
			}
			i++
			continue
		}
		if inDoubleQuote {
			if c == '"' {
				inDoubleQuote = false
			}
			i++
			continue
		}
		if c == '\'' {
			inSingleQuote = true
			i++
			continue
		}
		if c == '"' {
			inDoubleQuote = true
			i++
			continue
		}

		// 检测 <, <<, <<-, <<<
		if c == '<' {
			if i+1 < len(runes) && runes[i+1] == '<' {
				// 是 << 开头
				if i+2 < len(runes) && runes[i+2] == '<' {
					// <<< here-string，既非 heredoc 也非 file_redirect
					i += 3
					continue
				} else if i+2 < len(runes) && runes[i+2] == '-' {
					// <<- heredoc
					hasHeredoc = true
					i += 3
					continue
				} else {
					// << heredoc
					hasHeredoc = true
					i += 2
					continue
				}
			}
			// 单独 < 是输入重定向
			hasFileRedirect = true
			i++
			continue
		}

		// 检测 >, >>
		if c == '>' {
			hasFileRedirect = true
			if i+1 < len(runes) && runes[i+1] == '>' {
				i += 2
			} else {
				i++
			}
			continue
		}

		// 检测命令分隔符: && || ; |
		if c == '&' && i+1 < len(runes) && runes[i+1] == '&' {
			hasSeparator = true
			i += 2
			continue
		}
		if c == '|' && i+1 < len(runes) && runes[i+1] == '|' {
			hasSeparator = true
			i += 2
			continue
		}
		if c == '|' {
			hasSeparator = true
			i++
			continue
		}
		if c == ';' {
			hasSeparator = true
			i++
			continue
		}

		i++
	}

	return hasHeredoc && !hasFileRedirect && !hasSeparator
}

func fallbackToolName(name string) string {
	if name == "" {
		return "未知工具"
	}
	return name
}

func truncateMessage(msg string, limit int) string {
	if msg == "" {
		return ""
	}
	if len(msg) <= limit {
		return msg
	}
	return msg[:limit-3] + "..."
}

// buildDetail 构造审批卡片的"查看详情"折叠区内容，展示完整上下文。
// buildDetail 构造审批卡片"查看详情"弹层内容，包含元信息 + 完整对话上下文。
func buildDetail(p payload) string {
	var sb strings.Builder
	sb.WriteString("**会话** `")
	sb.WriteString(p.SessionID)
	sb.WriteString("`  **目录** `")
	sb.WriteString(p.CWD)
	sb.WriteString("`  **模式** `")
	sb.WriteString(p.PermissionMode)
	sb.WriteString("`  **模型** `")
	sb.WriteString(p.Model)
	sb.WriteString("`\n\n")

	// 完整 tool_input JSON
	if p.ToolInput != nil {
		if raw, err := json.MarshalIndent(p.ToolInput, "", "  "); err == nil {
			sb.WriteString("**Tool Input**\n```json\n")
			sb.Write(raw)
			sb.WriteString("\n```\n\n")
		}
	}

	// 读取 transcript 文件，提取完整对话上下文
	if p.TranscriptPath != "" {
		if transcript := extractTranscript(p.TranscriptPath); transcript != "" {
			sb.WriteString("---\n**对话上下文**\n\n")
			sb.WriteString(transcript)
		}
	}

	return sb.String()
}

// extractTranscript 读取 Codex transcript JSONL 文件，从最新记录向前提取，
// 累计不超过 maxTranscriptBytes 字节（飞书富文本单条限制约 30KB，留余量给元信息）。
func extractTranscript(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	// 从后往前解析，累计字节数不超过上限
	const maxTranscriptBytes = 26000
	var entries []string
	totalBytes := 0
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		formatted := formatTranscriptLine(line)
		if formatted == "" {
			continue
		}
		if totalBytes+len(formatted) > maxTranscriptBytes {
			break
		}
		entries = append([]string{formatted}, entries...)
		totalBytes += len(formatted)
	}

	return strings.Join(entries, "")
}

// formatTranscriptLine 解析单行 transcript JSON，返回格式化文本。
func formatTranscriptLine(line string) string {
	var entry struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return ""
	}
	if entry.Type != "response_item" {
		return ""
	}
	var pl struct {
		Type      string          `json:"type"`
		Role      string          `json:"role"`
		Content   json.RawMessage `json:"content"`
		Name      string          `json:"name"`
		Arguments string          `json:"arguments"`
		Output    string          `json:"output"`
		Summary   json.RawMessage `json:"summary"`
	}
	if err := json.Unmarshal(entry.Payload, &pl); err != nil {
		return ""
	}
	switch pl.Type {
	case "message":
		text := extractContentText(pl.Content)
		if text == "" {
			return ""
		}
		var sb strings.Builder
		if pl.Role == "user" {
			sb.WriteString("👤 用户\n")
		} else {
			sb.WriteString("🤖 助手\n")
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
		return sb.String()
	case "reasoning":
		text := extractSummaryText(pl.Summary)
		if text == "" {
			return ""
		}
		return "💭 思考\n" + text + "\n\n"
	case "function_call":
		if pl.Name == "" {
			return ""
		}
		return "🔧 工具调用: " + pl.Name + "\n" + pl.Arguments + "\n\n"
	case "function_call_output":
		if pl.Output == "" {
			return ""
		}
		return "📤 输出\n" + pl.Output + "\n\n"
	}
	return ""
}

// extractContentText 从 message 的 content 数组中提取所有文本。
func extractContentText(raw json.RawMessage) string {
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		if item.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(item.Text)
		}
	}
	return sb.String()
}

// extractSummaryText 从 reasoning 的 summary 数组中提取文本。
func extractSummaryText(raw json.RawMessage) string {
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		if item.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(item.Text)
		}
	}
	return sb.String()
}
