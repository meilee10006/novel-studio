package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func coreDesignSubmissionRecordPath(submissionID string) string {
	return filepath.Join("meta", "core", "design", "reconcile", submissionID+".json")
}

func coreDesignSnapshotPath(submissionID string) string {
	return filepath.Join("meta", "core", "design", "snapshots", submissionID)
}

func (s *CoreStore) LoadCoreDesignSubmissionRecord(submissionID string) (*domain.CoreDesignSubmissionRecord, error) {
	if submissionID == "" || submissionID == "." || submissionID == ".." || filepath.Base(submissionID) != submissionID {
		return nil, fmt.Errorf("invalid design submission id %q", submissionID)
	}
	var record domain.CoreDesignSubmissionRecord
	if err := s.io.ReadJSON(coreDesignSubmissionRecordPath(submissionID), &record); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *CoreStore) SaveCoreDesignSubmissionRecord(record *domain.CoreDesignSubmissionRecord) error {
	if record == nil || record.SubmissionID == "" || record.SubmissionID == "." || record.SubmissionID == ".." || filepath.Base(record.SubmissionID) != record.SubmissionID {
		return fmt.Errorf("valid design submission record is required")
	}
	return s.io.WriteJSON(coreDesignSubmissionRecordPath(record.SubmissionID), record)
}

func (s *CoreStore) SaveCoreDesignSnapshot(submissionID string, files map[string][]byte) error {
	if submissionID == "" || submissionID == "." || submissionID == ".." || filepath.Base(submissionID) != submissionID {
		return fmt.Errorf("invalid design submission id %q", submissionID)
	}
	base := s.io.path(coreDesignSnapshotPath(submissionID))
	if _, err := os.Stat(base); err == nil {
		return compareExistingSnapshot(base, files)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(base), ".design-snapshot-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		if name == "" || filepath.Base(name) != name {
			return fmt.Errorf("invalid design snapshot artifact %q", name)
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return err
		}
	}
	return os.Rename(tmp, base)
}

func (s *CoreStore) ReadCoreDesignSnapshotFile(submissionID, name string) ([]byte, error) {
	if submissionID == "" || submissionID == "." || submissionID == ".." || filepath.Base(submissionID) != submissionID {
		return nil, fmt.Errorf("invalid design submission id %q", submissionID)
	}
	if name == "" || filepath.Base(name) != name {
		return nil, fmt.Errorf("invalid design snapshot artifact %q", name)
	}
	return s.io.ReadFile(filepath.Join(coreDesignSnapshotPath(submissionID), name))
}

func coreDesignReceiptPath(submissionID string) string {
	return filepath.Join("meta", "core", "design", "receipts", submissionID+".json")
}

func (s *CoreStore) CompareAndSwapCoreDesignHead(expectedRoot string, head *domain.CoreDesignHead) error {
	if head == nil || head.DesignRoot == "" {
		return fmt.Errorf("design head with root is required")
	}
	data, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		return err
	}
	return s.io.WithWriteLock(func() error {
		currentRoot := ""
		existing, err := s.io.ReadFileUnlocked(coreDesignHeadPath)
		if err == nil {
			var current domain.CoreDesignHead
			if err := json.Unmarshal(existing, &current); err != nil {
				return err
			}
			currentRoot = current.DesignRoot
		} else if !os.IsNotExist(err) {
			return err
		}
		if currentRoot != expectedRoot {
			return fmt.Errorf("design head CAS mismatch: current=%q expected=%q", currentRoot, expectedRoot)
		}
		return s.io.WriteFileUnlocked(coreDesignHeadPath, data)
	})
}

func (s *CoreStore) SaveCoreDesignReceipt(receipt *domain.CoreDesignReceipt) (string, error) {
	if receipt == nil || receipt.SubmissionID == "" || receipt.SubmissionID == "." || receipt.SubmissionID == ".." || filepath.Base(receipt.SubmissionID) != receipt.SubmissionID {
		return "", fmt.Errorf("valid design receipt submission id is required")
	}
	rel := coreDesignReceiptPath(receipt.SubmissionID)
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", err
	}
	if err := s.saveImmutableCoreDesignFile(rel, data); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *CoreStore) LoadCoreDesignReceipt(submissionID string) (*domain.CoreDesignReceipt, error) {
	if submissionID == "" || submissionID == "." || submissionID == ".." || filepath.Base(submissionID) != submissionID {
		return nil, fmt.Errorf("invalid design receipt submission id %q", submissionID)
	}
	var receipt domain.CoreDesignReceipt
	if err := s.io.ReadJSON(coreDesignReceiptPath(submissionID), &receipt); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &receipt, nil
}

type CoreDesignCommitEntry struct {
	Root   string
	Commit domain.CoreDesignCommit
}

func (s *CoreStore) ListCoreDesignArtifactDigests() ([]string, error) {
	return s.listCoreDesignJSONNames(filepath.Join("meta", "core", "design", "objects"))
}

func (s *CoreStore) ListCoreDesignBundleDigests() ([]string, error) {
	return s.listCoreDesignJSONNames(filepath.Join("meta", "core", "design", "bundles"))
}

func (s *CoreStore) ListCoreDesignCommits() ([]CoreDesignCommitEntry, error) {
	roots, err := s.listCoreDesignJSONNames(filepath.Join("meta", "core", "design", "commits"))
	if err != nil {
		return nil, err
	}
	out := make([]CoreDesignCommitEntry, 0, len(roots))
	for _, root := range roots {
		var commit domain.CoreDesignCommit
		if err := s.io.ReadJSON(coreDesignCommitPath(root), &commit); err != nil {
			return nil, fmt.Errorf("read design commit %s: %w", root, err)
		}
		out = append(out, CoreDesignCommitEntry{Root: root, Commit: commit})
	}
	return out, nil
}

func (s *CoreStore) ListCoreDesignReceipts() ([]domain.CoreDesignReceipt, error) {
	dirRel := filepath.Join("meta", "core", "design", "receipts")
	names, err := s.listCoreDesignJSONNames(dirRel)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CoreDesignReceipt, 0, len(names))
	for _, name := range names {
		var receipt domain.CoreDesignReceipt
		if err := s.io.ReadJSON(filepath.Join(dirRel, name+".json"), &receipt); err != nil {
			return nil, fmt.Errorf("read design receipt %s: %w", name, err)
		}
		if receipt.SubmissionID != name {
			return nil, fmt.Errorf(
				"design receipt filename %q does not match submission_id %q",
				name, receipt.SubmissionID,
			)
		}
		out = append(out, receipt)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SubmissionID < out[j].SubmissionID
	})
	return out, nil
}

func (s *CoreStore) listCoreDesignJSONNames(dirRel string) ([]string, error) {
	dir := s.io.path(dirRel)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("design store contains symlink: %s", filepath.Join(dirRel, entry.Name()))
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("design store contains non-regular entry: %s", filepath.Join(dirRel, entry.Name()))
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if name == "" {
			return nil, fmt.Errorf("design store contains invalid json filename: %s", filepath.Join(dirRel, entry.Name()))
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
