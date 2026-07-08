package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	xiaoduHTTPTimeout      = 10 * time.Second
	maxXiaoduErrorBodySize = 512
	maxXiaoduSpeechRunes   = 80
	xiaoduTokenURL         = "https://openapi.baidu.com/oauth/2.0/token"
	xiaoduRefreshSkew      = 3 * 24 * time.Hour
	xiaoduDefaultExpiresIn = 30 * 24 * time.Hour
)

// XiaoduTokenState is the refreshed OAuth token state returned by Baidu.
type XiaoduTokenState struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

// XiaoduSender sends notifications through a Xiaodu smart speaker MCP/JSON-RPC endpoint.
type XiaoduSender struct {
	apiBaseURL   string
	accessToken  string
	refreshToken string
	clientID     string
	clientSecret string
	expiresAt    int64
	deviceID     string
	cuid         string
	httpClient   *http.Client
	onRefresh    func(XiaoduTokenState) error

	speakCompleted    bool
	repeatCount       int
	repeatInterval    time.Duration
	reminderStorePath string
	binaryPath        string
	scheduleRepeats   bool
}

type XiaoduSenderOptions struct {
	APIBaseURL        string
	AccessToken       string
	RefreshToken      string
	ClientID          string
	ClientSecret      string
	ExpiresAt         int64
	DeviceID          string
	CUID              string
	SpeakCompleted    *bool
	RepeatCount       int
	RepeatInterval    time.Duration
	ReminderStorePath string
	BinaryPath        string
	ScheduleRepeats   bool
	OnRefresh         func(XiaoduTokenState) error
}

// NewXiaoduSender creates a XiaoduSender with the provided native Xiaodu endpoint settings.
func NewXiaoduSender(apiBaseURL, accessToken, deviceID string) *XiaoduSender {
	return &XiaoduSender{
		apiBaseURL:     strings.TrimSpace(apiBaseURL),
		accessToken:    strings.TrimSpace(accessToken),
		deviceID:       strings.TrimSpace(deviceID),
		httpClient:     &http.Client{Timeout: xiaoduHTTPTimeout},
		speakCompleted: true,
		repeatCount:    1,
	}
}

// NewXiaoduSenderWithOAuth creates a XiaoduSender with token refresh support.
func NewXiaoduSenderWithOAuth(
	apiBaseURL string,
	accessToken string,
	refreshToken string,
	clientID string,
	clientSecret string,
	expiresAt int64,
	deviceID string,
	cuid string,
	onRefresh func(XiaoduTokenState) error,
) *XiaoduSender {
	s := NewXiaoduSender(apiBaseURL, accessToken, deviceID)
	s.refreshToken = strings.TrimSpace(refreshToken)
	s.clientID = strings.TrimSpace(clientID)
	s.clientSecret = strings.TrimSpace(clientSecret)
	s.expiresAt = expiresAt
	s.cuid = strings.TrimSpace(cuid)
	s.onRefresh = onRefresh
	return s
}

func NewXiaoduSenderWithOptions(opts XiaoduSenderOptions) *XiaoduSender {
	s := NewXiaoduSenderWithOAuth(
		opts.APIBaseURL,
		opts.AccessToken,
		opts.RefreshToken,
		opts.ClientID,
		opts.ClientSecret,
		opts.ExpiresAt,
		opts.DeviceID,
		opts.CUID,
		opts.OnRefresh,
	)
	if opts.SpeakCompleted != nil {
		s.speakCompleted = *opts.SpeakCompleted
	}
	if opts.RepeatCount > 0 {
		s.repeatCount = opts.RepeatCount
	}
	s.repeatInterval = opts.RepeatInterval
	s.reminderStorePath = opts.ReminderStorePath
	s.binaryPath = strings.TrimSpace(opts.BinaryPath)
	s.scheduleRepeats = opts.ScheduleRepeats
	return s
}

func (s *XiaoduSender) Name() string { return "xiaodu" }

