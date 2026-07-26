package common

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireFileLockAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	l := AcquireFileLock(path, time.Second)
	if l == nil {
		t.Fatal("expected to acquire lock")
	}
	l.Release()

	// 释放后可再次获取
	l2 := AcquireFileLock(path, time.Second)
	if l2 == nil {
		t.Fatal("expected to re-acquire lock after release")
	}
	l2.Release()
}

func TestReleaseOnNilLockIsSafe(t *testing.T) {
	var l *FileLock
	l.Release() // 不应 panic —— fail-open 路径会无条件 defer Release
}

// TestMain 支持测试二进制自重执行:以 HOLD_FILE_LOCK 模式启动时,
// 本进程只负责「持锁 3 秒」,供跨进程锁测试当作对端进程使用。
func TestMain(m *testing.M) {
	if lockPath := os.Getenv("HOLD_FILE_LOCK"); lockPath != "" {
		l := AcquireFileLock(lockPath, time.Second)
		if l == nil {
			os.Exit(1)
		}
		_ = os.WriteFile(os.Getenv("HOLD_READY_FILE"), nil, 0o644)
		time.Sleep(3 * time.Second)
		l.Release()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestFileLockBlocksOtherProcess 验证锁对「另一个进程」生效——
// 这是生产拓扑(每次 hook 触发都是新进程),进程内 mutex 测不出来。
// 重执行测试二进制自身作为持锁子进程,父进程在此期间抢锁应超时失败。
func TestFileLockBlocksOtherProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns subprocess")
	}

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")
	readyPath := filepath.Join(dir, "ready")

	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "HOLD_FILE_LOCK="+lockPath, "HOLD_READY_FILE="+readyPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	// 等子进程确认持锁
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never acquired lock")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 子进程持锁期间,父进程抢锁应在超时后返回 nil(fail-open)
	start := time.Now()
	l := AcquireFileLock(lockPath, 500*time.Millisecond)
	elapsed := time.Since(start)
	if l != nil {
		l.Release()
		t.Fatal("acquired lock while another process holds it")
	}
	if elapsed < 400*time.Millisecond {
		t.Fatalf("returned too fast (%v), should have retried until timeout", elapsed)
	}
}
