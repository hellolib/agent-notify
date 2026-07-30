package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/hellolib/agent-notify/internal/common"
)

const freezeFileName = "freeze.json"

// RemoteFreezeChannels 是可冻结的远程推送渠道名（有序列表）。
// 不含 system：人在电脑旁仍希望看到桌面通知。
// 名称必须与 notify.Sender.Name() 对齐。
// 实际默认勾选/写入的渠道由调用方按「当前已配置」解析，不再默认全选本列表。
var RemoteFreezeChannels = []string{
	"feishu",
	"wechat",
	"wechat-work",
	"dingtalk",
	"bark",
	"ntfy",
	"slack",
}

// DefaultFreezeChannels 保留别名，兼容旧引用；语义见 RemoteFreezeChannels。
var DefaultFreezeChannels = RemoteFreezeChannels

// FreezeState 是落盘的冻结快照。until 为零值或已过期表示未冻结。
type FreezeState struct {
	Until    time.Time `json:"until"`
	Channels []string  `json:"channels"`
	Updated  time.Time `json:"updated"`
}

// Active 报告 now 时刻是否处于有效冻结期。
func (s FreezeState) Active(now time.Time) bool {
	return !s.Until.IsZero() && now.Before(s.Until) && len(s.Channels) > 0
}

// Blocks 报告指定 sender 在 now 时刻是否应被静默丢弃。
func (s FreezeState) Blocks(senderName string, now time.Time) bool {
	if !s.Active(now) {
		return false
	}
	return slices.Contains(s.Channels, senderName)
}

// FreezeStore 持久化临时冻结截止时间与渠道列表。
// 与 FocusStore / Store 一样：进程内 mu + 跨进程 flock + 原子写。
// 读失败 / 文件损坏一律视为「未冻结」（fail-open：宁可漏冻，不可永久静音）。
type FreezeStore struct {
	path string
	mu   sync.Mutex
}

// FreezePath 返回与 state.json 同目录下的 freeze.json 路径。
func FreezePath(statePath string) string {
	return filepath.Join(filepath.Dir(statePath), freezeFileName)
}

// NewFreezeStore 构造基于给定文件路径的 FreezeStore。
func NewFreezeStore(path string) *FreezeStore {
	return &FreezeStore{path: path}
}

// Load 读取冻结状态。文件缺失、损坏或 I/O 失败时返回空状态（未冻结）。
func (s *FreezeStore) Load() FreezeState {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return FreezeState{}
	}
	return st
}

// Set 写入/覆盖冻结截止时间与渠道列表。
// until 必须严格晚于 now；channels 必须非空（由调用方按已配置渠道解析默认值）。
func (s *FreezeStore) Set(until time.Time, channels []string, now time.Time) error {
	if !until.After(now) {
		return errors.New("freeze until must be in the future")
	}
	channels = uniqueNonEmpty(channels)
	if len(channels) == 0 {
		return errors.New("freeze channels must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	lock := common.AcquireFileLock(s.path+".lock", lockTimeout)
	defer lock.Release()

	return s.save(FreezeState{
		Until:    until,
		Channels: channels,
		Updated:  now,
	})
}

// Clear 清除冻结状态（写空状态）。文件本就不存在时视为成功。
func (s *FreezeStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lock := common.AcquireFileLock(s.path+".lock", lockTimeout)
	defer lock.Release()

	// 写空状态而非删文件，避免并发读到半删除；Active 对零值 until 返回 false。
	return s.save(FreezeState{Updated: time.Now()})
}

func (s *FreezeStore) load() (FreezeState, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return FreezeState{}, nil
	}
	if err != nil {
		return FreezeState{}, err
	}
	if len(data) == 0 {
		return FreezeState{}, nil
	}
	var st FreezeState
	if err := json.Unmarshal(data, &st); err != nil {
		// 损坏时重置为空，不阻塞通知。
		return FreezeState{}, nil
	}
	return st, nil
}

func (s *FreezeStore) save(st FreezeState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return common.WriteFileAtomic(s.path, data, 0o644)
}

func uniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
