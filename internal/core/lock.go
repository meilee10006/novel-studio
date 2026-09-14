package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var ErrProjectLocked = errors.New("novel core project is locked by another writer")

func (p *Project) acquireProjectWriteLock() (func(), error) {
	return p.acquireProjectLock(syscall.LOCK_EX)
}

func (p *Project) acquireProjectReadLock() (func(), error) {
	return p.acquireProjectLock(syscall.LOCK_SH)
}

func (p *Project) acquireProjectLock(mode int) (func(), error) {
	lockPath := filepath.Join(p.root, "meta", "core", "project.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), mode|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%w: %s", ErrProjectLocked, p.root)
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
