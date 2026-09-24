package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var ErrProjectLocked = errors.New("novel core project is locked by another writer")

func (p *Project) acquireProjectWriteLock() (func(), error) {
	return p.acquireProjectLock(syscall.LOCK_EX)
}

func (p *Project) acquireProjectMutationLock() (func(), error) {
	release, err := p.acquireProjectWriteLock()
	if err != nil {
		return nil, err
	}
	state, err := p.store.LoadCoreProjectState()
	if err != nil {
		release()
		return nil, err
	}
	if state != nil && state.SchemaVersion != coreSchemaVersion {
		release()
		return nil, fmt.Errorf("local core schema version %d requires explicit migration to %d", state.SchemaVersion, coreSchemaVersion)
	}
	if state != nil && state.ProtocolVersion != protocol.CurrentVersion {
		release()
		return nil, fmt.Errorf("local protocol version %q requires explicit migration to %q", state.ProtocolVersion, protocol.CurrentVersion)
	}
	for _, from := range []int{0, 1} {
		receipt, err := p.store.LoadCoreMigrationReceipt(from, coreSchemaVersion)
		if err != nil {
			release()
			return nil, err
		}
		if receipt != nil && receipt.State == "prepared" {
			release()
			return nil, fmt.Errorf("prepared schema migration requires recovery before mutation")
		}
	}
	for _, from := range []string{protocol.LegacyVersion, protocol.PreviousVersion} {
		receipt, err := p.store.LoadCoreProtocolMigrationReceipt(from, protocol.CurrentVersion)
		if err != nil {
			release()
			return nil, err
		}
		if receipt != nil && receipt.State == "prepared" {
			release()
			return nil, fmt.Errorf("prepared protocol migration requires recovery before mutation")
		}
	}
	return release, nil
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
