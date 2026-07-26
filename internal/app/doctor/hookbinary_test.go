package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHookBinaryPathExtractsQuotedAndLegacyForms(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			name: "quoted path (current format)",
			json: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"\"/opt/bin/agent-notify\" handle-claude-hook"}]}]}}`,
			want: "/opt/bin/agent-notify",
		},
		{
			name: "legacy unquoted path",
			json: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/opt/bin/agent-notify handle-claude-hook"}]}]}}`,
			want: "/opt/bin/agent-notify",
		},
		{
			name: "quoted path with space",
			json: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"\"C:/Users/John Doe/.agent-notify/agent-notify.exe\" handle-claude-hook"}]}]}}`,
			want: "C:/Users/John Doe/.agent-notify/agent-notify.exe",
		},
		{
			// ZCode 把 hook 埋在 hooks.events.<Event> 下,递归遍历同样命中
			name: "nested under events (zcode shape)",
			json: `{"hooks":{"enabled":true,"events":{"Stop":[{"hooks":[{"type":"command","command":"\"/opt/bin/agent-notify\" handle-claude-hook"}]}]}}}`,
			want: "/opt/bin/agent-notify",
		},
		{
			name: "no managed hook",
			json: `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(tc.json), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := hookBinaryPath(path, "handle-claude-hook"); got != tc.want {
				t.Fatalf("hookBinaryPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

// issue #34:hook 指向已删除的二进制时 doctor 必须报「程序缺失」,
// 而不是显示集成正常。
func TestHookBinaryMissing(t *testing.T) {
	dir := t.TempDir()
	realBin := filepath.Join(dir, "agent-notify")
	if err := os.WriteFile(realBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(t *testing.T, command string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "settings.json")
		body := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":` + command + `}]}]}}`
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("existing binary is not missing", func(t *testing.T) {
		p := write(t, `"\"`+realBin+`\" handle-claude-hook"`)
		if hookBinaryMissing(p, "handle-claude-hook") {
			t.Fatal("existing binary reported as missing")
		}
	})

	t.Run("deleted binary is missing", func(t *testing.T) {
		p := write(t, `"\"/nonexistent/stale/agent-notify\" handle-claude-hook"`)
		if !hookBinaryMissing(p, "handle-claude-hook") {
			t.Fatal("stale path not reported as missing")
		}
	})

	t.Run("bare command name is not judged", func(t *testing.T) {
		p := write(t, `"agent-notify handle-claude-hook"`)
		if hookBinaryMissing(p, "handle-claude-hook") {
			t.Fatal("PATH-resolved bare command must not be reported missing")
		}
	})

	t.Run("unreadable settings does not false-positive", func(t *testing.T) {
		if hookBinaryMissing(filepath.Join(dir, "nope.json"), "handle-claude-hook") {
			t.Fatal("missing settings file must not report binary missing")
		}
	})
}
