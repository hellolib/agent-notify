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

func TestActivationURIFromRequest(t *testing.T) {
	want := "codex://threads/019ff54e-d894-7d43-9dfc-fa3fb41479e7"
	got, ok := activationURIFromRequest(windowsToastRequest{
		ActivationURI: want,
		FocusCapture:  `not-json`,
		Workspace:     `C:\work\demo`,
	})
	if !ok || got != want {
		t.Fatalf("activationURIFromRequest() = (%q, %t), want (%q, true)", got, ok, want)
	}

	for _, raw := range []string{"", "not a URI", "://missing-scheme"} {
		if got, ok := activationURIFromRequest(windowsToastRequest{ActivationURI: raw}); ok || got != "" {
			t.Fatalf("ActivationURI %q => (%q, %t), want (\"\", false)", raw, got, ok)
		}
	}
}
