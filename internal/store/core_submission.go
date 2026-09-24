package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func (s *CoreStore) LoadCoreSubmissionRecord(attemptID string) (*domain.CoreSubmissionRecord, error) {
	rel := filepath.Join("meta", "core", "reconcile", attemptID+".json")
	var record domain.CoreSubmissionRecord
	if err := s.io.ReadJSON(rel, &record); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *CoreStore) SaveCoreSubmissionRecord(record *domain.CoreSubmissionRecord) error {
	if record == nil || record.AttemptID == "" {
		return fmt.Errorf("submission record attempt id is required")
	}
	rel := filepath.Join("meta", "core", "reconcile", record.AttemptID+".json")
	return s.io.WriteJSON(rel, record)
}
func (s *CoreStore) SaveCoreSnapshot(attemptID string, files map[string][]byte) error {
	if attemptID == "" {
		return fmt.Errorf("snapshot attempt id is required")
	}
	base := filepath.Join("meta", "core", "snapshots", attemptID)
	if _, err := os.Stat(s.io.path(base)); err == nil {
		return compareExistingSnapshot(s.io.path(base), files)
	} else if !os.IsNotExist(err) {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(s.io.path(base)), ".snapshot-*")
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(filepath.Dir(s.io.path(base)), 0o755); mkErr != nil {
				return mkErr
			}
			tmp, err = os.MkdirTemp(filepath.Dir(s.io.path(base)), ".snapshot-*")
		}
		if err != nil {
			return err
		}
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		if filepath.Base(name) != name {
			return fmt.Errorf("invalid snapshot artifact %q", name)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(tmp, s.io.path(base))
}

func (s *CoreStore) ReadCoreSnapshotFile(attemptID, name string) ([]byte, error) {
	if filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid snapshot artifact %q", name)
	}
	return s.io.ReadFile(filepath.Join("meta", "core", "snapshots", attemptID, name))
}

func compareExistingSnapshot(base string, files map[string][]byte) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	if len(entries) != len(files) {
		return fmt.Errorf("existing snapshot content differs")
	}
	for _, entry := range entries {
		want, ok := files[entry.Name()]
		if !ok || entry.IsDir() {
			return fmt.Errorf("existing snapshot content differs")
		}
		got, err := os.ReadFile(filepath.Join(base, entry.Name()))
		if err != nil || !bytes.Equal(got, want) {
			return fmt.Errorf("existing snapshot content differs")
		}
	}
	return nil
}

func (s *CoreStore) ReadCoreSnapshot(attemptID string, names []string) (map[string][]byte, error) {
	if attemptID == "" {
		return nil, fmt.Errorf("snapshot attempt id is required")
	}
	base := filepath.Join("meta", "core", "snapshots", attemptID)
	entries, err := os.ReadDir(s.io.path(base))
	if err != nil {
		return nil, err
	}
	required := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" || filepath.Base(name) != name || required[name] {
			return nil, fmt.Errorf("invalid required snapshot artifact %q", name)
		}
		required[name] = true
	}
	if len(entries) != len(required) {
		return nil, fmt.Errorf("snapshot artifact set differs from required files")
	}
	out := make(map[string][]byte, len(required))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !required[entry.Name()] {
			return nil, fmt.Errorf("snapshot artifact set differs from required files")
		}
		data, err := s.io.ReadFile(filepath.Join(base, entry.Name()))
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = data
	}
	return out, nil
}
