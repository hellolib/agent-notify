package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	return atomicWrite(path, perm, func(tmp *os.File) error {
		_, err := tmp.Write(data)
		return err
	})
}

func AtomicWriteFromReader(path string, reader io.Reader, perm os.FileMode) (int64, error) {
	var copied int64
	err := atomicWrite(path, perm, func(tmp *os.File) error {
		n, err := io.Copy(tmp, reader)
		copied = n
		return err
	})
	if err != nil {
		return 0, err
	}
	return copied, nil
}

func atomicWrite(path string, perm os.FileMode, writeFn func(tmp *os.File) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := writeFn(tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	success = true
	return nil
}
