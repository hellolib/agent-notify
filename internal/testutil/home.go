// Package testutil provides shared helpers for unit tests.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// IsolateHome redirects the process home directory used by os.UserHomeDir
// (and therefore config.DefaultPath / agent settings paths) into a temp dir.
//
// On Windows, UserHomeDir prefers USERPROFILE over HOME; setting only HOME
// leaves tests writing into the real user profile (e.g. polluting
// %USERPROFILE%\.agent-notify\config.yaml).
func IsolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		// Prevent HOMEDRIVE+HOMEPATH fallback if USERPROFILE is cleared elsewhere.
		t.Setenv("HOMEDRIVE", filepath.VolumeName(dir))
		t.Setenv("HOMEPATH", dir[len(filepath.VolumeName(dir)):])
	}
	return dir
}

// FakeAgentsOnPath 伪造全部四个 agent 的安装痕迹,使 DetectInstalled 在无
// 真实 agent 的环境(CI runner)中也返回 true:
//   - claude / codex / grok:PATH 上的可执行占位文件(exec.LookPath 探测);
//   - zcode:home 下的 ~/.zcode 目录(os.Stat 探测)——须先 IsolateHome。
// 本地开发机装了真实 agent 时这些伪造是冗余但无害的。
func FakeAgentsOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"claude", "codex", "grok"} {
		if runtime.GOOS == "windows" {
			writeFakeExec(t, filepath.Join(dir, name+".bat"), "@echo off\r\n")
		} else {
			writeFakeExec(t, filepath.Join(dir, name), "#!/bin/sh\nexit 0\n")
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".zcode"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFakeExec(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
