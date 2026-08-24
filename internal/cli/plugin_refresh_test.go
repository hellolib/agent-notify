package cli

import "testing"

// hook handler 必须走无头路径：它们由 agent 高频拉起，不能带额外副作用。
func TestIsHeadlessInvocation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"无参数启动 TUI", nil, false},
		{"空切片", []string{}, false},
		{"init", []string{"init"}, false},
		{"doctor", []string{"doctor"}, false},
		{"test 带子命令", []string{"test", "feishu"}, false},
		{"opencode 安装", []string{"opencode", "--scope", "user"}, false},
		{"claude hook", []string{"handle-claude-hook"}, true},
		{"codex hook", []string{"handle-codex-hook"}, true},
		{"codex notify", []string{"handle-codex-notify", `{}`}, true},
		{"zcode hook", []string{"handle-zcode-hook"}, true},
		{"grok hook", []string{"handle-grok-hook"}, true},
		{"droid hook", []string{"handle-droid-hook"}, true},
		{"opencode hook", []string{"handle-opencode-hook"}, true},
		{"linux 点击回调", []string{"linux-notify-wait"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHeadlessInvocation(tc.args); got != tc.want {
				t.Fatalf("isHeadlessInvocation(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
