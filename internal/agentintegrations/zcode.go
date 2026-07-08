package agentintegrations

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/zcodehooks"
)

// ZcodeIntegration implements Integration for ZCode (Z.ai)。
type ZcodeIntegration struct{}

// NewZcodeIntegration creates a new ZCode integration.
func NewZcodeIntegration() *ZcodeIntegration {
	return &ZcodeIntegration{}
}

// Name returns the display name for ZCode.
func (z *ZcodeIntegration) Name() string {
	return "ZCode"
}

// DetectInstalled 检查 ZCode 是否安装。ZCode 是 Electron 桌面应用，
// 没有全局 CLI，因此通过判断用户配置目录 ~/.zcode 是否存在来检测。
func (z *ZcodeIntegration) DetectInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".zcode"))
	return err == nil && info.IsDir()
}

// SettingsPath 返回 ZCode 用户级 CLI 配置文件路径。
// ZCode 的 hook 配置与 mcp 配置共用 ~/.zcode/cli/config.json。
// 该文件由 ZCode 内部通过 join(~/.zcode/cli, "config.json") 解析。
func (z *ZcodeIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".zcode", "cli", "config.json"), nil
	case "project":
		return filepath.Join(".zcode", "cli", "config.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install 写入 ZCode config.json，订阅 SessionStart / PermissionRequest /
// PostToolUse / PostToolUseFailure / Stop 事件。已存在 agent-notify hook 的事件会被跳过；
// 用户挂载的其他 hook、以及 config.json 中的其它顶层键（如 mcp）原样保留。
func (z *ZcodeIntegration) Install(settingsPath, binaryPath string) error {
	return zcodehooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only the hook entries written by agent-notify from
// ZCode's config.json. User-defined hooks and other top-level keys are
// preserved.
func (z *ZcodeIntegration) Uninstall(settingsPath string) error {
	return zcodehooks.Uninstall(settingsPath)
}

// IsHookInstalled 检查 ZCode config.json 中是否已经登记了 handle-zcode-hook。
func (z *ZcodeIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return zcodehooks.IsInstalled(settingsPath)
}
