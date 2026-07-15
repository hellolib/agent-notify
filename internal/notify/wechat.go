package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	wechatHTTPTimeout      = 10 * time.Second
	maxWechatErrorBodySize = 512
)

// WechatSender sends notifications through a personal WeChat push webhook API.
//
// Expected request:
//
//	POST {webhook_url}
//	Content-Type: application/json
//	{
//	  "msgType": "text",
//	  "title": "...",
//	  "content": "..."
//	}
type WechatSender struct {
	webhookURL string
	httpClient *http.Client
}

// NewWechatSender creates a WechatSender with the provided notify API URL.
func NewWechatSender(webhookURL string) *WechatSender {
	return &WechatSender{
		webhookURL: strings.TrimSpace(webhookURL),
		httpClient: &http.Client{Timeout: wechatHTTPTimeout},
	}
}

func (s *WechatSender) Name() string { return "wechat" }

func (s *WechatSender) Send(ctx context.Context, msg Message) error {
	if s.webhookURL == "" {
		return fmt.Errorf("wechat: webhook_url is empty")
	}

	payload := map[string]string{
		"msgType": "text",
		"title":   wechatTitle(msg),
		"content": wechatContent(msg),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("wechat: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wechat: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wechat: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxWechatErrorBodySize))
		return fmt.Errorf("wechat: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	// Some gateways return JSON {code, message}; treat non-zero code as failure when present.
	var result struct {
		Code    int    `json:"code"`
		ErrCode int    `json:"errcode"`
		Message string `json:"message"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if result.Code != 0 && result.Code != 200 {
			return fmt.Errorf("wechat: api error code=%d msg=%s", result.Code, firstNonEmpty(result.Message, result.ErrMsg))
		}
		if result.ErrCode != 0 {
			return fmt.Errorf("wechat: api error code=%d msg=%s", result.ErrCode, firstNonEmpty(result.ErrMsg, result.Message))
		}
	}

	return nil
}

func wechatTitle(msg Message) string {
	if strings.TrimSpace(msg.Title) != "" {
		return msg.Title
	}
	return "Agent Notify"
}

func wechatContent(msg Message) string {
	var b strings.Builder
	if msg.Body != "" {
		b.WriteString(msg.Body)
	}
	if msg.Workspace != "" && msg.Agent != "codex" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("目录: ")
		b.WriteString(shortenWorkspace(msg.Workspace))
	}
	if msg.Event != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("事件: ")
		b.WriteString(eventDisplayName(msg.Event))
	}
	if b.Len() == 0 {
		return "Agent Notify"
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
