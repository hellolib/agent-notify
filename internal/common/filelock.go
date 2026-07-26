package common

import (
	"os"
	"time"
)

// FileLock 是基于 OS 建议锁(flock / LockFileEx)的跨进程互斥锁。
// 锁由内核管理:持锁进程退出(包括被 kill)时自动释放,不存在陈旧锁问题。
// hook 进程绝不能阻塞宿主 agent,因此获取锁带超时:超时后调用方应放行
// 继续执行(fail-open),而不是等待或报错。
type FileLock struct {
	f *os.File
}

// lockRetryInterval 是非阻塞抢锁失败后的重试间隔。
const lockRetryInterval = 20 * time.Millisecond

// AcquireFileLock 在 timeout 内尝试获取 path 上的排他锁。
// 成功返回锁(调用方负责 Release);超时或出错返回 nil ——
// 调用方据此 fail-open,不应视为致命错误。
func AcquireFileLock(path string, timeout time.Duration) *FileLock {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for {
		ok, err := tryLock(f)
		if ok {
			return &FileLock{f: f}
		}
		if err != nil || time.Now().After(deadline) {
			f.Close()
			return nil
		}
		time.Sleep(lockRetryInterval)
	}
}

// Release 释放锁。对 nil 接收者安全,配合 AcquireFileLock 返回 nil 的
// fail-open 路径,调用方可以无条件 defer Release。
func (l *FileLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unlock(l.f)
	_ = l.f.Close()
	l.f = nil
}
