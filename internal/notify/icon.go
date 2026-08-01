package notify

import (
	"os"
	"path/filepath"
)

// agentLogoFilename 将 agent 名称映射到 logo 文件名。
// 保持文件名人类友好（如 openai.png 而非 codex.png），映射逻辑集中在此。
func agentLogoFilename(agent string) string {
	switch agent {
	case "claude_code":
		return "claude.png"
	case "codex":
		return "openai.png"
	default:
		return agent + ".png"
	}
}

// AgentLogoPath 返回 agent 对应的 logo 文件绝对路径。
// 查找优先级：~/.agent-notify/agentlogo/{agent}.png → ~/.agent-notify/agent-notify.png → 空串。
// 空串表示无可用图标，调用方应跳过图标参数让系统使用默认图标。
func AgentLogoPath(agent string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 优先：agent 专属 logo
	filename := agentLogoFilename(agent)
	agentPath := filepath.Join(home, ".agent-notify", "agentlogo", filename)
	if info, err := os.Stat(agentPath); err == nil && !info.IsDir() {
		return agentPath
	}

	// 回退：agent-notify 默认图标
	fallbackPath := filepath.Join(home, ".agent-notify", "agent-notify.png")
	if info, err := os.Stat(fallbackPath); err == nil && !info.IsDir() {
		return fallbackPath
	}

	return ""
}
