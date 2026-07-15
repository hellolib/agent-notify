package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWorkspaceKeepsValidPath(t *testing.T) {
	in := `E:\学习ai编程项目\agent-notify`
	got := ResolveWorkspace(in)
	if !strings.Contains(got, "学习ai编程项目") {
		t.Fatalf("ResolveWorkspace(%q) = %q, want Chinese path preserved", in, got)
	}
}

func TestResolveWorkspaceRejectsQuestionMarks(t *testing.T) {
	// Simulate ANSI-corrupted payload; fall back to process cwd.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkspace(`E:\????\agent-notify`)
	if strings.Contains(got, "?") {
		t.Fatalf("ResolveWorkspace should not return corrupted path, got %q", got)
	}
	// Cleaned paths should match (slash style may differ after Clean)
	if filepath.Clean(got) != filepath.Clean(wd) && !strings.HasSuffix(filepath.ToSlash(got), filepath.Base(wd)) {
		// Must at least be usable and free of '?'
		if !isUsableWorkspace(got) {
			t.Fatalf("ResolveWorkspace returned unusable %q (wd=%q)", got, wd)
		}
	}
}

func TestResolveWorkspaceEmptyFallsBack(t *testing.T) {
	got := ResolveWorkspace("")
	if got == "" {
		// getwd can fail in theory; empty is only ok if truly unavailable
		if wd, err := os.Getwd(); err == nil && wd != "" {
			t.Fatalf("ResolveWorkspace(\"\") = empty, want cwd fallback")
		}
	}
	if strings.Contains(got, "?") {
		t.Fatalf("fallback path must not contain '?', got %q", got)
	}
}

func TestShortenWorkspaceChinese(t *testing.T) {
	in := `E:\学习ai编程项目\agent-notify`
	got := shortenWorkspace(in)
	if got != "学习ai编程项目/agent-notify" {
		t.Fatalf("shortenWorkspace(%q) = %q, want 学习ai编程项目/agent-notify", in, got)
	}
}

func TestIsUsableWorkspace(t *testing.T) {
	if !isUsableWorkspace(`E:\学习ai编程项目\agent-notify`) {
		t.Fatal("valid Chinese path should be usable")
	}
	if isUsableWorkspace(`E:\????\agent-notify`) {
		t.Fatal("path with ? should not be usable")
	}
	if isUsableWorkspace("   ") {
		t.Fatal("blank should not be usable")
	}
}
