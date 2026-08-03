package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	focusWindowsFileName = "focus-windows.json"
	focusMaxEntries      = 64
	focusMaxAge          = 24 * time.Hour
)

// FocusStore 持久化 session_id -> X11 窗口 ID 的映射，供 Linux 点击聚焦使用。
// SessionStart 时刻抓到的精确窗口写在这里，后续事件（permission/input/…）据此
// 聚焦到正确的终端窗口，而不是按 PID 猜（同一终端进程可能有多个兄弟窗口）。
type FocusStore struct {
	path string
	mu   sync.Mutex
}

type focusEntry struct {
	WindowID string    `json:"window_id"`
	Updated  time.Time `json:"updated"`
}

type focusFile struct {
	Windows map[string]focusEntry `json:"windows"`
}

// FocusWindowsPath 返回与 state.json 同目录下的 focus-windows.json 路径。
func FocusWindowsPath(statePath string) string {
	return filepath.Join(BaseDir(statePath), focusWindowsFileName)
}

// NewFocusStore 构造一个基于给定文件路径的 FocusStore。
func NewFocusStore(path string) *FocusStore {
	return &FocusStore{path: path}
}

// Save 写入/更新一条 session -> window 映射，并顺带做 GC。
// sessionID 或 windowID 为空时忽略。
func (s *FocusStore) Save(sessionID, windowID string, now time.Time) error {
	if sessionID == "" || windowID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ff, err := s.load()
	if err != nil {
		return err
	}
	ff.Windows[sessionID] = focusEntry{WindowID: windowID, Updated: now}
	pruneFocus(ff.Windows, now)
	return s.save(ff)
}

// Load 返回某 session 缓存的窗口 ID；不存在时第二个返回值为 false。
func (s *FocusStore) Load(sessionID string) (string, bool) {
	if sessionID == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	ff, err := s.load()
	if err != nil {
		return "", false
	}
	entry, ok := ff.Windows[sessionID]
	if !ok || entry.WindowID == "" {
		return "", false
	}
	return entry.WindowID, true
}

func (s *FocusStore) load() (focusFile, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return focusFile{Windows: map[string]focusEntry{}}, nil
	}
	if err != nil {
		return focusFile{}, err
	}
	var ff focusFile
	if len(data) > 0 {
		if err := json.Unmarshal(data, &ff); err != nil {
			// 文件损坏时重置为空，不阻塞聚焦。
			return focusFile{Windows: map[string]focusEntry{}}, nil
		}
	}
	if ff.Windows == nil {
		ff.Windows = map[string]focusEntry{}
	}
	return ff, nil
}

func (s *FocusStore) save(ff focusFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ff, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// pruneFocus 先删除超过 focusMaxAge 的过期项，再在超额时按 Updated 从旧到新裁剪到上限。
func pruneFocus(windows map[string]focusEntry, now time.Time) {
	for id, e := range windows {
		if now.Sub(e.Updated) > focusMaxAge {
			delete(windows, id)
		}
	}
	if len(windows) <= focusMaxEntries {
		return
	}
	type kv struct {
		id string
		t  time.Time
	}
	items := make([]kv, 0, len(windows))
	for id, e := range windows {
		items = append(items, kv{id: id, t: e.Updated})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].t.Before(items[j].t) })
	for i := 0; i < len(items)-focusMaxEntries; i++ {
		delete(windows, items[i].id)
	}
}
