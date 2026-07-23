package notify

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hellolib/agent-notify/internal/state"
)

type fakeSender struct {
	name  string
	err   error
	calls int
}

func (f *fakeSender) Name() string { return f.name }

func (f *fakeSender) Send(_ context.Context, _ Message) error {
	f.calls++
	return f.err
}

func TestDispatcherSendAllDoesNotStopOnSingleChannelFailure(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	fail := &fakeSender{name: "remote", err: errors.New("boom")}
	ok := &fakeSender{name: "system"}

	dispatcher := NewDispatcher(store, 60*time.Second, fail, ok)
	err := dispatcher.SendAll(context.Background(), Message{
		Event:     "permission_required",
		SessionID: "sess-1",
	})

	if err == nil {
		t.Fatal("SendAll() error = nil, want aggregated error")
	}
	if fail.calls != 1 || ok.calls != 1 {
		t.Fatalf("calls = fail:%d ok:%d, want 1/1", fail.calls, ok.calls)
	}
}

func TestDispatcherSendAllDedupeIsPerAgentEventSessionAndSender(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	system := &fakeSender{name: "system"}
	feishu := &fakeSender{name: "feishu"}
	dispatcher := NewDispatcher(store, 60*time.Second, system, feishu)

	msg := Message{
		Agent:     "codex",
		Event:     "permission_required",
		SessionID: "sess-1",
	}
	if err := dispatcher.SendAll(context.Background(), msg); err != nil {
		t.Fatalf("first SendAll() error = %v, want nil", err)
	}
	if err := dispatcher.SendAll(context.Background(), msg); err != nil {
		t.Fatalf("second SendAll() error = %v, want nil", err)
	}

	if system.calls != 1 {
		t.Fatalf("system calls = %d, want 1", system.calls)
	}
	if feishu.calls != 1 {
		t.Fatalf("feishu calls = %d, want 1", feishu.calls)
	}
}

func TestDispatcherSendAllDedupeIDSeparatesPromptsInOneSession(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	sender := &fakeSender{name: "feishu"}
	dispatcher := NewDispatcher(store, 60*time.Second, sender)

	first := Message{
		Agent:     "codex",
		Event:     "input_required",
		SessionID: "sess-1",
		DedupeID:  "call-1",
	}
	second := first
	second.DedupeID = "call-2"
	if err := dispatcher.SendAll(context.Background(), first); err != nil {
		t.Fatalf("first SendAll() error = %v, want nil", err)
	}
	if err := dispatcher.SendAll(context.Background(), second); err != nil {
		t.Fatalf("second SendAll() error = %v, want nil", err)
	}
	if sender.calls != 2 {
		t.Fatalf("sender calls = %d, want 2 for distinct DedupeID values", sender.calls)
	}
}

func TestDispatcherSendAllDedupeIDStillDeduplicatesSamePrompt(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	sender := &fakeSender{name: "feishu"}
	dispatcher := NewDispatcher(store, 60*time.Second, sender)
	msg := Message{
		Agent:     "codex",
		Event:     "input_required",
		SessionID: "sess-1",
		DedupeID:  "call-1",
	}
	if err := dispatcher.SendAll(context.Background(), msg); err != nil {
		t.Fatalf("first SendAll() error = %v, want nil", err)
	}
	if err := dispatcher.SendAll(context.Background(), msg); err != nil {
		t.Fatalf("second SendAll() error = %v, want nil", err)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1 for duplicate DedupeID", sender.calls)
	}
}

func TestDispatcherSendAllRetriesOnlyFailedSendersAfterPartialFailure(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	fail := &fakeSender{name: "remote", err: errors.New("boom")}
	ok := &fakeSender{name: "system"}
	dispatcher := NewDispatcher(store, 60*time.Second, ok, fail)

	msg := Message{
		Agent:     "claude",
		Event:     "permission_required",
		SessionID: "sess-1",
	}
	if err := dispatcher.SendAll(context.Background(), msg); err == nil {
		t.Fatal("first SendAll() error = nil, want aggregated error")
	}

	fail.err = nil
	if err := dispatcher.SendAll(context.Background(), msg); err != nil {
		t.Fatalf("second SendAll() error = %v, want nil", err)
	}

	if ok.calls != 1 {
		t.Fatalf("ok calls = %d, want 1", ok.calls)
	}
	if fail.calls != 2 {
		t.Fatalf("fail calls = %d, want 2", fail.calls)
	}
}

func TestDispatcherSendAllDoesNotDuplicateConcurrentSendsForSameSender(t *testing.T) {
	store := state.NewStore(filepath.Join(t.TempDir(), "state.json"))
	sender := &fakeSender{name: "system"}
	dispatcher := NewDispatcher(store, 60*time.Second, sender)
	msg := Message{
		Agent:     "claude",
		Event:     "permission_required",
		SessionID: "sess-1",
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = dispatcher.SendAll(context.Background(), msg)
	}()
	go func() {
		defer wg.Done()
		_ = dispatcher.SendAll(context.Background(), msg)
	}()
	wg.Wait()

	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1", sender.calls)
	}
}

func TestUnsupportedSenderReturnsExplicitError(t *testing.T) {
	sender := NewUnsupportedSender("plan9")
	err := sender.Send(context.Background(), Message{Title: "hello"})
	if err == nil {
		t.Fatal("Send() error = nil, want unsupported platform error")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Fatalf("Send() error = %v, want unsupported platform message", err)
	}
}
