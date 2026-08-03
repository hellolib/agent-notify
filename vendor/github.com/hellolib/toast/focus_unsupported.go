//go:build !windows

package toast

import "errors"

// FindFocusHelper is only supported on Windows.
func FindFocusHelper(_ ...string) (string, error) {
	return "", errors.New("toast focus helper is only supported on windows")
}

// RegisterFocusProtocol is only supported on Windows.
func RegisterFocusProtocol(_ string, _ ...string) error {
	return errors.New("toast focus protocol is only supported on windows")
}

// PrepareFocusActivation is only supported on Windows.
func PrepareFocusActivation(_ int, _ ...string) (FocusActivation, error) {
	return FocusActivation{}, errors.New("toast focus activation is only supported on windows")
}

// PrepareFocusActivationVerbose is only supported on Windows.
func PrepareFocusActivationVerbose(pid int, _ string, _ ...string) (FocusActivation, FocusDiag, error) {
	return FocusActivation{}, FocusDiag{StartPID: uint32(pid)},
		errors.New("toast focus activation is only supported on windows")
}
