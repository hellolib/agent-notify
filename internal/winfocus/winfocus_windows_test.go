//go:build windows

package winfocus

import "testing"

func TestTitlesCompatibleAllowsDynamicClaudeTitles(t *testing.T) {
	cases := []struct {
		name     string
		expected string
		current  string
	}{
		{"same after spinner", "⠂ Claude Code", "⠐ Claude Code"},
		{"short cached title", "claude", "⠂ Claude Code"},
		{"reverse containment", "Claude Code", "claude"},
	}
	for _, c := range cases {
		if !titlesCompatible(c.expected, c.current) {
			t.Fatalf("%s: titlesCompatible(%q,%q)=false, want true", c.name, c.expected, c.current)
		}
	}
}

func TestTitlesCompatibleRejectsDifferentWindows(t *testing.T) {
	cases := []struct {
		expected string
		current  string
	}{
		{"claude", "Windows PowerShell"},
		{"Claude Code", "命令提示符"},
		{"", "Claude Code"},
		{"Claude Code", ""},
	}
	for _, c := range cases {
		if titlesCompatible(c.expected, c.current) {
			t.Fatalf("titlesCompatible(%q,%q)=true, want false", c.expected, c.current)
		}
	}
}
