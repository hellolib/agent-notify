package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func runTestXiaodu(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	xiaoduCfg := xiaoduConfigFromConfig(cfg)
	if xiaoduCfg.APIBaseURL == "" || xiaoduCfg.AccessToken == "" {
		return fmt.Errorf("%s", i18n.T("err.xiaodu_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestXiaodu(ctx, xiaoduCfg)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitXiaodu(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	current := xiaoduConfigFromConfig(cfg)
	apiBaseURL, err := prompter.Input(i18n.T("prompt.xiaodu_api_base_url"), current.APIBaseURL)
	if err != nil {
		return err
	}
	accessToken, err := prompter.Input(i18n.T("prompt.xiaodu_access_token"), current.AccessToken)
	if err != nil {
		return err
	}
	refreshToken, err := prompter.Input(i18n.T("prompt.xiaodu_refresh_token"), current.RefreshToken)
	if err != nil {
		return err
	}
	clientID, err := prompter.Input(i18n.T("prompt.xiaodu_client_id"), current.ClientID)
	if err != nil {
		return err
	}
	clientSecret, err := prompter.Input(i18n.T("prompt.xiaodu_client_secret"), current.ClientSecret)
	if err != nil {
		return err
	}
	expiresAtRaw := ""
	if current.ExpiresAt > 0 {
		expiresAtRaw = strconv.FormatInt(current.ExpiresAt, 10)
	}
	expiresAtInput, err := prompter.Input(i18n.T("prompt.xiaodu_expires_at"), expiresAtRaw)
	if err != nil {
		return err
	}
	expiresAt := int64(0)
	if expiresAtInput != "" {
		expiresAt, err = strconv.ParseInt(expiresAtInput, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid xiaodu expires_at: %w", err)
		}
	}
	deviceID, err := prompter.Input(i18n.T("prompt.xiaodu_device_id"), current.DeviceID)
	if err != nil {
		return err
	}
	cuid, err := prompter.Input(i18n.T("prompt.xiaodu_cuid"), current.CUID)
	if err != nil {
		return err
	}

	next := config.XiaoduChannelConfig{
		Enabled:      true,
		APIBaseURL:   apiBaseURL,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		ExpiresAt:    expiresAt,
		DeviceID:     deviceID,
		CUID:         cuid,
	}
	cfg.Notify.ClaudeCode.Channels.Xiaodu = next
	cfg.Notify.Codex.Channels.Xiaodu = next
	cfg.Notify.ZCode.Channels.Xiaodu = next

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("xiaodu.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func xiaoduConfigFromConfig(cfg config.Config) config.XiaoduChannelConfig {
	for _, item := range []config.XiaoduChannelConfig{
		cfg.Notify.ClaudeCode.Channels.Xiaodu,
		cfg.Notify.Codex.Channels.Xiaodu,
		cfg.Notify.ZCode.Channels.Xiaodu,
	} {
		if item.APIBaseURL != "" || item.AccessToken != "" || item.DeviceID != "" || item.CUID != "" {
			return item
		}
	}
	return config.XiaoduChannelConfig{}
}
