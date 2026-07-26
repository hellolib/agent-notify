package common

import "testing"

func TestTruncateRunes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"short ascii unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"ascii truncated", "hello world", 8, "hello..."},
		// issue #19/#33 的核心场景:CJK 每字 3 字节,按字节切必产生半个字符
		{"cjk truncated at rune boundary", "任务执行失败:权限被拒绝", 8, "任务执行失..."},
		{"cjk short unchanged", "完成", 100, "完成"},
		{"max at 3 hard cut", "abcdef", 3, "abc"},
		{"empty", "", 5, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateRunes(tc.in, tc.max)
			if got != tc.want {
				t.Fatalf("TruncateRunes(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
			// 输出必须是合法 UTF-8(无被切断的多字节序列)
			for _, r := range got {
				if r == '�' {
					t.Fatalf("output contains U+FFFD: %q", got)
				}
			}
		})
	}
}
