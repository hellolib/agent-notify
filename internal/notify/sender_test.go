package notify

import "testing"

func TestNewSystemSenderPassesFocusPrecision(t *testing.T) {
	// We assert the factory doesn't panic and returns a non-nil sender for each precision.
	for _, p := range []string{"app", "window", ""} {
		if s := NewSystemSender(DefaultRunner, true, p, false); s == nil {
			t.Fatalf("NewSystemSender(precision=%q) returned nil", p)
		}
	}
}
