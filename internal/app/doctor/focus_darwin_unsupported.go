//go:build !darwin

package doctor

// detectMacFocusHelper is a no-op stub on non-darwin platforms, where the
// mac window-focus helper is never present.
func detectMacFocusHelper() bool { return false }
