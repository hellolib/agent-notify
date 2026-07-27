package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 本文件是四个 agent 包共享的 settings 读写形状。它们的 Install / Uninstall /
// IsInstalled 曾是逐行相同的四份拷贝(CLAUDE.md:「加一个 agent 就是加一个同
// 形状的包」),差异只在 marker、托管事件列表,以及事件对象挂在文件的哪一层
// (claude/codex/grok 是顶层 hooks,zcode 是 hooks.events)。

// ReadOrderedSettings 以保序形式读入一个 JSON 配置文件。
// 文件不存在或为空时返回空对象,让调用方走「新建」路径。
func ReadOrderedSettings(path string) (OrderedObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OrderedObject{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return OrderedObject{}, nil
	}
	return DecodeOrderedObject(data)
}

// WriteOrderedSettings 缩进写出保序对象。未被改动的子树以原始字节透传,
// MarshalIndent 只统一缩进,键序与数值字面量原样保留。
func WriteOrderedSettings(path string, settings OrderedObject) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	// 用户配置文件:原子写 + 覆盖式 .bak 备份(issue #29)
	return WriteFileAtomicWithBackup(path, out, 0o644)
}

// ChildObject 取出 key 对应的子对象。键不存在或为 null 时返回空对象。
// 值存在但不是对象时报错——调用方需要据此决定是拒绝写入还是原样跳过。
func ChildObject(parent OrderedObject, key string) (OrderedObject, error) {
	raw, ok := parent.Get(key)
	if !ok {
		return OrderedObject{}, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return OrderedObject{}, nil
	}
	return DecodeOrderedObject(trimmed)
}

// SetChildObject 把子对象写回 parent 的 key 下,保持 key 的原有位置。
func SetChildObject(parent *OrderedObject, key string, child OrderedObject) error {
	encoded, err := json.Marshal(child)
	if err != nil {
		return err
	}
	parent.Set(key, encoded)
	return nil
}

// InstallManagedHooks 在 events 对象里为每个托管事件登记一条 hook。
//
// 已存在托管 hook 的事件只同步 command(二进制路径可能已过期,issue #34),
// 不重复追加。用户挂在同一事件下的其它 entry 一个字节都不会被碰。
//
// 事件的值不是数组时把它交给 onNonArray 处理:返回错误则中止整个安装
// (文件不被写入,用户原值完好),返回 nil 则跳过该事件、保留原值。
func InstallManagedHooks(events *OrderedObject, managed []string, marker, command string,
	onNonArray func(event string, err error) error) error {
	for _, event := range managed {
		raw, _ := events.Get(event)
		entries, err := HookEntries(raw)
		if err != nil {
			if handleErr := onNonArray(event, err); handleErr != nil {
				return handleErr
			}
			continue
		}

		found := false
		for i, entry := range entries {
			updated, hit, _ := SyncEntryCommand(entry, marker, command)
			if hit {
				found = true
				entries[i] = updated
			}
		}
		if !found {
			entry, err := NewManagedEntry(command)
			if err != nil {
				return err
			}
			entries = append(entries, entry)
		}

		encoded, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		events.Set(event, encoded)
	}
	return nil
}

// UninstallManagedHooks 从 events 对象里摘掉所有托管 hook,并删除因此变空的事件。
//
// 非数组形态的事件值原样保留:那里面不可能有我们写的 entry,而卸载不该被
// 用户的无关配置阻塞。
func UninstallManagedHooks(events *OrderedObject, marker string) error {
	for _, event := range events.Keys() {
		raw, _ := events.Get(event)
		entries, err := HookEntries(raw)
		if err != nil {
			continue
		}

		kept := make([]json.RawMessage, 0, len(entries))
		for _, entry := range entries {
			stripped, keep := StripManagedHooks(entry, marker)
			if keep {
				kept = append(kept, stripped)
			}
		}
		if len(kept) == 0 {
			events.Delete(event)
			continue
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return err
		}
		events.Set(event, encoded)
	}
	return nil
}

// HasManagedHook 检测 events 对象里是否已登记我们的 hook。
// 任一托管事件命中即视为已安装。
func HasManagedHook(events OrderedObject, managed []string, marker string) bool {
	for _, event := range managed {
		raw, ok := events.Get(event)
		if !ok {
			continue
		}
		entries, err := HookEntries(raw)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if EntryHasManagedHook(entry, marker) {
				return true
			}
		}
	}
	return false
}

// RefuseNonArrayEvent 构造 InstallManagedHooks 的 onNonArray 策略:报错中止整个安装。
//
// 用户把某个托管事件手写成对象形式时,我们无法安全地往里追加——旧实现把它
// 当成「没有」而整个替换掉,用户的 hook 定义无声消失。拒绝写入让文件保持
// 原样,并告诉用户该改哪里;这与 zcodehooks 对无法识别的 hooks 键的处理一致。
//
// pathPrefix 是事件在文件里的父路径,用于让报错指向准确位置
// (claude/codex/grok 是 "hooks",zcode 是 "hooks.events")。
func RefuseNonArrayEvent(pathPrefix string) func(string, error) error {
	return func(event string, err error) error {
		return fmt.Errorf("%s.%s %w;无法安全追加 agent-notify hook,"+
			"请先手动改成数组形式后重试", pathPrefix, event, err)
	}
}
