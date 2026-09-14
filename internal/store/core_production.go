package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

const coreProductionStatePath = "meta/core/production.json"

func (s *Store) LoadCoreProductionState() (*domain.CoreProductionState, error) {
	var state domain.CoreProductionState
	if err := s.Progress.io.ReadJSON(coreProductionStatePath, &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *Store) SaveCoreProductionState(state *domain.CoreProductionState) error {
	if state == nil {
		return fmt.Errorf("core production state is required")
	}
	return s.Progress.io.WriteJSON(coreProductionStatePath, state)
}

func (s *Store) SaveCoreReceipt(receipt *domain.CoreReceipt) (string, error) {
	if receipt == nil || receipt.AttemptID == "" {
		return "", fmt.Errorf("receipt attempt id is required")
	}
	rel := filepath.Join("meta", "core", "receipts", receipt.AttemptID+".json")
	if _, err := os.Stat(s.Progress.io.path(rel)); err == nil {
		var existing domain.CoreReceipt
		if err := s.Progress.io.ReadJSON(rel, &existing); err != nil {
			return "", err
		}
		if reflect.DeepEqual(existing, *receipt) {
			return rel, nil
		}
		return "", fmt.Errorf("conflicting receipt already exists for attempt %s", receipt.AttemptID)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := s.Progress.io.WriteJSON(rel, receipt); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *Store) ListCoreReceipts() ([]domain.CoreReceipt, error) {
	dir := s.Progress.io.path(filepath.Join("meta", "core", "receipts"))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]domain.CoreReceipt, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var receipt domain.CoreReceipt
		if err := s.Progress.io.ReadJSON(filepath.Join("meta", "core", "receipts", entry.Name()), &receipt); err != nil {
			return nil, err
		}
		out = append(out, receipt)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AttemptID < out[j].AttemptID })
	return out, nil
}

func (s *Store) SaveCoreCanon(state *domain.CoreCanonState, head *domain.CoreCanonHead, artifacts map[string][]byte) error {
	if state == nil || head == nil {
		return fmt.Errorf("canon state and head are required")
	}
	return s.Progress.io.WithWriteLock(func() error {
		for name, data := range artifacts {
			rel := filepath.Join("meta", "core", "canon", "artifacts", name)
			if err := s.Progress.io.WriteFileUnlocked(rel, data); err != nil {
				return err
			}
		}
		if err := s.Progress.io.WriteJSONUnlocked(filepath.Join("meta", "core", "canon", "state.json"), state); err != nil {
			return err
		}
		return s.Progress.io.WriteJSONUnlocked(filepath.Join("meta", "core", "canon", "head.json"), head)
	})
}

func (s *Store) LoadCoreCanonHead() (*domain.CoreCanonHead, error) {
	var head domain.CoreCanonHead
	if err := s.Progress.io.ReadJSON(filepath.Join("meta", "core", "canon", "head.json"), &head); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &head, nil
}

func (s *Store) ReadCoreCanonStateBytes() ([]byte, error) {
	return s.Progress.io.ReadFile(filepath.Join("meta", "core", "canon", "state.json"))
}

func (s *Store) ReadCoreCanonArtifact(name string) ([]byte, error) {
	return s.Progress.io.ReadFile(filepath.Join("meta", "core", "canon", "artifacts", name))
}

func MarshalCoreJSON(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
