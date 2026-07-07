package cli

import (
	"bytes"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

func TestRunInitXiaoduWritesBehaviorOptions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	streams := Streams{Stdout: &bytes.Buffer{}}
	prompter := &fakePrompter{
		confirm: []bool{false},
		inputs: []string{
			"https://example.com/rpc",
			"access-token",
			"refresh-token",
			"client-id",
			"client-secret",
			"123",
			"device-id",
			"cuid",
			"3",
			"10",
		},
	}

	if err := runInitXiaodu(streams, prompter); err != nil {
		t.Fatalf("runInitXiaodu() error = %v", err)
	}

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	x := cfg.Notify.Codex.Channels.Xiaodu
	if x.ShouldSpeakCompleted() {
		t.Fatal("ShouldSpeakCompleted() = true, want false from TUI")
	}
	if x.RepeatCount != 3 {
		t.Fatalf("RepeatCount = %d, want 3", x.RepeatCount)
	}
	if x.RepeatIntervalSeconds != 10 {
		t.Fatalf("RepeatIntervalSeconds = %d, want 10", x.RepeatIntervalSeconds)
	}
}
