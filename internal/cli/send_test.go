package cli

import "testing"

func TestNormalizeSendAgent(t *testing.T) {
	tests := map[string]string{
		"Claude Code": "claude",
		"open_code":   "opencode",
		"oh-my-pi":    "omp",
		"pi":          "omp",
	}
	for input, want := range tests {
		if got := normalizeSendAgent(input); got != want {
			t.Fatalf("normalizeSendAgent(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeSendChannel(t *testing.T) {
	tests := map[string]string{
		"wechat_work": "wechat-work",
		"wechatwork":  "wechat-work",
		"wecom":       "wechat-work",
		"dingtalk":    "dingtalk",
	}
	for input, want := range tests {
		if got := normalizeSendChannel(input); got != want {
			t.Fatalf("normalizeSendChannel(%q) = %q, want %q", input, got, want)
		}
	}
}
