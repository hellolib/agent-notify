package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWechatSenderRequiresURL(t *testing.T) {
	s := NewWechatSender("")
	err := s.Send(context.Background(), Message{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected error for empty webhook")
	}
}

func TestWechatSenderPostsExpectedPayload(t *testing.T) {
	var gotMethod string
	var gotCT string
	var gotPayload map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok"}`))
	}))
	defer srv.Close()

	s := NewWechatSender(srv.URL)
	msg := Message{
		Title:     "Grok 运行完成",
		Body:      "任务已完成，请查看结果",
		Workspace: `E:\学习ai编程项目\agent-notify`,
		Event:     "run_completed",
		Agent:     "grok",
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if !strings.Contains(gotCT, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", gotCT)
	}
	if gotPayload["msgType"] != "text" {
		t.Fatalf("msgType = %q, want text", gotPayload["msgType"])
	}
	if gotPayload["title"] != msg.Title {
		t.Fatalf("title = %q, want %q", gotPayload["title"], msg.Title)
	}
	if !strings.Contains(gotPayload["content"], msg.Body) {
		t.Fatalf("content = %q, want body", gotPayload["content"])
	}
	if !strings.Contains(gotPayload["content"], "学习ai编程项目") {
		t.Fatalf("content = %q, want shortened Chinese workspace", gotPayload["content"])
	}
}

func TestWechatSenderSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":400,"message":"invalid token"}`))
	}))
	defer srv.Close()

	s := NewWechatSender(srv.URL)
	err := s.Send(context.Background(), Message{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Fatalf("error = %v, want invalid token", err)
	}
}

func TestWechatSenderName(t *testing.T) {
	if NewWechatSender("http://x").Name() != "wechat" {
		t.Fatal("Name() want wechat")
	}
}
