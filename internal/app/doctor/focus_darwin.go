//go:build darwin

package doctor

import "os"

// detectMacFocusHelper reports whether the mac window-focus helper binary is
// present at ~/.agent-notify/mac-focus-helper. AX trust (AXIsProcessTrusted)
// cannot be probed in pure Go without cgo, so it is intentionally not checked
// here — the helper self-degrades at click time if AX is missing.
func detectMacFocusHelper() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	p := home + "/.agent-notify/mac-focus-helper"
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
