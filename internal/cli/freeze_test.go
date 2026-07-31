package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
)

func TestParseFreezeUntilDuration(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	until, err := parseFreezeUntil(now, []string{"30m"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !until.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("until = %v, want %v", until, now.Add(30*time.Minute))
	}

	until, err = parseFreezeUntil(now, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !until.Equal(now.Add(time.Hour)) {
		t.Fatalf("default until = %v, want +1h", until)
	}

	if _, err := parseFreezeUntil(now, []string{"0s"}, ""); err == nil {
		t.Fatal("expected error for non-positive duration")
	}
	if _, err := parseFreezeUntil(now, []string{"nope"}, ""); err == nil {
		t.Fatal("expected error for bad duration")
	}
}

func TestParseUntilFlagHHMMRollsToNextDay(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, loc)

	until, err := parseUntilFlag(now, "18:00")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 7, 31, 18, 0, 0, 0, loc)
	if !until.Equal(want) {
		t.Fatalf("until = %v, want %v", until, want)
	}

	until, err = parseUntilFlag(now, "21:30")
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 7, 30, 21, 30, 0, 0, loc)
	if !until.Equal(want) {
		t.Fatalf("until = %v, want %v", until, want)
	}
}

func TestParseFreezeChannels(t *testing.T) {
	got, err := parseFreezeChannels("feishu, slack, wechat_work")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "feishu" || got[1] != "slack" || got[2] != "wechat-work" {
		t.Fatalf("got %v", got)
	}
	if _, err := parseFreezeChannels("nope"); err == nil {
		t.Fatal("expected unknown channel error")
	}
	got, err = parseFreezeChannels("")
	if err != nil || got != nil {
		t.Fatalf("empty should yield nil defaults, got %v err %v", got, err)
	}
}

func TestFormatRemaining(t *testing.T) {
	if got := formatRemaining(90 * time.Minute); got != "1h30m" {
		t.Fatalf("got %q", got)
	}
	if got := formatRemaining(45 * time.Minute); got != "45m" {
		t.Fatalf("got %q", got)
	}
}

func TestEnabledRemoteFreezeChannelsUsesConfiguredOnly(t *testing.T) {
	cfg := config.Default()
	// system on — must never appear
	cfg.Notify.ClaudeCode.Channels.System.Enabled = true
	cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = true
	// wechat enabled but no URL → not configured
	cfg.Notify.ClaudeCode.Channels.Wechat.Enabled = true
	cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL = ""
	// bark fully configured on another agent
	cfg.Notify.Codex.Channels.Bark.Enabled = true
	cfg.Notify.Codex.Channels.Bark.WebhookURL = "https://api.day.app/key"
	// slack enabled without URL → skip
	cfg.Notify.ZCode.Channels.Slack.Enabled = true

	got := enabledRemoteFreezeChannels(cfg)
	want := []string{"feishu", "bark"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, ch := range got {
		if ch == "system" {
			t.Fatal("system must not be in default freeze channels")
		}
	}
}

func TestEnabledRemoteFreezeChannelsEmptyWhenNoneConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.ClaudeCode.Channels.System.Enabled = true
	got := enabledRemoteFreezeChannels(cfg)
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}
