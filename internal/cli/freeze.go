package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
	"github.com/hellolib/agent-notify/internal/state"
	"github.com/spf13/cobra"
)

// knownFreezeChannels 是 --channel 可接受的全部 sender 名。
var knownFreezeChannels = map[string]struct{}{
	"system":      {},
	"feishu":      {},
	"wechat":      {},
	"wechat-work": {},
	"dingtalk":    {},
	"bark":        {},
	"ntfy":        {},
	"slack":       {},
}

func newFreezeCmd(streams Streams) *cobra.Command {
	var untilFlag string
	var channelsFlag string

	cmd := &cobra.Command{
		Use:   "freeze [duration]",
		Short: "Temporarily freeze notification channels",
		Long: `Temporarily freeze remote notification channels without changing agent hooks.

Examples:
  agent-notify freeze            # default 1h, remote channels only
  agent-notify freeze 30m
  agent-notify freeze 2h
  agent-notify freeze --until 18:00
  agent-notify freeze --channel feishu,slack 1h
  agent-notify freeze status
  agent-notify unfreeze`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] == "status" {
				return runFreezeStatus(streams)
			}
			return runFreeze(streams, args, untilFlag, channelsFlag)
		},
	}

	cmd.Flags().StringVar(&untilFlag, "until", "", "freeze until local HH:MM or RFC3339 timestamp")
	cmd.Flags().StringVar(&channelsFlag, "channel", "", "comma-separated channels to freeze (default: all remote)")
	cmd.AddCommand(newFreezeStatusCmd(streams))
	return cmd
}

func newFreezeStatusCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show freeze status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFreezeStatus(streams)
		},
	}
}

func newUnfreezeCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "unfreeze",
		Short: "Clear notification freeze",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnfreeze(streams)
		},
	}
}

func freezeStoreFromDefault() (*state.FreezeStore, error) {
	statePath, err := config.StatePath()
	if err != nil {
		return nil, err
	}
	return state.NewFreezeStore(state.FreezePath(statePath)), nil
}

func runFreeze(streams Streams, args []string, untilFlag, channelsFlag string) error {
	now := time.Now()
	until, err := parseFreezeUntil(now, args, untilFlag)
	if err != nil {
		return err
	}
	channels, err := parseFreezeChannels(channelsFlag)
	if err != nil {
		return err
	}

	store, err := freezeStoreFromDefault()
	if err != nil {
		return err
	}
	if err := store.Set(until, channels, now); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("freeze.err_save"), err)
	}

	// Set 可能回填默认渠道，再读一次用于展示。
	st := store.Load()
	fmt.Fprintln(streams.Stdout, formatFreezeDone(st, now))
	return nil
}

func runUnfreeze(streams Streams) error {
	store, err := freezeStoreFromDefault()
	if err != nil {
		return err
	}
	prev := store.Load()
	now := time.Now()
	if !prev.Active(now) {
		fmt.Fprintln(streams.Stdout, i18n.T("freeze.not_active"))
		return nil
	}
	if err := store.Clear(); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("freeze.err_save"), err)
	}
	fmt.Fprintln(streams.Stdout, i18n.T("freeze.cleared"))
	return nil
}

func runFreezeStatus(streams Streams) error {
	store, err := freezeStoreFromDefault()
	if err != nil {
		return err
	}
	st := store.Load()
	now := time.Now()
	if !st.Active(now) {
		fmt.Fprintln(streams.Stdout, i18n.T("freeze.status_inactive"))
		return nil
	}
	fmt.Fprintln(streams.Stdout, formatFreezeStatus(st, now))
	return nil
}

func parseFreezeUntil(now time.Time, args []string, untilFlag string) (time.Time, error) {
	if untilFlag != "" && len(args) > 0 && args[0] != "status" {
		return time.Time{}, fmt.Errorf("%s", i18n.T("freeze.err_duration_and_until"))
	}
	if untilFlag != "" {
		return parseUntilFlag(now, untilFlag)
	}

	raw := "1h"
	if len(args) == 1 {
		raw = args[0]
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %s", i18n.T("freeze.err_duration"), raw)
	}
	if d <= 0 {
		return time.Time{}, fmt.Errorf("%s", i18n.T("freeze.err_duration_positive"))
	}
	return now.Add(d), nil
}

func parseUntilFlag(now time.Time, raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		if !t.After(now) {
			return time.Time{}, fmt.Errorf("%s", i18n.T("freeze.err_duration_positive"))
		}
		return t, nil
	}

	// HH:MM 本地时区；已过则视为次日。
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("%s: %s", i18n.T("freeze.err_until"), raw)
	}
	var hour, min int
	if _, err := fmt.Sscanf(raw, "%d:%d", &hour, &min); err != nil {
		return time.Time{}, fmt.Errorf("%s: %s", i18n.T("freeze.err_until"), raw)
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return time.Time{}, fmt.Errorf("%s: %s", i18n.T("freeze.err_until"), raw)
	}
	loc := now.Location()
	until := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, loc)
	if !until.After(now) {
		until = until.Add(24 * time.Hour)
	}
	return until, nil
}

func parseFreezeChannels(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil // FreezeStore.Set 会填默认
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(strings.ToLower(p))
		if name == "" {
			continue
		}
		// 允许 wechat_work 别名
		if name == "wechat_work" || name == "wecom" {
			name = "wechat-work"
		}
		if _, ok := knownFreezeChannels[name]; !ok {
			return nil, fmt.Errorf("%s: %s", i18n.T("freeze.err_channel"), name)
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("freeze.err_channel_empty"))
	}
	return out, nil
}

func formatFreezeDone(st state.FreezeState, now time.Time) string {
	return fmt.Sprintf(
		i18n.T("freeze.done"),
		strings.Join(st.Channels, ","),
		st.Until.Local().Format("15:04"),
		formatRemaining(st.Until.Sub(now)),
	)
}

func formatFreezeStatus(st state.FreezeState, now time.Time) string {
	return fmt.Sprintf(
		i18n.T("freeze.status_active"),
		st.Until.Local().Format("2006-01-02 15:04"),
		formatRemaining(st.Until.Sub(now)),
		strings.Join(st.Channels, ","),
	)
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	// 向上取整到分钟，避免显示 0s 的尴尬。
	d = d.Round(time.Second)
	if d >= time.Hour {
		h := int(d / time.Hour)
		m := int((d % time.Hour) / time.Minute)
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return fmt.Sprintf("%ds", int(d/time.Second))
}
