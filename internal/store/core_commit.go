package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func (s *Store) SaveCoreCommitJournal(j *domain.CoreCommitJournal) error {
	if j == nil || j.AttemptID == "" {
		return fmt.Errorf("commit journal attempt id is required")
	}
	return s.Progress.io.WriteJSON(filepath.Join("meta", "core", "commits", j.AttemptID+".json"), j)
}

func (s *Store) LoadCoreCommitJournal(attemptID string) (*domain.CoreCommitJournal, error) {
	var j domain.CoreCommitJournal
	err := s.Progress.io.ReadJSON(filepath.Join("meta", "core", "commits", attemptID+".json"), &j)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) ListCoreCommitJournals() ([]domain.CoreCommitJournal, error) {
	dir := s.Progress.io.path(filepath.Join("meta", "core", "commits"))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]domain.CoreCommitJournal, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var journal domain.CoreCommitJournal
		if err := s.Progress.io.ReadJSON(filepath.Join("meta", "core", "commits", entry.Name()), &journal); err != nil {
			return nil, err
		}
		out = append(out, journal)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AttemptID < out[j].AttemptID })
	return out, nil
}

func (s *Store) SaveCorePreparedArtifacts(attemptID string, files map[string][]byte) error {
	base := filepath.Join("meta", "core", "commits", attemptID, "artifacts")
	if _, err := os.Stat(s.Progress.io.path(base)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(s.Progress.io.path(base))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".prepared-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		path := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(tmp, s.Progress.io.path(base))
}

func (s *Store) ReadCorePreparedArtifact(attemptID, name string) ([]byte, error) {
	return s.Progress.io.ReadFile(filepath.Join("meta", "core", "commits", attemptID, "artifacts", filepath.FromSlash(name)))
}

func (s *Store) LoadPendingCoreCommitJournal() (*domain.CoreCommitJournal, error) {
	dir := s.Progress.io.path(filepath.Join("meta", "core", "commits"))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending *domain.CoreCommitJournal
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var j domain.CoreCommitJournal
		if err := s.Progress.io.ReadJSON(filepath.Join("meta", "core", "commits", entry.Name()), &j); err != nil {
			return nil, err
		}
		if j.State == "committed" {
			continue
		}
		if pending != nil {
			return nil, fmt.Errorf("multiple pending core commits")
		}
		copy := j
		pending = &copy
	}
	return pending, nil
}
