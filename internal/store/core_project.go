package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

const coreProjectStatePath = "meta/core/project.json"

func (s *CoreStore) LoadCoreProjectState() (*domain.CoreProjectState, error) {
	var state domain.CoreProjectState
	if err := s.io.ReadJSON(coreProjectStatePath, &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (s *CoreStore) SaveCoreProjectState(state *domain.CoreProjectState) error {
	if state == nil {
		return nil
	}
	return s.io.WriteJSON(coreProjectStatePath, state)
}

func coreMigrationReceiptPath(fromSchema, toSchema int) string {
	return filepath.Join("meta", "core", "migrations", fmt.Sprintf("schema-%06d-to-%06d.json", fromSchema, toSchema))
}

func (s *CoreStore) LoadCoreMigrationReceipt(fromSchema, toSchema int) (*domain.CoreMigrationReceipt, error) {
	var receipt domain.CoreMigrationReceipt
	if err := s.io.ReadJSON(coreMigrationReceiptPath(fromSchema, toSchema), &receipt); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &receipt, nil
}

func (s *CoreStore) SaveCoreMigrationReceipt(receipt *domain.CoreMigrationReceipt) (string, error) {
	if receipt == nil {
		return "", fmt.Errorf("migration receipt is required")
	}
	rel := coreMigrationReceiptPath(receipt.FromSchema, receipt.ToSchema)
	if err := s.io.WriteJSON(rel, receipt); err != nil {
		return "", err
	}
	return rel, nil
}

func coreProtocolMigrationReceiptPath(fromProtocol, toProtocol string) string {
	slug := func(value string) string {
		return strings.NewReplacer(".", "_", "/", "_", "\\", "_").Replace(value)
	}
	return filepath.Join("meta", "core", "migrations", fmt.Sprintf("protocol-%s-to-%s.json", slug(fromProtocol), slug(toProtocol)))
}

func (s *CoreStore) LoadCoreProtocolMigrationReceipt(fromProtocol, toProtocol string) (*domain.CoreProtocolMigrationReceipt, error) {
	var receipt domain.CoreProtocolMigrationReceipt
	if err := s.io.ReadJSON(coreProtocolMigrationReceiptPath(fromProtocol, toProtocol), &receipt); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &receipt, nil
}

func (s *CoreStore) SaveCoreProtocolMigrationReceipt(receipt *domain.CoreProtocolMigrationReceipt) (string, error) {
	if receipt == nil {
		return "", fmt.Errorf("protocol migration receipt is required")
	}
	rel := coreProtocolMigrationReceiptPath(receipt.FromProtocol, receipt.ToProtocol)
	if err := s.io.WriteJSON(rel, receipt); err != nil {
		return "", err
	}
	return rel, nil
}
