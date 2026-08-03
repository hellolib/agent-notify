package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadPending(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	req := PendingRequest{
		RequestID: "req-1",
		SessionID: "sess-1",
		ToolName:  "Bash",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := SavePending(statePath, req); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPending(statePath, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ToolName != "Bash" {
		t.Fatalf("ToolName = %q, want Bash", loaded.ToolName)
	}
	if loaded.Status != "pending" {
		t.Fatalf("Status = %q, want pending", loaded.Status)
	}
}

func TestResolvePending(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	req := PendingRequest{
		RequestID: "req-2",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := SavePending(statePath, req); err != nil {
		t.Fatal(err)
	}

	if err := ResolvePending(statePath, "req-2", "approved", "allow"); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPending(statePath, "req-2")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "approved" {
		t.Fatalf("Status = %q, want approved", loaded.Status)
	}
	if loaded.Action != "allow" {
		t.Fatalf("Action = %q, want allow", loaded.Action)
	}
	if loaded.ResolvedAt.IsZero() {
		t.Fatal("ResolvedAt should be set")
	}
}

func TestPendingExists(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	req := PendingRequest{RequestID: "req-3", Status: "pending", CreatedAt: time.Now()}
	_ = SavePending(statePath, req)

	if !PendingExists(statePath, "req-3") {
		t.Fatal("should exist")
	}
	if PendingExists(statePath, "nope") {
		t.Fatal("should not exist")
	}
}

func TestRemovePending(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	req := PendingRequest{RequestID: "req-4", Status: "pending", CreatedAt: time.Now()}
	_ = SavePending(statePath, req)
	RemovePending(statePath, "req-4")

	if PendingExists(statePath, "req-4") {
		t.Fatal("should be removed")
	}
}

func TestCleanExpiredPending(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	// 写一个 pending 请求，然后修改文件时间为很久以前
	req := PendingRequest{RequestID: "req-old", Status: "pending", CreatedAt: time.Now()}
	_ = SavePending(statePath, req)
	oldTime := time.Now().Add(-20 * time.Minute)
	_ = os.Chtimes(filepath.Join(PendingDir(statePath), "req-old.json"), oldTime, oldTime)

	CleanExpiredPending(statePath, 10*time.Minute)
	if PendingExists(statePath, "req-old") {
		t.Fatal("old request should be cleaned")
	}
}
