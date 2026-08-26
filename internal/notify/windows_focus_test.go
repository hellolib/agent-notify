//go:build windows

package notify

import (
	"strings"
	"testing"

	"github.com/hellolib/toast"
)

func TestSelectWorkspaceWindowFindsUniqueVSCodeWindow(t *testing.T) {
	chain := []toast.AncestorWindows{{
		PID: 4180,
		Exe: "Code.exe",
		Windows: []toast.WindowInfo{
			{HWND: 0x105b4, Title: "README.md - Rise-LLM - Visual Studio Code", Visible: true},
			{HWND: 0x105c4, Title: "main.go - 8 正式开发 - Visual Studio Code", Visible: true},
			{HWND: 0x10604, Title: "paper.tex - ACM_CCS_Camera_Ready - Visual Studio Code", Visible: true},
		},
	}}

	hwnd, pid, reason := selectWorkspaceWindow(chain, `D:\Users\admin\Desktop\Rise-LLM`)
	if hwnd != 0x105b4 || pid != 4180 {
		t.Fatalf("selection = (0x%x,%d,%q), want (0x105b4,4180,...)", hwnd, pid, reason)
	}
}

func TestSelectWorkspaceWindowRequiresUniqueUsableMatch(t *testing.T) {
	chain := []toast.AncestorWindows{{
		PID: 7,
		Windows: []toast.WindowInfo{
			{HWND: 1, Title: "project - Visual Studio Code", Visible: true},
			{HWND: 2, Title: "project - Visual Studio Code", Visible: true},
			{HWND: 3, Title: "project", Visible: false},
			{HWND: 4, Title: "project", Visible: true, HasOwner: true},
		},
	}}

	hwnd, _, reason := selectWorkspaceWindow(chain, "/work/project/")
	if hwnd != 0 || !strings.Contains(reason, "matched 2") {
		t.Fatalf("selection = (0x%x,%q), want ambiguous match", hwnd, reason)
	}
}

func TestWorkspaceBaseNameSupportsBothPathStyles(t *testing.T) {
	for input, want := range map[string]string{
		`C:\work\demo\`: "demo",
		"/work/demo/":   "demo",
		`C:\`:           "",
		"/":             "",
	} {
		if got := workspaceBaseName(input); got != want {
			t.Errorf("workspaceBaseName(%q) = %q, want %q", input, got, want)
		}
	}
}
