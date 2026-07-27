package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// 本文件是 hook 条目的「原字节优先」操作层,与 hooks.go 里基于 map 的旧实现
// 并存。区别在于:这里的每个函数在不需要改动时都原样返回传入的字节,只有
// 确实含我们托管 hook 的条目才会被解析和重新编码。用户手写的 entry
// (含 matcher、自定义键、缩进风格)因此一个字节都不会被我们碰过。

// HookEntries 把某个事件下的原始值解析成 entry 列表。
//
// raw 为空(键不存在)或 JSON null 时返回空列表——可以安全追加。
// raw 是其它非数组形态时报错:用户手写成对象形式的 hook 定义不能被当成
// 「没有」而静默替换或删除掉。
func HookEntries(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] != '[' {
		return nil, fmt.Errorf("期望数组,实际是%s", jsonKindOf(trimmed))
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(trimmed, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// NewManagedEntry 构造一个只挂着我们那条 command hook 的 entry。
func NewManagedEntry(command string) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": command},
		},
	})
}

// EntryHasManagedHook 判断 entry 内是否挂着 command 含 marker 的 hook。
func EntryHasManagedHook(entry json.RawMessage, marker string) bool {
	hooks, ok := entryHooks(entry)
	if !ok {
		return false
	}
	for _, h := range hooks {
		if strings.Contains(hookCommand(h), marker) {
			return true
		}
	}
	return false
}

// SyncEntryCommand 把 entry 里托管 hook 的 command 更新为 want,用于修复
// 二进制路径过期的情况(issue #34)。返回 (新字节, 是否含托管 hook, 是否改动)。
// 不含托管 hook 或无需改动时,原字节原样返回。
func SyncEntryCommand(entry json.RawMessage, marker, want string) (json.RawMessage, bool, bool) {
	obj, hooks, ok := entryParts(entry)
	if !ok {
		return entry, false, false
	}

	found, changed := false, false
	for i, h := range hooks {
		cmd := hookCommand(h)
		if !strings.Contains(cmd, marker) {
			continue
		}
		found = true
		if cmd == want {
			continue
		}
		// 只重写这一条 hook:同一 entry 内用户挂的其它 hook 保持原字节
		hookObj, err := DecodeOrderedObject(h)
		if err != nil {
			continue
		}
		quoted, err := json.Marshal(want)
		if err != nil {
			continue
		}
		hookObj.Set("command", quoted)
		rewritten, err := json.Marshal(hookObj)
		if err != nil {
			continue
		}
		hooks[i] = rewritten
		changed = true
	}
	if !changed {
		return entry, found, false
	}
	out, err := rebuildEntry(obj, hooks)
	if err != nil {
		return entry, found, false
	}
	return out, found, true
}

// StripManagedHooks 从 entry 中摘掉所有托管 hook,用于卸载。
// 返回 (新字节, 是否保留这个 entry)。摘光后 entry 不再有 hook,返回 keep=false
// 由调用方丢弃。entry 不含托管 hook 时原字节原样返回。
func StripManagedHooks(entry json.RawMessage, marker string) (json.RawMessage, bool) {
	obj, hooks, ok := entryParts(entry)
	if !ok {
		// 解析不了或没有 hooks 键:不是我们写的,原样保留
		return entry, true
	}

	kept := make([]json.RawMessage, 0, len(hooks))
	removed := false
	for _, h := range hooks {
		if strings.Contains(hookCommand(h), marker) {
			removed = true
			continue
		}
		kept = append(kept, h)
	}
	if !removed {
		return entry, true
	}
	if len(kept) == 0 {
		return nil, false
	}
	out, err := rebuildEntry(obj, kept)
	if err != nil {
		return entry, true
	}
	return out, true
}

// entryParts 把 entry 拆成保序的顶层对象与它的 hooks 数组。
func entryParts(entry json.RawMessage) (OrderedObject, []json.RawMessage, bool) {
	obj, err := DecodeOrderedObject(entry)
	if err != nil {
		return nil, nil, false
	}
	raw, ok := obj.Get("hooks")
	if !ok {
		return nil, nil, false
	}
	var hooks []json.RawMessage
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return nil, nil, false
	}
	return obj, hooks, true
}

// entryHooks 只取 entry 的 hooks 数组,不关心其余键。
func entryHooks(entry json.RawMessage) ([]json.RawMessage, bool) {
	_, hooks, ok := entryParts(entry)
	return hooks, ok
}

func rebuildEntry(obj OrderedObject, hooks []json.RawMessage) (json.RawMessage, error) {
	encoded, err := json.Marshal(hooks)
	if err != nil {
		return nil, err
	}
	obj.Set("hooks", encoded)
	return json.Marshal(obj)
}

// hookCommand 取单条 hook 的 command 字段;形态不符时返回空串。
func hookCommand(raw json.RawMessage) string {
	var h struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return ""
	}
	return h.Command
}

// jsonKindOf 给出一段 JSON 的形态名,用于让报错信息指出用户到底写了什么。
func jsonKindOf(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "空"
	}
	switch trimmed[0] {
	case '{':
		return "对象"
	case '[':
		return "数组"
	case '"':
		return "字符串"
	case 't', 'f':
		return "布尔值"
	case 'n':
		return "null"
	default:
		return "数字"
	}
}
