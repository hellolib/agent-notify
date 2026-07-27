package common

import (
	"encoding/json"
	"testing"
)

func TestHookEntriesTreatsMissingAndNullAsEmpty(t *testing.T) {
	for _, raw := range []string{"", "null", "  "} {
		entries, err := HookEntries(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("HookEntries(%q) error: %v", raw, err)
		}
		if len(entries) != 0 {
			t.Fatalf("HookEntries(%q) = %v, want empty", raw, entries)
		}
	}
}

func TestHookEntriesRejectsNonArray(t *testing.T) {
	cases := map[string]string{
		`{"hooks":[]}`: "对象",
		`"a string"`:   "字符串",
		`42`:           "数字",
		`true`:         "布尔值",
	}
	for raw, wantKind := range cases {
		_, err := HookEntries(json.RawMessage(raw))
		if err == nil {
			t.Fatalf("HookEntries(%s) should fail", raw)
		}
		if !contains(err.Error(), wantKind) {
			t.Fatalf("HookEntries(%s) error = %q, want it to mention %q", raw, err, wantKind)
		}
	}
}

func TestHookEntriesSplitsArrayIntoRawElements(t *testing.T) {
	entries, err := HookEntries(json.RawMessage(`[{"a":1},{"b":2}]`))
	if err != nil {
		t.Fatalf("HookEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if string(entries[0]) != `{"a":1}` || string(entries[1]) != `{"b":2}` {
		t.Fatalf("entries = %s / %s", entries[0], entries[1])
	}
}

func TestEntryHasManagedHook(t *testing.T) {
	managed := json.RawMessage(`{"hooks":[{"type":"command","command":"/bin/an handle-claude-hook"}]}`)
	if !EntryHasManagedHook(managed, "handle-claude-hook") {
		t.Fatal("managed entry not detected")
	}

	foreign := json.RawMessage(`{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`)
	if EntryHasManagedHook(foreign, "handle-claude-hook") {
		t.Fatal("user entry misdetected as managed")
	}

	// 形态异常不应 panic,也不该误判为托管
	for _, raw := range []string{`"text"`, `[1,2]`, `{"hooks":"not-an-array"}`, `{}`} {
		if EntryHasManagedHook(json.RawMessage(raw), "handle-claude-hook") {
			t.Fatalf("%s misdetected as managed", raw)
		}
	}
}

func TestSyncEntryCommandLeavesForeignEntryByteIdentical(t *testing.T) {
	// 用户写的 entry:键序 matcher 在前、缩进带空格、还有个自定义键
	original := json.RawMessage(`{"matcher": "Bash", "note": "mine", "hooks": [{"type":"command","command":"echo hi"}]}`)

	out, found, changed := SyncEntryCommand(original, "handle-claude-hook", "/new/path handle-claude-hook")

	if found || changed {
		t.Fatalf("found=%v changed=%v, want both false", found, changed)
	}
	if string(out) != string(original) {
		t.Fatalf("用户的 entry 被改写了:\n got %s\nwant %s", out, original)
	}
}

func TestSyncEntryCommandRewritesOnlyTheManagedHook(t *testing.T) {
	// 同一 entry 内既有用户的 hook 也有我们的:只有我们那条该被重写
	original := json.RawMessage(`{"matcher":"Bash","hooks":[` +
		`{"type":"command","command":"echo hi","extra":true},` +
		`{"type":"command","command":"/old/path handle-claude-hook"}]}`)

	out, found, changed := SyncEntryCommand(original, "handle-claude-hook", "/new/path handle-claude-hook")

	if !found || !changed {
		t.Fatalf("found=%v changed=%v, want both true", found, changed)
	}
	if !contains(string(out), `/new/path handle-claude-hook`) {
		t.Fatalf("command not updated: %s", out)
	}
	// 用户那条 hook 的自定义键必须还在
	if !contains(string(out), `"extra":true`) {
		t.Fatalf("用户 hook 的 extra 键丢了: %s", out)
	}
	if !contains(string(out), `"matcher":"Bash"`) {
		t.Fatalf("matcher 丢了: %s", out)
	}
}

func TestSyncEntryCommandNoopWhenCommandAlreadyCurrent(t *testing.T) {
	original := json.RawMessage(`{"hooks":[{"type":"command","command":"/bin/an handle-claude-hook"}]}`)

	out, found, changed := SyncEntryCommand(original, "handle-claude-hook", "/bin/an handle-claude-hook")

	if !found {
		t.Fatal("managed hook should be found")
	}
	if changed {
		t.Fatal("command is already current, nothing should change")
	}
	if string(out) != string(original) {
		t.Fatalf("bytes changed: %s", out)
	}
}

func TestStripManagedHooksKeepsForeignEntryByteIdentical(t *testing.T) {
	original := json.RawMessage(`{"matcher": "Bash", "hooks": [{"type":"command","command":"echo hi"}]}`)

	out, keep := StripManagedHooks(original, "handle-claude-hook")

	if !keep {
		t.Fatal("user entry must be kept")
	}
	if string(out) != string(original) {
		t.Fatalf("用户的 entry 被改写了:\n got %s\nwant %s", out, original)
	}
}

func TestStripManagedHooksDropsEntryWhenOnlyManagedHookRemains(t *testing.T) {
	original := json.RawMessage(`{"hooks":[{"type":"command","command":"/bin/an handle-claude-hook"}]}`)

	out, keep := StripManagedHooks(original, "handle-claude-hook")

	if keep {
		t.Fatalf("entry should be dropped, got %s", out)
	}
}

func TestStripManagedHooksKeepsSiblingHooksInMixedEntry(t *testing.T) {
	original := json.RawMessage(`{"matcher":"Bash","hooks":[` +
		`{"type":"command","command":"echo hi"},` +
		`{"type":"command","command":"/bin/an handle-claude-hook"}]}`)

	out, keep := StripManagedHooks(original, "handle-claude-hook")

	if !keep {
		t.Fatal("entry still has a user hook, must be kept")
	}
	if contains(string(out), "handle-claude-hook") {
		t.Fatalf("managed hook not removed: %s", out)
	}
	if !contains(string(out), "echo hi") || !contains(string(out), `"matcher":"Bash"`) {
		t.Fatalf("user content lost: %s", out)
	}
}

func TestNewManagedEntry(t *testing.T) {
	raw, err := NewManagedEntry("/bin/an handle-claude-hook")
	if err != nil {
		t.Fatalf("NewManagedEntry: %v", err)
	}
	if !EntryHasManagedHook(raw, "handle-claude-hook") {
		t.Fatalf("built entry is not recognised as managed: %s", raw)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}
