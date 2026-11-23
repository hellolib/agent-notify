package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/antigravityhooks"
	"github.com/hellolib/agent-notify/internal/common"
)

// AntigravityIntegration implements Integration for Google Antigravity
// (Antigravity CLI / Antigravity App / Antigravity IDE).
type AntigravityIntegration struct{}

// NewAntigravityIntegration creates a new Antigravity integration.
func NewAntigravityIntegration() *AntigravityIntegration {
	return &AntigravityIntegration{}
}

// Name returns the display name for Antigravity.
func (a *AntigravityIntegration) Name() string {
	return "Antigravity"
}

// DetectInstalled checks whether Antigravity CLI/App/IDE is present on this machine:
// looks for `agy` or `antigravity` on PATH, or any Antigravity data/config directory
// (~/.gemini/antigravity-cli, ~/.gemini/antigravity, ~/.gemini/antigravity-ide,
// ~/.config/antigravity). Note: ~/.gemini alone is intentionally not checked because
// other Gemini tools share it.
func (a *AntigravityIntegration) DetectInstalled() bool {
	for _, bin := range []string{"agy", "antigravity"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	candidates := []string{
		filepath.Join(home, ".gemini", "antigravity-cli"),
		filepath.Join(home, ".gemini", "antigravity"),
		filepath.Join(home, ".gemini", "antigravity-ide"),
		filepath.Join(home, ".config", "antigravity"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// SettingsPath returns the path to Antigravity's hooks.json file.
// Antigravity loads global hooks from ~/.gemini/config/hooks.json (shared by
// Antigravity App, CLI, and IDE) and workspace hooks from .agents/hooks.json.
func (a *AntigravityIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".gemini", "config", "hooks.json"), nil
	case "project":
		return filepath.Join(".agents", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install writes Antigravity hooks for SessionStart / PreToolUse / Stop
// into hooks.json under the "agent-notify" named hook key.
func (a *AntigravityIntegration) Install(settingsPath, binaryPath string) error {
	return antigravityhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only agent-notify hook entries from Antigravity's hooks.json.
func (a *AntigravityIntegration) Uninstall(settingsPath string) error {
	return antigravityhooks.Uninstall(settingsPath)
}

// IsHookInstalled checks whether agent-notify hooks are registered for Antigravity.
func (a *AntigravityIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return antigravityhooks.IsInstalled(settingsPath)
}
