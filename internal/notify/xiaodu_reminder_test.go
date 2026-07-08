package notify

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestXiaoduReminderStoreCancelPermissionByTool(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xiaodu-reminders.json")
	store := NewXiaoduReminderStore(path)

	keep := XiaoduReminder{
		Key:       XiaoduReminderKey("claude_code", "s1", "permission_required", "Edit"),
		Agent:     "claude_code",
		SessionID: "s1",
		Event:     "permission_required",
		ToolName:  "Edit",
		Text:      "keep",
		Remaining: 1,
	}
	cancel := XiaoduReminder{
		Key:       XiaoduReminderKey("claude_code", "s1", "permission_required", "Bash"),
		Agent:     "claude_code",
		SessionID: "s1",
		Event:     "permission_required",
		ToolName:  "Bash",
		Text:      "cancel",
		Remaining: 1,
	}
	if err := store.Save(keep); err != nil {
		t.Fatalf("Save(keep) error = %v", err)
	}
	if err := store.Save(cancel); err != nil {
		t.Fatalf("Save(cancel) error = %v", err)
	}

	if err := store.CancelPermission("claude_code", "s1", "Bash"); err != nil {
		t.Fatalf("CancelPermission() error = %v", err)
	}

	if _, ok, err := store.Get(cancel.Key); err != nil {
		t.Fatalf("Get(cancel) error = %v", err)
	} else if ok {
		t.Fatal("cancelled reminder still exists")
	}
	if _, ok, err := store.Get(keep.Key); err != nil {
		t.Fatalf("Get(keep) error = %v", err)
	} else if !ok {
		t.Fatal("unrelated tool reminder was removed")
	}
}

func TestXiaoduReminderStoreCancelSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xiaodu-reminders.json")
	store := NewXiaoduReminderStore(path)
	reminder := XiaoduReminder{
		Key:       XiaoduReminderKey("zcode", "s1", "permission_required", "Bash"),
		Agent:     "zcode",
		SessionID: "s1",
		Event:     "permission_required",
		ToolName:  "Bash",
		Text:      "cancel",
		Remaining: 1,
	}
	if err := store.Save(reminder); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := store.CancelSession("zcode", "s1"); err != nil {
		t.Fatalf("CancelSession() error = %v", err)
	}

	if _, ok, err := store.Get(reminder.Key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if ok {
		t.Fatal("session reminder still exists")
	}
}

func TestRunXiaoduReminderWorkerSkipsCancelledReminder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xiaodu-reminders.json")
	store := NewXiaoduReminderStore(path)
	key := XiaoduReminderKey("codex", "s1", "permission_required", "Bash")

	if err := store.Save(XiaoduReminder{
		Key:                   key,
		Agent:                 "codex",
		SessionID:             "s1",
		Event:                 "permission_required",
		ToolName:              "Bash",
		Text:                  "speak",
		Remaining:             1,
		RepeatIntervalSeconds: 1,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.CancelPermission("codex", "s1", "Bash"); err != nil {
		t.Fatalf("CancelPermission() error = %v", err)
	}

	sent := 0
	err := RunXiaoduReminderWorker(context.Background(), XiaoduReminderWorker{
		Store:    store,
		Key:      key,
		Interval: time.Nanosecond,
		SendText: func(context.Context, XiaoduReminder) error {
			sent++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunXiaoduReminderWorker() error = %v", err)
	}
	if sent != 0 {
		t.Fatalf("sent = %d, want 0 for cancelled reminder", sent)
	}
}

func TestRunXiaoduReminderWorkerDoesNotResurrectReminderCancelledDuringSend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xiaodu-reminders.json")
	store := NewXiaoduReminderStore(path)
	key := XiaoduReminderKey("codex", "s1", "permission_required", "Bash")

	if err := store.Save(XiaoduReminder{
		Key:                   key,
		Agent:                 "codex",
		SessionID:             "s1",
		Event:                 "permission_required",
		ToolName:              "Bash",
		Text:                  "speak",
		Remaining:             2,
		RepeatIntervalSeconds: 1,
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	sent := 0
	err := RunXiaoduReminderWorker(context.Background(), XiaoduReminderWorker{
		Store:    store,
		Key:      key,
		Interval: time.Nanosecond,
		SendText: func(context.Context, XiaoduReminder) error {
			sent++
			return store.CancelPermission("codex", "s1", "Bash")
		},
	})
	if err != nil {
		t.Fatalf("RunXiaoduReminderWorker() error = %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent = %d, want 1", sent)
	}
	if _, ok, err := store.Get(key); err != nil {
		t.Fatalf("Get() error = %v", err)
	} else if ok {
		t.Fatal("cancelled reminder was resurrected")
	}
}
