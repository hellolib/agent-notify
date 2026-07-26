//go:build !windows

package common

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock 非阻塞抢排他锁;已被他人持有返回 (false, nil) 供调用方重试。
func tryLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return false, err
}

func unlock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}
