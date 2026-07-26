package state

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/hellolib/agent-notify/internal/common"
)

// lockTimeout 是抢跨进程文件锁的等待上限。hook 进程绝不能阻塞宿主 agent,
// 超时拿不到锁就 fail-open 继续执行——宁可偶发重复通知,不可卡住或漏发。
const lockTimeout = 2 * time.Second

// Store 是文件后端的去重存储。每次 hook 触发都是独立进程,因此
// 跨进程互斥靠 OS 文件锁(common.FileLock),进程内并发靠 mu;
// 「预留」直接持久化进 state 文件(而非进程内存),使并发进程互相可见。
type Store struct {
	path string
	mu   sync.Mutex
}

type fileState struct {
	LastSent map[string]time.Time `json:"last_sent"`
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) lockPath() string {
	return s.path + ".lock"
}

// ShouldSend 只读判断窗口内是否已发送过。load 失败时 fail-open(返回可发送)。
func (s *Store) ShouldSend(key string, window time.Duration, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return true, err
	}

	last, ok := st.LastSent[key]
	if ok && now.Sub(last) < window {
		return false, nil
	}

	return true, nil
}

// ReserveSend 在锁内完成「检查 + 预留」:窗口内已发送(或已被其它进程预留)
// 返回 false;否则立即把 key 写入文件作为预留,使并发进程的同一检查失败。
// 发送失败时调用方应 ClearReservation 回滚,成功后 MarkSent 刷新时间戳。
// store 故障时返回 (true, err):fail-open,调用方照发并自行记录错误。
func (s *Store) ReserveSend(key string, window time.Duration, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock := common.AcquireFileLock(s.lockPath(), lockTimeout)
	defer lock.Release()

	st, err := s.load()
	if err != nil {
		return true, err
	}

	last, ok := st.LastSent[key]
	if ok && now.Sub(last) < window {
		return false, nil
	}

	st.LastSent[key] = now
	if err := s.save(st); err != nil {
		return true, err
	}
	return true, nil
}

// MarkSent 确认发送完成:刷新 key 的时间戳并顺带 GC 过期条目。
// 剔除超出窗口的历史条目:age > window 的键此后永远判定为「可发送」,
// 删除与保留对去重结果等价(无损),却能把 map/文件体积限制在最近一个窗口内,
// 避免内容进 key 后的无限增长。
func (s *Store) MarkSent(key string, window time.Duration, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock := common.AcquireFileLock(s.lockPath(), lockTimeout)
	defer lock.Release()

	st, err := s.load()
	if err != nil {
		return err
	}

	for k, t := range st.LastSent {
		if now.Sub(t) > window {
			delete(st.LastSent, k)
		}
	}

	st.LastSent[key] = now
	return s.save(st)
}

// ClearReservation 回滚 ReserveSend 落盘的预留(发送失败时调用),
// 使同一事件的后续重试不被自己的失败预留挡住。
func (s *Store) ClearReservation(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock := common.AcquireFileLock(s.lockPath(), lockTimeout)
	defer lock.Release()

	st, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := st.LastSent[key]; !ok {
		return nil
	}
	delete(st.LastSent, key)
	return s.save(st)
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
		// 文件损坏(如历史版本的撕裂写)时重置为空,自愈而非永久报错:
		// 报错会级联成「所有通知静默停发」,代价远大于丢一窗口的去重记录。
		return fileState{LastSent: map[string]time.Time{}}, nil
	}
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}
	return st, nil
}

func (s *Store) save(st fileState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return common.WriteFileAtomic(s.path, data, 0o644)
}