func (s *XiaoduSender) Send(ctx context.Context, msg Message) error {
	if !s.shouldSpeak(msg) {
		return nil
	}
	text := xiaoduSpeechText(msg)
	if err := s.sendText(ctx, msg, text); err != nil {
		return err
	}
	if s.shouldRepeat(msg) {
		return s.scheduleReminder(ctx, msg, text)
	}
	return nil
}

func (s *XiaoduSender) SendReminderText(ctx context.Context, reminder XiaoduReminder) error {
	return s.sendText(ctx, Message{
		Agent:     reminder.Agent,
		Event:     reminder.Event,
		SessionID: reminder.SessionID,
		ToolName:  reminder.ToolName,
	}, reminder.Text)
}

func (s *XiaoduSender) sendText(ctx context.Context, msg Message, text string) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.resolveAccessToken(ctx); err != nil {
		return err
	}

	device, err := s.resolveDevice(ctx)
	if err != nil {
		return err
	}

	args := map[string]any{
		"client_id": device.ClientID,
		"cuid":      device.CUID,
		"text":      text,
	}
	if _, err := s.callTool(ctx, "xiaodu_speak", args); err != nil {
		return err
	}
	return nil
}

func (s *XiaoduSender) shouldSpeak(msg Message) bool {
	return msg.Event != "run_completed" || s.speakCompleted
}

func (s *XiaoduSender) shouldRepeat(msg Message) bool {
	if s.repeatCount <= 1 || s.reminderStorePath == "" {
		return false
	}
	switch msg.Event {
	case "permission_required", "input_required", "run_failed":
		return true
	default:
		return false
	}
}

func (s *XiaoduSender) scheduleReminder(ctx context.Context, msg Message, text string) error {
	reminder := XiaoduReminder{
		Key:                   XiaoduReminderKey(msg.Agent, msg.SessionID, msg.Event, msg.ToolName),
		Agent:                 msg.Agent,
		SessionID:             msg.SessionID,
		Event:                 msg.Event,
		ToolName:              msg.ToolName,
		Text:                  text,
		Remaining:             s.repeatCount - 1,
		RepeatIntervalSeconds: int(s.repeatInterval.Seconds()),
	}
	if reminder.RepeatIntervalSeconds <= 0 {
		reminder.RepeatIntervalSeconds = 25
	}
	store := NewXiaoduReminderStore(s.reminderStorePath)
	if err := store.Save(reminder); err != nil {
		return err
	}
	if !s.scheduleRepeats {
		return nil
	}
	return s.startReminderWorker(reminder.Key)
}

func (s *XiaoduSender) startReminderWorker(key string) error {
	binaryPath := s.binaryPath
	if binaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("xiaodu: resolve executable for reminder worker: %w", err)
		}
		binaryPath = exe
	}
	cmd := exec.Command(binaryPath, "xiaodu-reminder", "--key", key)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (s *XiaoduSender) resolveDevice(ctx context.Context) (xiaoduDevice, error) {
	if s.deviceID != "" && s.cuid != "" {
		return xiaoduDevice{ClientID: s.deviceID, CUID: s.cuid, Online: true}, nil
	}

	devices, err := s.listDevices(ctx)
	if err != nil {
		return xiaoduDevice{}, err
	}
	for _, device := range devices {
		if device.ClientID == "" || device.CUID == "" {
			continue
		}
		if device.Online {
			return device, nil
		}
	}
	return xiaoduDevice{}, fmt.Errorf("xiaodu: no online device found")
}

