package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFreezeStateActiveAndBlocks(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	st := FreezeState{
		Until:    now.Add(time.Hour),
		Channels: []string{"feishu", "slack"},
	}
	if !st.Active(now) {
		t.Fatal("expected active freeze")
	}
	if !st.Blocks("feishu", now) {
		t.Fatal("expected feishu blocked")
	}
	if st.Blocks("system", now) {
		t.Fatal("system should not be blocked")
	}
	if st.Active(now.Add(2 * time.Hour)) {
		t.Fatal("expired freeze should be inactive")
	}
	if st.Blocks("feishu", now.Add(2*time.Hour)) {
		t.Fatal("expired freeze should not block")
	}

	empty := FreezeState{Until: now.Add(time.Hour)}
	if empty.Active(now) {
		t.Fatal("empty channels should be inactive")
	}
}

func TestFreezeStoreSetLoadClear(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "freeze.json")
	store := NewFreezeStore(path)
	now := time.Now()
	until := now.Add(30 * time.Minute)

	if err := store.Set(until, []string{"feishu", "feishu", "slack", ""}, now); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got := store.Load()
	if !got.Active(now) {
		t.Fatal("expected active after Set")
	}
	if !got.Until.Equal(until) {
		t.Fatalf("Until = %v, want %v", got.Until, until)
	}
	if len(got.Channels) != 2 || got.Channels[0] != "feishu" || got.Channels[1] != "slack" {
		t.Fatalf("Channels = %v, want [feishu slack]", got.Channels)
	}
	if !got.Blocks("feishu", now) || !got.Blocks("slack", now) || got.Blocks("system", now) {
		t.Fatalf("unexpected Blocks with channels %v", got.Channels)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	got = store.Load()
	if got.Active(now) {
		t.Fatalf("expected inactive after Clear, got %+v", got)
	}
}

func TestFreezeStoreSetRejectsEmptyChannels(t *testing.T) {
	dir := t.TempDir()
	store := NewFreezeStore(filepath.Join(dir, "freeze.json"))
	now := time.Now()
	if err := store.Set(now.Add(time.Hour), nil, now); err == nil {
		t.Fatal("expected error for empty channels")
	}
	if err := store.Set(now.Add(time.Hour), []string{"", ""}, now); err == nil {
		t.Fatal("expected error for blank-only channels")
	}
}

func TestFreezeStoreSetKeepsExplicitChannels(t *testing.T) {
	dir := t.TempDir()
	store := NewFreezeStore(filepath.Join(dir, "freeze.json"))
	now := time.Now()
	if err := store.Set(now.Add(time.Hour), []string{"feishu"}, now); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got := store.Load()
	if len(got.Channels) != 1 || got.Channels[0] != "feishu" {
		t.Fatalf("Channels = %v, want [feishu]", got.Channels)
	}
	if got.Blocks("system", now) {
		t.Fatal("system must not be blocked unless explicitly set")
	}
}

func TestFreezeStoreSetRejectsPastUntil(t *testing.T) {
	dir := t.TempDir()
	store := NewFreezeStore(filepath.Join(dir, "freeze.json"))
	now := time.Now()
	if err := store.Set(now.Add(-time.Minute), []string{"feishu"}, now); err == nil {
		t.Fatal("expected error for past until")
	}
}

func TestFreezeStoreLoadMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	missing := NewFreezeStore(filepath.Join(dir, "missing.json"))
	if missing.Load().Active(time.Now()) {
		t.Fatal("missing file should be inactive")
	}

	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	corrupt := NewFreezeStore(path)
	if corrupt.Load().Active(time.Now()) {
		t.Fatal("corrupt file should be inactive (fail-open)")
	}
}

func TestFreezePath(t *testing.T) {
	got := FreezePath("/home/u/.agent-notify/state.json")
	want := filepath.Join("/home/u/.agent-notify", "freeze.json")
	if got != want {
		t.Fatalf("FreezePath = %q, want %q", got, want)
	}
}
