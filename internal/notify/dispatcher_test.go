package notify

import (
	"context"
	"errors"
	"os"
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

// store 故障时必须 fail-open 照发(issue #28):漏发的代价远大于重复。
// 把 state.json 路径造成目录,使 load/save 都必然失败。
func TestDispatcherSendAllFailsOpenOnStoreError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := os.MkdirAll(statePath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := state.NewStore(statePath)
	sender := &fakeSender{name: "system"}
	dispatcher := NewDispatcher(store, 60*time.Second, sender)

	err := dispatcher.SendAll(context.Background(), Message{
		Agent:     "claude",
		Event:     "permission_required",
		SessionID: "sess-1",
	})

	if sender.calls != 1 {
		t.Fatalf("sender calls = %d, want 1 (must send despite store failure)", sender.calls)
	}
	if err == nil {
		t.Fatal("SendAll() error = nil, want store error surfaced for logging")
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

func TestDedupeKeySameContentSameKey(t *testing.T) {
	base := Message{Agent: "claude", Event: "run_completed", SessionID: "s1", Title: "A", Body: "done"}
	k := dedupeKey(base, "system", 100)
	parts := strings.Split(k, "\x00")
	if len(parts) != 5 {
		t.Fatalf("key = %q, want 5 NUL-separated segments, got %d", k, len(parts))
	}
	if parts[0] != "claude" || parts[1] != "s1" || parts[2] != "run_completed" || parts[4] != "system" {
		t.Fatalf("unexpected segments: %#v", parts)
	}
}

func TestDedupeKeyDistinguishesContent(t *testing.T) {
	base := Message{Agent: "claude", Event: "run_completed", SessionID: "s1", Title: "A", Body: "done"}
	k1 := dedupeKey(base, "system", 100)

	diffBody := base
	diffBody.Body = "failed"
	if dedupeKey(diffBody, "system", 100) == k1 {
		t.Fatal("different body must produce different key")
	}

	diffTitle := base
	diffTitle.Title = "B"
	if dedupeKey(diffTitle, "system", 100) == k1 {
		t.Fatal("different title must produce different key")
	}
}

func TestDedupeKeyEmptySessionFallsBackToPPID(t *testing.T) {
	msg := Message{Agent: "claude", Event: "run_completed", Title: "A", Body: "done"} // empty SessionID
	if dedupeKey(msg, "system", 111) == dedupeKey(msg, "system", 222) {
		t.Fatal("empty session with different ppid must not collapse")
	}

	withSess := msg
	withSess.SessionID = "s1"
	if dedupeKey(withSess, "system", 111) != dedupeKey(withSess, "system", 222) {
		t.Fatal("non-empty session must ignore ppid")
	}
}
