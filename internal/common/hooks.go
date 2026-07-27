package common

import "strings"

// 本文件保留两个基于 map[string]any 的小工具。它们曾是四个 agent 包读写
// hooks 的共享实现(issue #21),现已被 hookentries.go 的原字节版本取代——
// 保序改写要求不解析用户没让我们碰的内容。这两个仍在的原因是测试从磁盘
// 读回 JSON 校验结果时用得上。

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
