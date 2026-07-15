package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/grokhooks"
)

// GrokIntegration implements Integration for xAI Grok CLI.
type GrokIntegration struct{}

// NewGrokIntegration creates a new Grok integration.
func NewGrokIntegration() *GrokIntegration {
	return &GrokIntegration{}
}

// Name returns the display name for Grok.
func (g *GrokIntegration) Name() string {
	return "Grok"
}

// DetectInstalled checks if the grok CLI is installed, or if ~/.grok exists.
func (g *GrokIntegration) DetectInstalled() bool {
	if _, err := exec.LookPath("grok"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".grok"))
	return err == nil && info.IsDir()
}

// SettingsPath returns the path to Grok's dedicated agent-notify hooks file.
// Grok loads all JSON files from ~/.grok/hooks/ (global) or <project>/.grok/hooks/.
func (g *GrokIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".grok", "hooks", "agent-notify.json"), nil
	case "project":
		return filepath.Join(".grok", "hooks", "agent-notify.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install writes Grok hooks for SessionStart / Notification / Stop /
// StopFailure / PostToolUseFailure into the hooks directory file.
func (g *GrokIntegration) Install(settingsPath, binaryPath string) error {
	return grokhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only agent-notify hook entries from the Grok hooks file.
func (g *GrokIntegration) Uninstall(settingsPath string) error {
	return grokhooks.Uninstall(settingsPath)
}

// IsHookInstalled checks whether agent-notify hooks are registered for Grok.
func (g *GrokIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return grokhooks.IsInstalled(settingsPath)
}
