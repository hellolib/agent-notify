package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func TestMaybeStarPrompt(t *testing.T) {
	cases := []struct {
		name      string
		prompted  bool
		isTTY     bool
		wantShown bool
	}{
		{"first time interactive", false, true, true},
		{"already prompted", true, true, false},
		{"non tty", false, false, false},
		{"prompted and non tty", true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := maybeStarPrompt(config.Config{StarPrompted: tc.prompted}, &buf, tc.isTTY)
			if got != tc.wantShown {
				t.Fatalf("shown = %v, want %v", got, tc.wantShown)
			}
			if tc.wantShown && !strings.Contains(buf.String(), starRepoURL) {
				t.Fatalf("expected repo URL in output, got %q", buf.String())
			}
			if !tc.wantShown && buf.Len() != 0 {
				t.Fatalf("expected no output, got %q", buf.String())
			}
		})
	}
}

func TestMaybeStarPromptLocale(t *testing.T) {
	t.Cleanup(func() { i18n.Set("zh-CN") })

	i18n.Set("zh-CN")
	var zh bytes.Buffer
	maybeStarPrompt(config.Config{}, &zh, true)
	if !strings.Contains(zh.String(), "点个 Star") {
		t.Fatalf("zh output missing expected copy: %q", zh.String())
	}

	i18n.Set("en-US")
	var en bytes.Buffer
	maybeStarPrompt(config.Config{}, &en, true)
	if !strings.Contains(en.String(), "A GitHub star") {
		t.Fatalf("en output missing expected copy: %q", en.String())
	}
}
