package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreShouldSendBlocksRecentlyMarkedKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	key := "permission_required:sess-1"

	if err := store.MarkSent(key, 60*time.Second, now); err != nil {
		t.Fatalf("MarkSent() error = %v, want nil", err)
	}
	if allow, err := store.ShouldSend(key, 60*time.Second, now.Add(30*time.Second)); err != nil || allow {
		t.Fatalf("ShouldSend() within window = (%v, %v), want (false, nil)", allow, err)
	}
	if allow, err := store.ShouldSend(key, 60*time.Second, now.Add(61*time.Second)); err != nil || !allow {
		t.Fatalf("ShouldSend() after window = (%v, %v), want (true, nil)", allow, err)
	}
}

func TestStoreShouldSendDoesNotDedupeUntilMarkedSent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	key := "claude:permission_required:sess-1:system"

	if allow, err := store.ShouldSend(key, 60*time.Second, now); err != nil || !allow {
		t.Fatalf("first ShouldSend() = (%v, %v), want (true, nil)", allow, err)
	}
	if allow, err := store.ShouldSend(key, 60*time.Second, now.Add(30*time.Second)); err != nil || !allow {
		t.Fatalf("second ShouldSend() before mark = (%v, %v), want (true, nil)", allow, err)
	}
	if err := store.MarkSent(key, 60*time.Second, now.Add(30*time.Second)); err != nil {
		t.Fatalf("MarkSent() error = %v, want nil", err)
	}
	if allow, err := store.ShouldSend(key, 60*time.Second, now.Add(45*time.Second)); err != nil || allow {
		t.Fatalf("third ShouldSend() after mark = (%v, %v), want (false, nil)", allow, err)
	}
}

func TestStoreReserveSendPreventsDuplicateInFlightSend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	key := "claude:permission_required:sess-1:system"

	allow, err := store.ReserveSend(key, 60*time.Second, now)
	if err != nil || !allow {
		t.Fatalf("first ReserveSend() = (%v, %v), want (true, nil)", allow, err)
	}

	allow, err = store.ReserveSend(key, 60*time.Second, now.Add(time.Second))
	if err != nil || allow {
		t.Fatalf("second ReserveSend() = (%v, %v), want (false, nil)", allow, err)
	}

	if err := store.ClearReservation(key); err != nil {
		t.Fatalf("ClearReservation() error = %v", err)
	}

	allow, err = store.ReserveSend(key, 60*time.Second, now.Add(2*time.Second))
	if err != nil || !allow {
		t.Fatalf("third ReserveSend() = (%v, %v), want (true, nil)", allow, err)
	}
}

// 两个 Store 实例共享同一文件 = 生产拓扑(每次 hook 触发都是新进程)。
// 预留必须落盘对彼此可见,进程内存 map 挡不住跨进程重复发送(issue #28)。
func TestStoreReserveSendVisibleAcrossStoreInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	key := "claude:permission_required:sess-1:system"

	allow, err := NewStore(path).ReserveSend(key, 60*time.Second, now)
	if err != nil || !allow {
		t.Fatalf("first instance ReserveSend() = (%v, %v), want (true, nil)", allow, err)
	}

	// 「另一个进程」:全新 Store 实例,只共享文件
	allow, err = NewStore(path).ReserveSend(key, 60*time.Second, now.Add(time.Second))
	if err != nil || allow {
		t.Fatalf("second instance ReserveSend() = (%v, %v), want (false, nil)", allow, err)
	}
}

// 损坏的 state.json 必须自愈为空 state,而不是永久报错——
// 报错会级联为「所有通知静默停发」(issue #28)。
func TestStoreSelfHealsFromCorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"last_sent": {"k": "torn-wri`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	allow, err := store.ReserveSend("key", 60*time.Second, now)
	if err != nil || !allow {
		t.Fatalf("ReserveSend() on corrupted file = (%v, %v), want (true, nil)", allow, err)
	}
	if err := store.MarkSent("key", 60*time.Second, now); err != nil {
		t.Fatalf("MarkSent() should repair the file, got %v", err)
	}

	// 修复后文件应为合法 JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st fileState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("file still corrupted after MarkSent: %v", err)
	}
	if _, ok := st.LastSent["key"]; !ok {
		t.Fatal("repaired file should contain the new key")
	}
}

func TestStoreMarkSentPrunesExpiredEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	window := 10 * time.Second

	if err := store.MarkSent("stale", window, now); err != nil {
		t.Fatalf("MarkSent(stale) error = %v", err)
	}
	// 20s 后写入新键；stale 条目 age=20s > 10s 窗口，应被清理。
	if err := store.MarkSent("fresh", window, now.Add(20*time.Second)); err != nil {
		t.Fatalf("MarkSent(fresh) error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var st fileState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := st.LastSent["stale"]; ok {
		t.Fatal("stale key should have been pruned")
	}
	if _, ok := st.LastSent["fresh"]; !ok {
		t.Fatal("fresh key should remain")
	}
}
