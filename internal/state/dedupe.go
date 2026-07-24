package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	path         string
	mu           sync.Mutex
	reservations map[string]time.Time
}

type fileState struct {
	LastSent map[string]time.Time `json:"last_sent"`
}

func NewStore(path string) *Store {
	return &Store{path: path, reservations: map[string]time.Time{}}
}

func (s *Store) ShouldSend(key string, window time.Duration, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return false, err
	}

	last, ok := st.LastSent[key]
	if ok && now.Sub(last) < window {
		return false, nil
	}

	return true, nil
}

func (s *Store) ReserveSend(key string, window time.Duration, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return false, err
	}

	last, ok := st.LastSent[key]
	if ok && now.Sub(last) < window {
		return false, nil
	}
	if reservedAt, ok := s.reservations[key]; ok && now.Sub(reservedAt) < window {
		return false, nil
	}

	s.reservations[key] = now
	return true, nil
}

func (s *Store) MarkSent(key string, window time.Duration, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return err
	}

	// 剔除超出窗口的历史条目：age > window 的键此后永远判定为「可发送」，
	// 删除与保留对去重结果等价（无损），却能把 map/文件体积限制在最近一个窗口内，
	// 避免内容进 key 后的无限增长。
	for k, t := range st.LastSent {
		if now.Sub(t) > window {
			delete(st.LastSent, k)
		}
	}

	st.LastSent[key] = now
	delete(s.reservations, key)
	return s.save(st)
}

func (s *Store) ClearReservation(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.reservations, key)
	return nil
}

func (s *Store) load() (fileState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{LastSent: map[string]time.Time{}}, nil
	}
	if err != nil {
		return fileState{}, err
	}

	var st fileState
	if err := json.Unmarshal(data, &st); err != nil {
		return fileState{}, err
	}
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}
	return st, nil
}

func (s *Store) save(st fileState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0o644)
}
