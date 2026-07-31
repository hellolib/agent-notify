package agentintegrations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/droidhooks"
)

// DroidIntegration implements Integration for Factory Droid.
type DroidIntegration struct{}

// NewDroidIntegration creates a new Droid integration.
func NewDroidIntegration() *DroidIntegration {
	return &DroidIntegration{}
}

// Name returns the display name for Droid.
func (d *DroidIntegration) Name() string {
	return "Droid"
}

// DetectInstalled checks if the droid CLI is installed, or if ~/.factory exists.
func (d *DroidIntegration) DetectInstalled() bool {
	if _, err := exec.LookPath("droid"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".factory"))
	return err == nil && info.IsDir()
}

// SettingsPath returns the path to Droid's hooks.json file.
// Droid loads hooks from ~/.factory/hooks.json (user) or .factory/hooks.json (project).
func (d *DroidIntegration) SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".factory", "hooks.json"), nil
	case "project":
		return filepath.Join(".factory", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// Install writes Droid hooks for SessionStart / Notification / Stop
// into the hooks.json file.
func (d *DroidIntegration) Install(settingsPath, binaryPath string) error {
	return droidhooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

// Uninstall removes only agent-notify hook entries from the Droid hooks file.
func (d *DroidIntegration) Uninstall(settingsPath string) error {
	return droidhooks.Uninstall(settingsPath)
}

// IsHookInstalled checks whether agent-notify hooks are registered for Droid.
func (d *DroidIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return droidhooks.IsInstalled(settingsPath)
}
