package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDecodeXiaoduRPCResponseAcceptsDataFirstSSE(t *testing.T) {
	body := []byte("data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n")

	var got xiaoduRPCResponse
	if err := decodeXiaoduRPCResponse(body, &got); err != nil {
		t.Fatalf("decodeXiaoduRPCResponse() error = %v", err)
	}

	var result map[string]bool
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result["ok"] {
		t.Fatalf("result ok = false, want true")
	}
}

func TestXiaoduSenderUsesNearExpiryTokenWhenRefreshSetupMissing(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
	}))
	defer srv.Close()

	s := NewXiaoduSenderWithOAuth(
		srv.URL,
		"access-token",
		"",
		"",
		"",
		time.Now().Add(time.Hour).Unix(),
		"device-id",
		"cuid",
		nil,
	)

	if err := s.Send(context.Background(), Message{Agent: "codex", Event: "run_completed", Title: "done"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestXiaoduRefreshAccessTokenUsesPostForm(t *testing.T) {
	s := NewXiaoduSenderWithOAuth(
		"https://example.invalid/rpc",
		"old-access",
		"refresh-token",
		"client-id",
		"client-secret",
		time.Now().Add(-time.Hour).Unix(),
		"device-id",
		"cuid",
		nil,
	)
	s.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("RawQuery = %q, want empty query", req.URL.RawQuery)
			}
			if got := req.Header.Get("Content-Type"); !strings.Contains(got, "application/x-www-form-urlencoded") {
				t.Fatalf("Content-Type = %q, want form content type", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}
			if values.Get("grant_type") != "refresh_token" {
				t.Fatalf("grant_type = %q, want refresh_token", values.Get("grant_type"))
			}
			if values.Get("refresh_token") != "refresh-token" {
				t.Fatalf("refresh_token = %q, want refresh-token", values.Get("refresh_token"))
			}
			if values.Get("client_id") != "client-id" {
				t.Fatalf("client_id = %q, want client-id", values.Get("client_id"))
			}
			if values.Get("client_secret") != "client-secret" {
				t.Fatalf("client_secret = %q, want client-secret", values.Get("client_secret"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`,
				)),
				Request: req,
			}, nil
		}),
	}

	token, err := s.refreshAccessToken(context.Background())
	if err != nil {
		t.Fatalf("refreshAccessToken() error = %v", err)
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" {
		t.Fatalf("token = %#v, want refreshed tokens", token)
	}
}

func TestXiaoduSpeechTextIncludesProjectName(t *testing.T) {
	msg := Message{
		Agent:     "codex",
		Event:     "permission_required",
		Workspace: "/Users/foo/workspace/github/hellolib/agent-notify",
	}

	got := xiaoduSpeechText(msg)
	want := "Codex 在 hellolib/agent-notify 需要权限确认"
	if got != want {
		t.Fatalf("xiaoduSpeechText() = %q, want %q", got, want)
	}
}

func TestXiaoduSpeechTextFallsBackWithoutProjectName(t *testing.T) {
	msg := Message{Agent: "codex", Event: "permission_required"}

	got := xiaoduSpeechText(msg)
	want := "Codex 需要权限确认"
	if got != want {
		t.Fatalf("xiaoduSpeechText() = %q, want %q", got, want)
	}
}

func TestXiaoduSenderSchedulesActionReminder(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
	}))
	defer srv.Close()

	storePath := t.TempDir() + "/xiaodu-reminders.json"
	s := NewXiaoduSenderWithOptions(XiaoduSenderOptions{
		APIBaseURL:        srv.URL,
		AccessToken:       "access-token",
		DeviceID:          "device-id",
		CUID:              "cuid",
		SpeakCompleted:    boolPtr(true),
		RepeatCount:       2,
		RepeatInterval:    25 * time.Second,
		ReminderStorePath: storePath,
		ScheduleRepeats:   false,
	})

	msg := Message{
		Agent:     "codex",
		Event:     "permission_required",
		SessionID: "s1",
		ToolName:  "Bash",
		Workspace: "/Users/foo/src/agent-notify",
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want immediate speech only", calls)
	}

	store := NewXiaoduReminderStore(storePath)
	reminder, ok, err := store.Get(XiaoduReminderKey("codex", "s1", "permission_required", "Bash"))
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("expected pending reminder")
	}
	if reminder.Remaining != 1 {
		t.Fatalf("Remaining = %d, want 1", reminder.Remaining)
	}
	if reminder.Text != "Codex 在 src/agent-notify 需要权限确认" {
		t.Fatalf("Text = %q, want speech text with project", reminder.Text)
	}
}

func TestXiaoduSenderSkipsCompletedWhenDisabled(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewXiaoduSenderWithOptions(XiaoduSenderOptions{
		APIBaseURL:     srv.URL,
		AccessToken:    "access-token",
		DeviceID:       "device-id",
		CUID:           "cuid",
		SpeakCompleted: boolPtr(false),
	})

	if err := s.Send(context.Background(), Message{Agent: "codex", Event: "run_completed"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want no xiaodu speech when completed disabled", calls)
	}
}

func TestXiaoduSenderOptionsKeepCompletedSpeechEnabledByDefault(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`))
	}))
	defer srv.Close()

	s := NewXiaoduSenderWithOptions(XiaoduSenderOptions{
		APIBaseURL:  srv.URL,
		AccessToken: "access-token",
		DeviceID:    "device-id",
		CUID:        "cuid",
	})

	if err := s.Send(context.Background(), Message{Agent: "codex", Event: "run_completed"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want completed speech enabled by default", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func boolPtr(value bool) *bool {
	return &value
}
