package codexhooks

import (
	"testing"
)

// runRemoteApproval 现在是非阻塞模式：返回空字符串（不输出决策 JSON），
// Codex 回退终端审批菜单，serve 通过 TIOCSTI 注入按键。
// 旧测试（decisionJSON / permissionDecision）已移除，新增 TTY 查找测试。

func TestFindControllingTTY(t *testing.T) {
	// 在非终端环境（如 CI）下可能找不到控制终端，不应 panic
	_, err := FindControllingTTY()
	// 测试环境通常没有 TTY，允许返回错误
	_ = err
}
