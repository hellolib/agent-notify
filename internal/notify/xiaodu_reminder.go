package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	xiaoduReminderLockRetry   = 25 * time.Millisecond
	xiaoduReminderLockTimeout = 5 * time.Second
	xiaoduReminderLockStale   = 2 * time.Minute
)

type XiaoduReminder struct {
	Key                   string    `json:"key"`
	Agent                 string    `json:"agent"`
	SessionID             string    `json:"session_id"`
	Event                 string    `json:"event"`
	ToolName              string    `json:"tool_name,omitempty"`
	Text                  string    `json:"text"`
	Remaining             int       `json:"remaining"`
	RepeatIntervalSeconds int       `json:"repeat_interval_seconds"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type XiaoduReminderStore struct {
	path string
}

type xiaoduReminderFile struct {
	Reminders map[string]XiaoduReminder `json:"reminders"`
}

type XiaoduReminderWorker struct {
	Store    *XiaoduReminderStore
	Key      string
	Interval time.Duration
	SendText func(context.Context, XiaoduReminder) error
}

func NewXiaoduReminderStore(path string) *XiaoduReminderStore {
	return &XiaoduReminderStore{path: path}
}

func XiaoduReminderKey(agent, sessionID, event, toolName string) string {
	parts := []string{
		strings.TrimSpace(agent),
		strings.TrimSpace(sessionID),
		strings.TrimSpace(event),
		strings.TrimSpace(toolName),
	}
	return strings.Join(parts, ":")
}

func (s *XiaoduReminderStore) Save(reminder XiaoduReminder) error {
	if reminder.Key == "" {
		reminder.Key = XiaoduReminderKey(reminder.Agent, reminder.SessionID, reminder.Event, reminder.ToolName)
	}
	if reminder.CreatedAt.IsZero() {
		reminder.CreatedAt = time.Now()
	}
	reminder.UpdatedAt = time.Now()

	return s.withLock(func() error {
		st, err := s.load()
		if err != nil {
			return err
		}
		st.Reminders[reminder.Key] = reminder
		return s.save(st)
	})
}

func (s *XiaoduReminderStore) Get(key string) (XiaoduReminder, bool, error) {
	st, err := s.load()
	if err != nil {
		return XiaoduReminder{}, false, err
	}
	reminder, ok := st.Reminders[key]
	return reminder, ok, nil
}

func (s *XiaoduReminderStore) Cancel(key string) error {
	return s.deleteMatching(func(reminder XiaoduReminder) bool {
		return reminder.Key == key
	})
}

func (s *XiaoduReminderStore) CancelPermission(agent, sessionID, toolName string) error {
	toolName = strings.TrimSpace(toolName)
	return s.deleteMatching(func(reminder XiaoduReminder) bool {
		if reminder.Agent != agent || reminder.SessionID != sessionID || reminder.Event != "permission_required" {
			return false
		}
		return toolName == "" || reminder.ToolName == toolName
	})
}

func (s *XiaoduReminderStore) CancelInput(agent, sessionID string) error {
	return s.deleteMatching(func(reminder XiaoduReminder) bool {
		return reminder.Agent == agent && reminder.SessionID == sessionID && reminder.Event == "input_required"
	})
}

func (s *XiaoduReminderStore) CancelSession(agent, sessionID string) error {
	return s.deleteMatching(func(reminder XiaoduReminder) bool {
		return reminder.Agent == agent && reminder.SessionID == sessionID
	})
}

func (s *XiaoduReminderStore) Update(key string, update func(XiaoduReminder) (XiaoduReminder, bool)) (bool, error) {
	if update == nil {
		return false, fmt.Errorf("xiaodu reminder: update function is nil")
	}
	changed := false
	err := s.withLock(func() error {
		st, err := s.load()
		if err != nil {
			return err
		}
		reminder, ok := st.Reminders[key]
		if !ok {
			return nil
		}
		next, keep := update(reminder)
		if keep {
			next.Key = key
			if next.CreatedAt.IsZero() {
				next.CreatedAt = reminder.CreatedAt
			}
			next.UpdatedAt = time.Now()
			st.Reminders[key] = next
		} else {
			delete(st.Reminders, key)
		}
		changed = true
		return s.save(st)
	})
	return changed, err
}

func (s *XiaoduReminderStore) deleteMatching(match func(XiaoduReminder) bool) error {
	return s.withLock(func() error {
		st, err := s.load()
		if err != nil {
			return err
		}
		changed := false
		for key, reminder := range st.Reminders {
			if match(reminder) {
				delete(st.Reminders, key)
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return s.save(st)
	})
}

func (s *XiaoduReminderStore) load() (xiaoduReminderFile, error) {
	if s.path == "" {
		return xiaoduReminderFile{Reminders: map[string]XiaoduReminder{}}, nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return xiaoduReminderFile{Reminders: map[string]XiaoduReminder{}}, nil
	}
	if err != nil {
		return xiaoduReminderFile{}, err
	}
	var st xiaoduReminderFile
	if err := json.Unmarshal(data, &st); err != nil {
		return xiaoduReminderFile{}, err
	}
	if st.Reminders == nil {
		st.Reminders = map[string]XiaoduReminder{}
	}
	return st, nil
}

func (s *XiaoduReminderStore) save(st xiaoduReminderFile) error {
	if s.path == "" {
		return nil
	}
	if st.Reminders == nil {
		st.Reminders = map[string]XiaoduReminder{}
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func (s *XiaoduReminderStore) withLock(fn func() error) error {
	if s.path == "" {
		return fn()
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	lockPath := s.path + ".lock"
	deadline := time.Now().Add(xiaoduReminderLockTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, _ = fmt.Fprintf(lock, "%d\n", os.Getpid())
			_ = lock.Close()
			defer func() {
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if isStaleXiaoduReminderLock(lockPath) {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("xiaodu reminder: acquire lock %s: timeout", lockPath)
		}
		time.Sleep(xiaoduReminderLockRetry)
	}
}

func isStaleXiaoduReminderLock(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > xiaoduReminderLockStale
}

func RunXiaoduReminderWorker(ctx context.Context, worker XiaoduReminderWorker) error {
	if worker.Store == nil {
		return fmt.Errorf("xiaodu reminder: store is nil")
	}
	if worker.SendText == nil {
		return fmt.Errorf("xiaodu reminder: send function is nil")
	}

	for {
		reminder, ok, err := worker.Store.Get(worker.Key)
		if err != nil {
			return err
		}
		if !ok || reminder.Remaining <= 0 {
			return nil
		}

		interval := worker.Interval
		if interval <= 0 {
			interval = time.Duration(reminder.RepeatIntervalSeconds) * time.Second
		}
		if interval <= 0 {
			interval = 25 * time.Second
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		reminder, ok, err = worker.Store.Get(worker.Key)
		if err != nil {
			return err
		}
		if !ok || reminder.Remaining <= 0 {
			return nil
		}
		if err := worker.SendText(ctx, reminder); err != nil {
			_ = worker.Store.Cancel(worker.Key)
			return err
		}

		updated, err := worker.Store.Update(worker.Key, func(reminder XiaoduReminder) (XiaoduReminder, bool) {
			reminder.Remaining--
			return reminder, reminder.Remaining > 0
		})
		if err != nil {
			return err
		}
		if !updated {
			return nil
		}
	}
}
