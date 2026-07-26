//go:build windows

package common

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// tryLock 非阻塞抢排他锁;已被他人持有返回 (false, nil) 供调用方重试。
// LockFileEx 按字节范围加锁,这里锁定整个可能范围(offset 0,长度 MaxUint32
// 的高低双字),与 Unix flock 的整文件语义对齐。
func tryLock(f *os.File) (bool, error) {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		^uint32(0), ^uint32(0),
		ol,
	)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
}

func unlock(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(
		windows.Handle(f.Fd()),
		0,
		^uint32(0), ^uint32(0),
		ol,
	)
}