func (s *XiaoduSender) resolveAccessToken(ctx context.Context) error {
	if s.accessToken == "" {
		return fmt.Errorf("xiaodu: access_token is empty")
	}
	if s.expiresAt == 0 {
		return nil
	}
	untilExpiry := time.Until(time.Unix(s.expiresAt, 0))
	if untilExpiry > xiaoduRefreshSkew {
		return nil
	}
	if s.refreshToken == "" || s.clientID == "" || s.clientSecret == "" {
		if untilExpiry > 0 {
			return nil
		}
		return fmt.Errorf("xiaodu: token expired and refresh setup is incomplete")
	}

	token, err := s.refreshAccessToken(ctx)
	if err != nil {
		return err
	}
	s.accessToken = token.AccessToken
	s.refreshToken = token.RefreshToken
	s.expiresAt = token.ExpiresAt
	if s.onRefresh != nil {
		if err := s.onRefresh(token); err != nil {
			return fmt.Errorf("xiaodu: persist refreshed token: %w", err)
		}
	}
	return nil
}

func (s *XiaoduSender) refreshAccessToken(ctx context.Context) (XiaoduTokenState, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", s.refreshToken)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xiaoduTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: create token refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: read token refresh response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: token refresh status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int64  `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: decode token refresh response: %w", err)
	}
	if result.Error != "" {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: token refresh failed: %s %s", result.Error, result.ErrorDescription)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		return XiaoduTokenState{}, fmt.Errorf("xiaodu: token refresh response missing access_token or refresh_token")
	}
	if result.ExpiresIn <= 0 {
		result.ExpiresIn = int64(xiaoduDefaultExpiresIn.Seconds())
	}
	return XiaoduTokenState{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().Unix() + result.ExpiresIn,
	}, nil
}

