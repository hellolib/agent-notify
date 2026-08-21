package opencodehooks

import (
	"fmt"
	"io"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

// payload 描述 opencode 插件通过 stdin 投递的事件 JSON。
// opencode 插件收到的事件结构为 {id, type, properties}，
// 插件把整个 event 对象透传给 handle-opencode-hook 的 stdin。
// properties 内公共字段为 sessionID 和 timestamp，各事件另有自身字段。
type payload struct {
	Type       string     `json:"type"`
	Properties properties `json:"properties"`
}

type properties struct {
	SessionID  string         `json:"sessionID"`
	Timestamp  int64          `json:"timestamp"`
	Directory  string         `json:"directory"`  // session.created 的 info.directory
	Permission string         `json:"permission"` // permission.asked
	Patterns   []string       `json:"patterns"`   // permission.asked
	Questions  []questionInfo `json:"questions"`  // question.asked
	Status     statusPayload  `json:"status"`     // session.status
	Error      errorPayload   `json:"error"`      // session.error
}

// questionInfo 对应 opencode SDK 的 QuestionInfo。
// question 为完整问题文本，header 为不超过 30 字符的短标签，
// 一次 question.asked 可能携带多个问题（TUI 上以 tab 形式并列）。
type questionInfo struct {
	Question string `json:"question"`
	Header   string `json:"header"`
}

type statusPayload struct {
	Type string `json:"type"` // busy / idle
}

type errorPayload struct {
	Message string `json:"message"`
}

func ParseMessage(stdin io.Reader) (notify.Message, error) {
	var p payload
	if err := common.DecodeHookPayload(stdin, &p); err != nil {
		return notify.Message{}, err
	}

	switch p.Type {
	case "session.created":
		return notify.Message{
			Agent:     "opencode",
			Event:     "session_start",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "session_start"),
			Body:      notify.DefaultBody("session_start"),
		}, nil
	case "permission.asked":
		return notify.Message{
			Agent:     "opencode",
			Event:     "permission_required",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "permission_required"),
			Body:      permissionBody(p.Properties.Permission),
		}, nil
	case "question.asked":
		// Agent 通过 ask 工具抛出选择题，会话此时仍为 busy，
		// session.status(idle) / session.idle 都不会触发，
		// 因此必须单独订阅，否则用户等在选择题界面收不到任何通知。
		// 归一化为 input_required：语义上就是"Agent 被阻塞、等你回答"。
		return notify.Message{
			Agent:     "opencode",
			Event:     "input_required",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "input_required"),
			Body:      questionBody(p.Properties.Questions),
		}, nil
	case "session.status":
		// 仅 idle 状态转发（插件已过滤 busy），表示会话空闲等待用户输入。
		if p.Properties.Status.Type != "idle" {
			return notify.Message{}, fmt.Errorf("skip session.status type=%s", p.Properties.Status.Type)
		}
		return notify.Message{
			Agent:     "opencode",
			Event:     "input_required",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "input_required"),
			Body:      "等待您的输入",
		}, nil
	case "session.idle":
		return notify.Message{
			Agent:     "opencode",
			Event:     "run_completed",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "run_completed"),
			Body:      notify.DefaultBody("run_completed"),
		}, nil
	case "session.error":
		errMsg := strings.TrimSpace(p.Properties.Error.Message)
		if errMsg == "" {
			errMsg = "运行出错"
		}
		return notify.Message{
			Agent:     "opencode",
			Event:     "run_failed",
			SessionID: p.Properties.SessionID,
			Workspace: p.Properties.Directory,
			Title:     notify.FormatTitle("opencode", "run_failed"),
			Body:      fmt.Sprintf("错误: %s", common.TruncateRunes(errMsg, 200)),
		}, nil
	default:
		return notify.Message{}, fmt.Errorf("unsupported opencode event: %s", p.Type)
	}
}

// questionBody 构造选择题通知的正文。
//
// 输入来自 opencode 的 question.asked 事件，一次可能携带多个问题
// （TUI 上以 tab 形式并列，如"任务类型/项目范围/回复风格"）。
//
// 正文格式为「数量 + 首条问题」：多问题时先用一行点明还有几问，
// 再补上第一条完整问题，让用户在锁屏上就能判断值不值得回到电脑前；
// 单问题时退化为只显示问题本身，不加冗余的"1 个问题"。
// Question 来自模型输出、长度不可控，截断到 200 runes。
func questionBody(questions []questionInfo) string {
	// 取第一条有内容的问题文本；question 为空时退而用 header 短标签。
	first := ""
	for _, q := range questions {
		if s := strings.TrimSpace(q.Question); s != "" {
			first = common.TruncateRunes(s, 200)
			break
		}
		if s := strings.TrimSpace(q.Header); s != "" {
			first = common.TruncateRunes(s, 200)
			break
		}
	}

	switch {
	case len(questions) > 1 && first != "":
		return fmt.Sprintf("%d 个问题待回答\n%s", len(questions), first)
	case len(questions) > 1:
		return fmt.Sprintf("%d 个问题待回答", len(questions))
	case first != "":
		return first
	default:
		return "等待您的回答"
	}
}

// permissionBody 构造授权等待通知的正文。
// permission 字段可能来自 OpenCode 的原始请求，长度不可控，截断到 200 runes。
func permissionBody(permission string) string {
	perm := strings.TrimSpace(permission)
	if perm != "" {
		return fmt.Sprintf("权限: %s\n操作需要您的授权许可", common.TruncateRunes(perm, 200))
	}
	return "操作需要您的授权许可"
}
