package omphooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

const (
	pluginMarker = "agent-notify OMP extension"
	binaryMarker = "__AGENT_NOTIFY_BINARY__"
)

// SettingsPath returns the extension file OMP should auto-discover.
// OMP uses ~/.omp/agent by default, supports profile-specific agent roots,
// and allows PI_CODING_AGENT_DIR to override the user root.
func SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		dir, err := userAgentDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "extensions", "agent-notify.ts"), nil
	case "project":
		return filepath.Join(".omp", "extensions", "agent-notify.ts"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

func userAgentDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	profile := strings.TrimSpace(os.Getenv("OMP_PROFILE"))
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("PI_PROFILE"))
	}
	if profile != "" {
		return filepath.Join(home, ".omp", "profiles", profile, "agent"), nil
	}
	return filepath.Join(home, ".omp", "agent"), nil
}

// Install writes the OMP extension. OMP discovers .ts files directly from
// its extensions directory, so no OMP JSON/YAML configuration is modified.
func Install(path, binaryPath string) error {
	return common.WriteFileAtomic(path, renderPlugin(common.ResolveBinaryPath(binaryPath)), 0o644)
}

// IsInstalled checks that the target file is an agent-notify-owned extension.
func IsInstalled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), pluginMarker), nil
}

// Uninstall removes only an extension containing our ownership marker.
func Uninstall(path string) error {
	installed, err := IsInstalled(path)
	if err != nil || !installed {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cleanEmptyParents(path)
	return nil
}

func cleanEmptyParents(path string) {
	dir := filepath.Dir(path)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

func renderPlugin(binaryPath string) []byte {
	// common.ResolveBinaryPath normalizes Windows separators. JSON string
	// escaping is still needed for spaces, quotes, and backslashes.
	quoted := fmt.Sprintf("%q", binaryPath)
	return []byte(strings.ReplaceAll(PluginTS, binaryMarker, quoted))
}

// BuildPluginSettings returns the path in the same shape used by print-hooks
// commands for the other integrations.
func BuildPluginSettings(path string) map[string]any {
	return map[string]any{"extension": path}
}