func (s *XiaoduSender) validate() error {
	if s.apiBaseURL == "" {
		return fmt.Errorf("xiaodu: api_base_url is empty")
	}
	u, err := url.Parse(s.apiBaseURL)
	if err != nil {
		return fmt.Errorf("xiaodu: parse api_base_url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("xiaodu: api_base_url must include scheme and host")
	}
	if s.accessToken == "" {
		return fmt.Errorf("xiaodu: access_token is empty")
	}
	return nil
}

type xiaoduDevice struct {
	ClientID string
	CUID     string
	Name     string
	Online   bool
}

func (s *XiaoduSender) listDevices(ctx context.Context) ([]xiaoduDevice, error) {
	result, err := s.callTool(ctx, "list_user_devices", map[string]any{})
	if err != nil {
		return nil, err
	}
	return parseXiaoduDevices(result)
}

type xiaoduRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type xiaoduRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *XiaoduSender) callTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	payload := xiaoduRPCRequest{
		JSONRPC: "2.0",
		ID:      int(time.Now().UnixNano()),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("xiaodu: marshal %s request: %w", name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("xiaodu: create %s request: %w", name, err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("ACCESS_TOKEN", s.accessToken)
	req.Header.Set("Authorization", "Bearer "+s.accessToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xiaodu: send %s request: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxXiaoduErrorBodySize))
		return nil, fmt.Errorf("xiaodu: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("xiaodu: read %s response: %w", name, err)
	}

	var rpcResp xiaoduRPCResponse
	if err := decodeXiaoduRPCResponse(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("xiaodu: decode %s response: %w", name, err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("xiaodu: rpc error code=%d msg=%s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func decodeXiaoduRPCResponse(body []byte, target *xiaoduRPCResponse) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty response")
	}
	if bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:")) || bytes.Contains(trimmed, []byte("\ndata:")) {
		return decodeXiaoduSSEResponse(trimmed, target)
	}
	return json.Unmarshal(trimmed, target)
}

func decodeXiaoduSSEResponse(body []byte, target *xiaoduRPCResponse) error {
	events := bytes.Split(body, []byte("\n\n"))
	for _, event := range events {
		var dataLines []string
		for _, line := range bytes.Split(event, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := strings.TrimSpace(string(bytes.TrimPrefix(line, []byte("data:"))))
			if data == "" || data == "[DONE]" {
				continue
			}
			dataLines = append(dataLines, data)
		}
		if len(dataLines) == 0 {
			continue
		}
		if err := json.Unmarshal([]byte(strings.Join(dataLines, "\n")), target); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no JSON-RPC data event found")
}

func parseXiaoduDevices(raw json.RawMessage) ([]xiaoduDevice, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("xiaodu: decode list_user_devices result: %w", err)
	}

	for _, candidate := range []json.RawMessage{result.StructuredContent, firstTextJSON(result.Content)} {
		if len(candidate) == 0 {
			continue
		}
		if devices := decodeXiaoduDevices(candidate); len(devices) > 0 {
			return devices, nil
		}
	}

	return nil, fmt.Errorf("xiaodu: no devices in list_user_devices response")
}

func firstTextJSON(content []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) json.RawMessage {
	for _, item := range content {
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		return json.RawMessage(item.Text)
	}
	return nil
}

func decodeXiaoduDevices(raw json.RawMessage) []xiaoduDevice {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil && strings.TrimSpace(text) != "" {
		return decodeXiaoduDevices(json.RawMessage(text))
	}

	var direct []map[string]any
	if err := json.Unmarshal(raw, &direct); err == nil {
		return devicesFromMaps(direct)
	}

	var single map[string]any
	if err := json.Unmarshal(raw, &single); err == nil {
		return devicesFromMaps([]map[string]any{single})
	}

	var wrapped struct {
		Devices []map[string]any `json:"devices"`
		Data    []map[string]any `json:"data"`
		Result  []map[string]any `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil
	}
	for _, items := range [][]map[string]any{wrapped.Devices, wrapped.Data, wrapped.Result} {
		if devices := devicesFromMaps(items); len(devices) > 0 {
			return devices
		}
	}
	return nil
}

func devicesFromMaps(items []map[string]any) []xiaoduDevice {
	devices := make([]xiaoduDevice, 0, len(items))
	for _, item := range items {
		clientID := firstString(item, "client_id", "clientId", "id", "device_id", "deviceId")
		cuid := firstString(item, "cuid", "CUID")
		if clientID == "" {
			continue
		}
		devices = append(devices, xiaoduDevice{
			ClientID: clientID,
			CUID:     cuid,
			Name:     firstString(item, "name", "device_name", "deviceName"),
			Online:   firstBool(item, "online", "online_status", "is_online", "isOnline"),
		})
	}
	return devices
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstBool(item map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch value := item[key].(type) {
		case bool:
			return value
		case string:
			return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "online")
		case float64:
			return value != 0
		}
	}
	return false
}

func xiaoduSpeechText(msg Message) string {
	subject := xiaoduSpeechSubject(msg)
	title := trimSpeechText(msg.Title)
	switch msg.Event {
	case "permission_required":
		return fmt.Sprintf("%s 需要权限确认", subject)
	case "input_required":
		return fmt.Sprintf("%s 正在等待你的输入", subject)
	case "run_completed":
		if title == "" {
			return fmt.Sprintf("%s 任务已完成", subject)
		}
		return fmt.Sprintf("%s 任务已完成：%s", subject, title)
	case "run_failed":
		return fmt.Sprintf("%s 任务失败，需要处理", subject)
	case "session_start":
		return fmt.Sprintf("%s 会话已开始", subject)
	default:
		if title == "" {
			return fmt.Sprintf("%s 有新的通知", subject)
		}
		return fmt.Sprintf("%s：%s", subject, title)
	}
}

func xiaoduSpeechSubject(msg Message) string {
	agent := displayAgentName(msg.Agent)
	project := trimSpeechText(shortenWorkspace(msg.Workspace))
	if project == "" {
		return agent
	}
	return fmt.Sprintf("%s 在 %s", agent, project)
}

func displayAgentName(agent string) string {
	switch agent {
	case "codex":
		return "Codex"
	case "zcode":
		return "ZCode"
	case "claude", "claude_code":
		return "Claude Code"
	default:
		return "Agent Notify"
	}
}

func trimSpeechText(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= maxXiaoduSpeechRunes {
		return text
	}
	return string(runes[:maxXiaoduSpeechRunes])
}
