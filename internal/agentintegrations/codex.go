package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/codexhooks"
	"github.com/hellolib/agent-notify/internal/common"
)

// CodexIntegration implements Integration for Codex.
type CodexIntegration struct{}

// NewCodexIntegration creates a new Codex integration.
func NewCodexIntegration() *CodexIntegration {
	return &CodexIntegration{}
}

// Name returns the display name for Codex.
func (c *CodexIntegration) Name() string {
	return "Codex"
}

// DetectInstalled checks if the codex CLI is installed.
func (c *CodexIntegration) DetectInstalled() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

// SettingsPath returns the path to Codex's hooks.json file.
// Codex 同时支持 ~/.codex/hooks.json 与 ~/.codex/config.toml 内联 [hooks]；
// 这里统一使用 hooks.json，结构上与 Claude settings.json 对齐，便于维护。
func (c *CodexIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".codex", "hooks.json"), nil
	case "project":
		return filepath.Join(".codex", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install 写入 Codex hooks.json，订阅 SessionStart、PermissionRequest 与
// PreToolUse(request_user_input) 事件。Stop 表示单轮结束，不是任务完成，
// 因此不订阅。
// 已存在 agent-notify hook 的事件会被跳过；用户挂载的其他 hook 原样保留。
func (c *CodexIntegration) Install(settingsPath, binaryPath string) error {
	return codexhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify from
// Codex's hooks.json. User-defined hooks are preserved. The
// [features] hooks toggle in config.toml is NOT removed — it is a generic
// switch other hooks may depend on.
func (c *CodexIntegration) Uninstall(settingsPath string) error {
	return codexhooks.Uninstall(settingsPath)
}

// IsHookInstalled 检查 Codex hooks.json 中是否已经登记了 handle-codex-hook。
func (c *CodexIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return codexhooks.IsInstalled(settingsPath)
}
