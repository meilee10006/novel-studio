package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

const coreProjectStatePath = "meta/core/project.json"

func (s *Store) LoadCoreProjectState() (*domain.CoreProjectState, error) {
	var state domain.CoreProjectState
	if err := s.Progress.io.ReadJSON(coreProjectStatePath, &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *Store) SaveCoreProjectState(state *domain.CoreProjectState) error {
	if state == nil {
		return nil
	}
	return s.Progress.io.WriteJSON(coreProjectStatePath, state)
}

func coreMigrationReceiptPath(fromSchema, toSchema int) string {
	return filepath.Join("meta", "core", "migrations", fmt.Sprintf("schema-%06d-to-%06d.json", fromSchema, toSchema))
}

func (s *Store) LoadCoreMigrationReceipt(fromSchema, toSchema int) (*domain.CoreMigrationReceipt, error) {
	var receipt domain.CoreMigrationReceipt
	if err := s.Progress.io.ReadJSON(coreMigrationReceiptPath(fromSchema, toSchema), &receipt); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &receipt, nil
}

func (s *Store) SaveCoreMigrationReceipt(receipt *domain.CoreMigrationReceipt) (string, error) {
	if receipt == nil {
		return "", fmt.Errorf("migration receipt is required")
	}
	rel := coreMigrationReceiptPath(receipt.FromSchema, receipt.ToSchema)
	if err := s.Progress.io.WriteJSON(rel, receipt); err != nil {
		return "", err
	}
	return rel, nil
}
