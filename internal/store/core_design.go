package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func coreDesignArtifactPath(digest string) string {
	return filepath.Join("meta", "core", "design", "objects", digest+".json")
}

func coreDesignBundlePath(digest string) string {
	return filepath.Join("meta", "core", "design", "bundles", digest+".json")
}

func coreDesignCommitPath(root string) string {
	return filepath.Join("meta", "core", "design", "commits", root+".json")
}

const coreDesignHeadPath = "meta/core/design/HEAD.json"

func (s *CoreStore) SaveCoreDesignArtifact(digest string, data []byte) error {
	if digest == "" {
		return fmt.Errorf("design artifact digest is required")
	}
	return s.saveImmutableCoreDesignFile(coreDesignArtifactPath(digest), data)
}

func (s *CoreStore) ReadCoreDesignArtifact(digest string) ([]byte, error) {
	return s.io.ReadFile(coreDesignArtifactPath(digest))
}

func (s *CoreStore) SaveCoreDesignBundle(digest string, data []byte) error {
	if digest == "" {
		return fmt.Errorf("design bundle digest is required")
	}
	return s.saveImmutableCoreDesignFile(coreDesignBundlePath(digest), data)
}

func (s *CoreStore) ReadCoreDesignBundle(digest string) ([]byte, error) {
	return s.io.ReadFile(coreDesignBundlePath(digest))
}

func (s *CoreStore) SaveCoreDesignCommit(root string, commit *domain.CoreDesignCommit) error {
	if root == "" {
		return fmt.Errorf("design root is required")
	}
	if commit == nil {
		return fmt.Errorf("design commit is required")
	}
	data, err := json.MarshalIndent(commit, "", "  ")
	if err != nil {
		return err
	}
	return s.saveImmutableCoreDesignFile(coreDesignCommitPath(root), data)
}

func (s *CoreStore) LoadCoreDesignCommit(root string) (*domain.CoreDesignCommit, error) {
	var commit domain.CoreDesignCommit
	if err := s.io.ReadJSON(coreDesignCommitPath(root), &commit); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &commit, nil
}

func (s *CoreStore) SaveCoreDesignHead(head *domain.CoreDesignHead) error {
	if head == nil {
		return fmt.Errorf("design head is required")
	}
	data, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		return err
	}
	return s.saveImmutableCoreDesignFile(coreDesignHeadPath, data)
}

func (s *CoreStore) LoadCoreDesignHead() (*domain.CoreDesignHead, error) {
	var head domain.CoreDesignHead
	if err := s.io.ReadJSON(coreDesignHeadPath, &head); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &head, nil
}

func (s *CoreStore) saveImmutableCoreDesignFile(rel string, data []byte) error {
	return s.io.WithWriteLock(func() error {
		existing, err := s.io.ReadFileUnlocked(rel)
		if err == nil {
			if bytes.Equal(existing, data) {
				return nil
			}
			return fmt.Errorf("conflicting immutable design data at %s", rel)
		}
		if !os.IsNotExist(err) {
			return err
		}
		return s.io.WriteFileUnlocked(rel, data)
	})
}
