//go:build windows

package toast

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
)

var _registeredAppIDs sync.Map

// RegisterAppID registers appID as a Windows AppUserModelID with a display name
// and an attribution icon, under HKCU\Software\Classes\AppUserModelId\<appID>.
// After registration, toasts shown via CreateToastNotifier(appID) display the
// name and icon in the notification's top (attribution) row. Idempotent.
func RegisterAppID(appID, displayName, iconPath string) error {
	if appID == "" {
		return fmt.Errorf("toast: empty AppUserModelID")
	}
	key := `HKCU\Software\Classes\AppUserModelId\` + appID
	commands := [][]string{
		{"add", key, "/v", "DisplayName", "/t", "REG_SZ", "/d", displayName, "/f"},
	}
	if iconPath != "" {
		commands = append(commands,
			[]string{"add", key, "/v", "IconUri", "/t", "REG_SZ", "/d", iconPath, "/f"})
	}
	for _, args := range commands {
		cmd := exec.Command("reg", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("reg %s: %w: %s", key, err, string(out))
		}
	}
	return nil
}

// ensureAppIDRegistered registers appID at most once per process; failures are
// swallowed so a toast still shows (just without the attribution name/icon).
func ensureAppIDRegistered(appID, displayName, iconPath string) {
	if appID == "" {
		return
	}
	if _, done := _registeredAppIDs.Load(appID); done {
		return
	}
	if err := RegisterAppID(appID, displayName, iconPath); err == nil {
		_registeredAppIDs.Store(appID, struct{}{})
	}
}
