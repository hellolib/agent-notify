//go:build windows

package notify

import "testing"

func TestFocusArgumentsFromCaptureRejectsEmptyAndInvalid(t *testing.T) {
	if args, ok := focusArgumentsFromCapture(windowsToastRequest{}); ok || args != "" {
		t.Fatalf("empty capture => (%q,%t), want (\"\",false)", args, ok)
	}
	if args, ok := focusArgumentsFromCapture(windowsToastRequest{FocusCapture: `not-json`}); ok || args != "" {
		t.Fatalf("invalid capture => (%q,%t), want (\"\",false)", args, ok)
	}
}
