package notify

func FormatTitle(agent, event string) string {
	return appDisplayName(agent) + " " + eventDisplayName(event)
}

func DefaultBody(event string) string {
	switch event {
	case "session_start":
		return "新的 Agent 会话已开始"
	case "run_completed":
		return "任务已完成，请查看结果"
	default:
		return ""
	}
}

func appDisplayName(agent string) string {
	switch agent {
	case "claude_code":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "zcode":
		return "ZCode"
	case "grok":
		return "Grok"
	default:
		return agent
	}
}

func eventDisplayName(event string) string {
	switch event {
	case "session_start":
		return "会话开始"
	case "permission_required":
		return "等待授权"
	case "input_required":
		return "等待输入"
	case "run_completed":
		return "运行完成"
	case "run_failed":
		return "运行失败"
	default:
		return event
	}
}
