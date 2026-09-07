package agentintegrations

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/omphooks"
)

// OmpIntegration implements the native TypeScript extension integration for
// OMP (oh-my-pi).
type OmpIntegration struct{}

func NewOmpIntegration() *OmpIntegration {
	return &OmpIntegration{}
}

func (o *OmpIntegration) Name() string {
	return "OMP (oh-my-pi)"
}

// DetectInstalled checks the omp executable and the default OMP state root.
func (o *OmpIntegration) DetectInstalled() bool {
	if _, err := exec.LookPath("omp"); err == nil {
		return true
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		info, err := os.Stat(dir)
		return err == nil && info.IsDir()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".omp"))
	return err == nil && info.IsDir()
}

func (o *OmpIntegration) SettingsPath(scope string) (string, error) {
	return omphooks.SettingsPath(scope)
}

func (o *OmpIntegration) Install(settingsPath, binaryPath string) error {
	return omphooks.Install(settingsPath, common.ResolveBinaryPath(binaryPath))
}

func (o *OmpIntegration) Uninstall(settingsPath string) error {
	return omphooks.Uninstall(settingsPath)
}

func (o *OmpIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return omphooks.IsInstalled(settingsPath)
}

var _ Integration = (*OmpIntegration)(nil)
