package notify

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFocusHelperLogPath(t *testing.T) {
	if got := focusHelperLogPath(false); got != "" {
		t.Fatalf("disabled should be empty, got %q", got)
	}
	got := focusHelperLogPath(true)
	if got == "" || !strings.HasSuffix(filepath.ToSlash(got), ".agent-notify/focus-helper.log") {
		t.Fatalf("enabled path unexpected: %q", got)
	}
}
