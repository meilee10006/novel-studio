package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func (s *CoreStore) LoadCoreControlRecord(messageID string) (*domain.CoreControlRecord, error) {
	var record domain.CoreControlRecord
	rel := filepath.Join("meta", "core", "control", "reconcile", messageID+".json")
	if err := s.io.ReadJSON(rel, &record); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *CoreStore) SaveCoreControlRecord(record *domain.CoreControlRecord) error {
	if record == nil || record.MessageID == "" {
		return fmt.Errorf("control message id is required")
	}
	rel := filepath.Join("meta", "core", "control", "reconcile", record.MessageID+".json")
	return s.io.WriteJSON(rel, record)
}
func (s *CoreStore) SaveCoreControlSnapshot(messageID string, files map[string][]byte) error {
	if messageID == "" {
		return fmt.Errorf("control message id is required")
	}
	base := filepath.Join("meta", "core", "control", "snapshots", messageID)
	if _, err := os.Stat(s.io.path(base)); err == nil {
		return compareExistingSnapshot(s.io.path(base), files)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.io.path(base)), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(s.io.path(base)), ".control-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		if filepath.Base(name) != name {
			return fmt.Errorf("invalid control snapshot file %q", name)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(tmp, s.io.path(base))
}

func (s *CoreStore) ReadCoreControlSnapshotFile(messageID, name string) ([]byte, error) {
	if filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid control snapshot file %q", name)
	}
	return s.io.ReadFile(filepath.Join("meta", "core", "control", "snapshots", messageID, name))
}
