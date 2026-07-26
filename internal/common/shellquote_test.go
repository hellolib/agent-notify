package common

import "testing"

func TestQuotePathForShell(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain unix path",
			in:   "/Users/liusaisai/.agent-notify/agent-notify",
			want: `"/Users/liusaisai/.agent-notify/agent-notify"`,
		},
		{
			// issue #30 的目标场景:Windows 用户名带空格
			name: "windows path with space",
			in:   "C:/Users/John Doe/.agent-notify/agent-notify.exe",
			want: `"C:/Users/John Doe/.agent-notify/agent-notify.exe"`,
		},
		{
			// POSIX 双引号内 $ 会展开,必须转义
			name: "dollar sign",
			in:   "/home/us$er/agent-notify",
			want: `"/home/us\$er/agent-notify"`,
		},
		{
			name: "backtick",
			in:   "/home/us`er/agent-notify",
			want: "\"/home/us\\`er/agent-notify\"",
		},
		{
			name: "backslash and quote",
			in:   `/odd/pa\th/"quoted"`,
			want: `"/odd/pa\\th/\"quoted\""`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QuotePathForShell(tc.in); got != tc.want {
				t.Fatalf("QuotePathForShell(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
