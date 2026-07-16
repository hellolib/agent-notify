package common

import "testing"

func TestToAnySlice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := ToAnySlice(nil); got != nil {
			t.Fatalf("ToAnySlice(nil) = %#v, want nil", got)
		}
	})
	t.Run("[]any", func(t *testing.T) {
		in := []any{"a", 1}
		got := ToAnySlice(in)
		if len(got) != 2 || got[0] != "a" {
			t.Fatalf("ToAnySlice([]any) = %#v", got)
		}
	})
	t.Run("[]map", func(t *testing.T) {
		in := []map[string]any{{"k": "v"}}
		got := ToAnySlice(in)
		if len(got) != 1 {
			t.Fatalf("len = %d", len(got))
		}
		m, ok := got[0].(map[string]any)
		if !ok || m["k"] != "v" {
			t.Fatalf("got %#v", got[0])
		}
	})
}

func TestIsManagedHook(t *testing.T) {
	marker := "handle-claude-hook"
	if !IsManagedHook(map[string]any{"command": "/bin/agent-notify handle-claude-hook"}, marker) {
		t.Fatal("expected managed hook")
	}
	if IsManagedHook(map[string]any{"command": "echo hi"}, marker) {
		t.Fatal("expected unmanaged hook")
	}
	if IsManagedHook("not-a-map", marker) {
		t.Fatal("expected false for non-map")
	}
}

func TestEventHasManagedHook(t *testing.T) {
	marker := "handle-grok-hook"
	hooks := map[string]any{
		"Stop": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "other"},
					map[string]any{"type": "command", "command": "agent-notify handle-grok-hook"},
				},
			},
		},
		"SessionStart": []any{
			map[string]any{
				"hooks": []any{
					map[string]any{"type": "command", "command": "echo only"},
				},
			},
		},
	}
	if !EventHasManagedHook(hooks, "Stop", marker) {
		t.Fatal("Stop should have managed hook")
	}
	if EventHasManagedHook(hooks, "SessionStart", marker) {
		t.Fatal("SessionStart should not have managed hook")
	}
	if EventHasManagedHook(hooks, "Missing", marker) {
		t.Fatal("Missing event should be false")
	}
}
