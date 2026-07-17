package cli

import "testing"

func TestSelectFilterIgnoresWhitespaceOnly(t *testing.T) {
	// Chinese menu labels have no spaces; default survey filter would hide all of them.
	labels := []string{
		"Agent通知配置",
		"消息渠道配置",
		"测试通知",
		"环境诊断",
		"查看配置",
		"清理配置",
		"语言[Language]",
		"退出",
	}

	for _, filter := range []string{" ", "  ", "\t", " \t "} {
		for _, label := range labels {
			if !selectFilter(filter, label, 0) {
				t.Fatalf("selectFilter(%q, %q) = false, want true (whitespace-only must not hide options)", filter, label)
			}
		}
	}
}

func TestSelectFilterStillMatchesSubstring(t *testing.T) {
	tests := []struct {
		filter string
		value  string
		want   bool
	}{
		{filter: "Agent", value: "Agent通知配置", want: true},
		{filter: "agent", value: "Agent通知配置", want: true}, // case-insensitive like survey default
		{filter: "setup", value: "Agent Setup", want: true},
		{filter: "zzz", value: "Agent通知配置", want: false},
		{filter: "", value: "退出", want: true},
		{filter: "  Agent  ", value: "Agent通知配置", want: true}, // trim + match
	}
	for _, tt := range tests {
		if got := selectFilter(tt.filter, tt.value, 0); got != tt.want {
			t.Fatalf("selectFilter(%q, %q) = %v, want %v", tt.filter, tt.value, got, tt.want)
		}
	}
}
