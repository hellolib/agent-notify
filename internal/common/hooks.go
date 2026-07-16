package common

import "strings"

// EventHasManagedHook reports whether any hook entry under event has a command
// that contains commandMarker (e.g. "handle-claude-hook").
//
// Shared by claudehooks / codexhooks / zcodehooks / grokhooks (issue #21).
func EventHasManagedHook(hooks map[string]any, event, commandMarker string) bool {
	for _, entry := range ToAnySlice(hooks[event]) {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range ToAnySlice(entryMap["hooks"]) {
			if IsManagedHook(h, commandMarker) {
				return true
			}
		}
	}
	return false
}

// IsManagedHook reports whether hook is a map whose command string contains commandMarker.
func IsManagedHook(hook any, commandMarker string) bool {
	m, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.Contains(cmd, commandMarker)
}

// ToAnySlice normalizes JSON-decoded hook arrays ([]any or []map[string]any)
// into []any for append/filter/rewrite.
func ToAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
