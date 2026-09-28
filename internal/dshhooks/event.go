package dshhooks

import (
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 是 DSH 插件（agent-notify-dsh）写来的一行 JSON。
//
// 字段名刻意沿用 Claude Code 方言（hook_event_name / session_id / cwd /
// tool_name），这样两端复用同一套具名语义，插件侧不必发明第二套词汇。
// 插件只投递「已归一化」的小信封，不含 DSH 的完整事件对象——避免把二进制
// 耦合到 DSH 的内部类型上。
type payload struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
	CWD           string `json:"cwd"`
	ToolName      string `json:"tool_name"`
	Reason        string `json:"reason"`
	Question      string `json:"question"`
	Error         string `json:"error"`
}

// agent 是本包归一化事件归属的 agent 标识，与 config 的 dsh 段、logo 文件名一致。
const agent = "dsh"

// ParseMessage 解析 DSH 插件投递的归一化事件。
//
// 事件名与归一化事件的对应关系：
//
//	SessionStart     -> session_start        （仅点击聚焦捕获，Dispatch 拦截、不发通知）
//	ApprovalRequest  -> permission_required  （等待工具授权，本接入的核心增量）
//	UserQuestion     -> input_required       （等待用户回答）
//	Stop             -> run_completed
//	RunFailed        -> run_failed
func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	sessionID := strings.TrimSpace(p.SessionID)
	workspace := strings.TrimSpace(p.CWD)

	switch p.HookEventName {
	case "SessionStart":
		return notify.Message{
			Agent:     agent,
			Event:     "session_start",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "ApprovalRequest":
		return notify.Message{
			Agent:     agent,
			Event:     "permission_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "permission_required"),
			Body:      permissionBody(p.ToolName, p.Reason),
		}, nil
	case "UserQuestion":
		return notify.Message{
			Agent:     agent,
			Event:     "input_required",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "input_required"),
			Body:      questionBody(p.Question),
		}, nil
	case "Stop":
		return notify.Message{
			Agent:     agent,
			Event:     "run_completed",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	case "RunFailed":
		return notify.Message{
			Agent:     agent,
			Event:     "run_failed",
			SessionID: sessionID,
			Workspace: workspace,
			Title:     notify.FormatTitle(agent, "run_failed"),
			Body:      failureBody(p.Error),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported DSH event: %s", p.HookEventName)
	}
}

// permissionBody 构造「等待授权」正文。DSH 的 approval/request 未必带原因，
// 有原因时附上并截断，避免手机通知被长文案撑爆。
func permissionBody(toolName, reason string) string {
	tool := fallbackToolName(toolName)
	if r := strings.TrimSpace(reason); r != "" {
		return fmt.Sprintf("工具 %s 需要授权\n原因: %s", tool, common.TruncateRunes(r, 180))
	}
	return fmt.Sprintf("工具 %s 需要授权", tool)
}

// questionBody 构造「等待输入」正文。DSH 的提问会阻塞会话等用户回答，
// 因此问题文本本身最有信息量；没有问题时退回通用文案。
func questionBody(question string) string {
	if q := strings.TrimSpace(question); q != "" {
		return fmt.Sprintf("等待您的回答：%s", common.TruncateRunes(q, 120))
	}
	return "需要您的输入"
}

// failureBody 构造「运行失败」正文。agent/error 的载荷已由插件归一化为字符串，
// 这里只做截断与空值兜底。
func failureBody(errMessage string) string {
	if msg := strings.TrimSpace(errMessage); msg != "" {
		return fmt.Sprintf("运行失败：%s", common.TruncateRunes(msg, 120))
	}
	return "运行失败"
}

// fallbackToolName 与其它 hooks 包保持一致：工具名缺失时给出可读占位，
// 而不是让正文出现「工具  需要授权」这样的断句。
func fallbackToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知工具"
	}
	return name
}
