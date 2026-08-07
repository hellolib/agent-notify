package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/opencodehooks"
)

// OpenCodeIntegration implements Integration for OpenCode.
type OpenCodeIntegration struct{}

// NewOpenCodeIntegration creates a new OpenCode integration.
func NewOpenCodeIntegration() *OpenCodeIntegration {
	return &OpenCodeIntegration{}
}

// Name returns the display name for OpenCode.
func (o *OpenCodeIntegration) Name() string {
	return "OpenCode"
}

// DetectInstalled checks if the opencode CLI is installed, or if ~/.config/opencode exists.
func (o *OpenCodeIntegration) DetectInstalled() bool {
	if _, err := exec.LookPath("opencode"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	// opencode 同时支持 XDG_CONFIG_HOME 和 ~/.config/opencode
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		if _, err := os.Stat(filepath.Join(xdgConfig, "opencode")); err == nil {
			return true
		}
	}
	info, err := os.Stat(filepath.Join(home, ".config", "opencode"))
	return err == nil && info.IsDir()
}

// SettingsPath returns the path to OpenCode's config file.
// OpenCode 加载 ~/.config/opencode/opencode.json（user）或
// <project>/opencode.json（project），同时也会加载 .jsonc 变体。
// 我们只写 .json，不碰 .jsonc。
func (o *OpenCodeIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		xdgConfig := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfig != "" {
			return filepath.Join(xdgConfig, "opencode", "opencode.json"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	case "project":
		return filepath.Join("opencode.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install 写入 agent-notify 插件到 pluginPath，并把插件路径注册到 opencode 配置的 plugin 数组。
func (o *OpenCodeIntegration) Install(settingsPath, binaryPath string) error {
	pluginPath, err := PluginFilePath()
	if err != nil {
		return err
	}
	resolvedBinary := common.ResolveBinaryPath(binaryPath)
	return opencodehooks.Install(settingsPath, pluginPath, resolvedBinary)
}

// Uninstall 从 opencode 配置的 plugin 数组中移除 agent-notify 插件，并删除插件 JS 文件。
func (o *OpenCodeIntegration) Uninstall(settingsPath string) error {
	pluginPath, err := PluginFilePath()
	if err != nil {
		return err
	}
	return opencodehooks.Uninstall(settingsPath, pluginPath)
}

// IsHookInstalled 检查 opencode 配置中是否已注册 agent-notify 插件。
func (o *OpenCodeIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return opencodehooks.IsInstalled(settingsPath)
}

// PluginFilePath 返回 agent-notify 分发的 opencode 插件 JS 文件路径。
func PluginFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "opencode-plugin.js"), nil
}
