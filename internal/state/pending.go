package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const pendingDirName = "pending-requests"

// PendingRequest 描述一个等待远程审批的 Codex 权限请求。
type PendingRequest struct {
	RequestID  string    `json:"request_id"`
	SessionID  string    `json:"session_id"`
	ToolName   string    `json:"tool_name"`
	Workspace  string    `json:"workspace,omitempty"`
	Status     string    `json:"status"` // pending / approved / denied / timeout / expired / info
	Action     string    `json:"action"` // allow / allow_prefix / allow_session / reject
	Body       string    `json:"body,omitempty"`
	Detail     string    `json:"detail,omitempty"`
	TtyPath    string    `json:"tty_path,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

// PendingDir 返回 pending 请求目录路径（与 state.json 同级）。
func PendingDir(statePath string) string {
	return filepath.Join(BaseDir(statePath), pendingDirName)
}

// BaseDir 返回 state 文件所在目录（agent-notify 数据根目录）。
func BaseDir(statePath string) string {
	return filepath.Dir(statePath)
}

// SavePending 写入一个 pending 请求文件。文件名为 <request_id>.json。
func SavePending(statePath string, req PendingRequest) error {
	dir := PendingDir(statePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, req.RequestID+".json"), append(data, '\n'), 0o600)
}

// LoadPending 读取一个 pending 请求。文件不存在时返回错误。
func LoadPending(statePath, requestID string) (PendingRequest, error) {
	data, err := os.ReadFile(filepath.Join(PendingDir(statePath), requestID+".json"))
	if err != nil {
		return PendingRequest{}, err
	}
	var req PendingRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return PendingRequest{}, err
	}
	return req, nil
}

// ErrAlreadyResolved 表示 pending 请求已被处理（非 pending 状态），拒绝重复操作。
var ErrAlreadyResolved = fmt.Errorf("请求已被处理")

// ResolvePending 更新 pending 请求的状态与动作，写入 resolved_at。
// 如果当前状态不是 pending（已被处理），返回 ErrAlreadyResolved 防止重复点击。
func ResolvePending(statePath, requestID, status, action string) error {
	req, err := LoadPending(statePath, requestID)
	if err != nil {
		return err
	}
	if req.Status != "pending" {
		return ErrAlreadyResolved
	}
	req.Status = status
	req.Action = action
	req.ResolvedAt = time.Now()
	return SavePending(statePath, req)
}

// CleanExpiredPending 删除超过 maxAge 的 pending 请求文件。
func CleanExpiredPending(statePath string, maxAge time.Duration) {
	dir := PendingDir(statePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

// PendingExists 检查 pending 请求文件是否存在。
func PendingExists(statePath, requestID string) bool {
	_, err := os.Stat(filepath.Join(PendingDir(statePath), requestID+".json"))
	return err == nil
}

// RemovePending 删除一个 pending 请求文件。
func RemovePending(statePath, requestID string) {
	_ = os.Remove(filepath.Join(PendingDir(statePath), requestID+".json"))
}

// ListPending 返回所有 pending 状态的请求（用于诊断/展示）。
func ListPending(statePath string) ([]PendingRequest, error) {
	dir := PendingDir(statePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []PendingRequest
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var req PendingRequest
		if err := json.Unmarshal(data, &req); err != nil {
			continue
		}
		result = append(result, req)
	}
	return result, nil
}

// String 返回 pending 请求的可读描述。
func (r PendingRequest) String() string {
	return fmt.Sprintf("%s [%s] %s -> %s", r.RequestID, r.Status, r.ToolName, r.Action)
}

// ShortID 返回 ID 的前 8 位用于卡片标题显示，长度不足时返回原值。
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// ExpirePendingBySession 将指定 session 下所有 pending 状态的请求标记为 expired。
// 用于新审批请求发出时，立即使同 session 的历史请求失效。
func ExpirePendingBySession(statePath, sessionID string) int {
	list, err := ListPending(statePath)
	if err != nil {
		return 0
	}
	count := 0
	for _, req := range list {
		if req.SessionID == sessionID && req.Status == "pending" {
			req.Status = "expired"
			req.ResolvedAt = time.Now()
			if err := SavePending(statePath, req); err == nil {
				count++
			}
		}
	}
	return count
}
