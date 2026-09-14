package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

const (
	coreBackupFormat           = "novel-core-backup.v1"
	coreBackupSchemaVersion    = 1
	maxCoreBackupManifestBytes = 4 << 20
)

type BackupResult struct {
	Path      string `json:"path"`
	CanonRoot string `json:"canon_root"`
	Files     int    `json:"files"`
}

type coreBackupFile struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type coreBackupManifest struct {
	Format              string           `json:"format"`
	BackupSchemaVersion int              `json:"backup_schema_version"`
	CoreSchemaVersion   int              `json:"core_schema_version"`
	ProjectID           string           `json:"project_id"`
	ProtocolVersion     string           `json:"protocol_version"`
	CanonRoot           string           `json:"canon_root"`
	Files               []coreBackupFile `json:"files"`
}

func (p *Project) CreateBackup(destination string) (BackupResult, error) {
	release, err := p.acquireProjectReadLock()
	if err != nil {
		return BackupResult{}, err
	}
	defer release()
	return p.createBackupUnlocked(destination)
}

func (p *Project) createBackupUnlocked(destination string) (BackupResult, error) {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return BackupResult{}, fmt.Errorf("backup destination is required")
	}
	backupRoot, err := filepath.Abs(destination)
	if err != nil {
		return BackupResult{}, err
	}
	if pathsOverlap(p.root, backupRoot) {
		return BackupResult{}, fmt.Errorf("backup destination must be outside the local project root")
	}
	if _, err := os.Lstat(backupRoot); err == nil {
		return BackupResult{}, fmt.Errorf("backup destination already exists: %s", backupRoot)
	} else if !os.IsNotExist(err) {
		return BackupResult{}, err
	}

	projectState, err := p.store.LoadCoreProjectState()
	if err != nil || projectState == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return BackupResult{}, err
	}
	canonRoot := ""
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return BackupResult{}, err
	}
	if head != nil {
		recomputed, err := p.RecomputeCanonRoot()
		if err != nil {
			return BackupResult{}, fmt.Errorf("verify canon before backup: %w", err)
		}
		if recomputed != head.Root {
			return BackupResult{}, fmt.Errorf("verify canon before backup: root mismatch")
		}
		if problems := p.verifyReceiptChain(head); len(problems) > 0 {
			return BackupResult{}, fmt.Errorf("verify receipt chain before backup: %s", strings.Join(problems, "; "))
		}
		canonRoot = head.Root
	}

	parent := filepath.Dir(backupRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return BackupResult{}, err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(backupRoot)+".tmp-*")
	if err != nil {
		return BackupResult{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(stage)
		}
	}()

	payloadRoot := filepath.Join(stage, "payload")
	files, err := p.copyCoreBackupPayload(payloadRoot)
	if err != nil {
		return BackupResult{}, err
	}
	manifest := coreBackupManifest{
		Format:              coreBackupFormat,
		BackupSchemaVersion: coreBackupSchemaVersion,
		CoreSchemaVersion:   projectState.SchemaVersion,
		ProjectID:           projectState.ProjectID,
		ProtocolVersion:     projectState.ProtocolVersion,
		CanonRoot:           canonRoot,
		Files:               files,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BackupResult{}, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), raw, 0o600); err != nil {
		return BackupResult{}, err
	}
	if err := os.Rename(stage, backupRoot); err != nil {
		return BackupResult{}, err
	}
	complete = true
	return BackupResult{Path: backupRoot, CanonRoot: canonRoot, Files: len(files)}, nil
}

func (p *Project) copyCoreBackupPayload(payloadRoot string) ([]coreBackupFile, error) {
	sourceRoot := filepath.Join(p.root, "meta", "core")
	var files []coreBackupFile
	err := filepath.WalkDir(sourceRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in local authority: %s", filePath)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular local authority file: %s", filePath)
		}
		rel, err := filepath.Rel(p.root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "meta/core/project.lock" {
			return nil
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		target := filepath.Join(payloadRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return err
		}
		files = append(files, coreBackupFile{Path: rel, Digest: sha256Bytes(data), Size: int64(len(data))})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func RestoreBackup(source, target string) (*Project, error) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" || target == "" {
		return nil, fmt.Errorf("backup source and restore target are required")
	}
	backupRoot, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	targetRoot, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if pathsOverlap(backupRoot, targetRoot) {
		return nil, fmt.Errorf("restore target must be outside the backup directory")
	}
	if info, err := os.Lstat(targetRoot); err == nil {
		return nil, fmt.Errorf("restore target already exists: %s (%s)", targetRoot, info.Mode())
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	manifest, err := loadCoreBackupManifest(backupRoot)
	if err != nil {
		return nil, err
	}
	payloadFiles, err := validateCoreBackupPayload(backupRoot, manifest)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(targetRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(targetRoot)+".restore-*")
	if err != nil {
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(stage)
		}
	}()
	for _, item := range manifest.Files {
		sourcePath := payloadFiles[item.Path]
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, err
		}
		if int64(len(data)) != item.Size || sha256Bytes(data) != item.Digest {
			return nil, fmt.Errorf("backup digest changed while restoring: %s", item.Path)
		}
		targetPath := filepath.Join(stage, filepath.FromSlash(item.Path))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(targetPath, data, 0o600); err != nil {
			return nil, err
		}
	}

	staged, err := OpenProject(stage)
	if err != nil {
		return nil, fmt.Errorf("open restored backup: %w", err)
	}
	status, err := staged.Status()
	if err != nil {
		return nil, fmt.Errorf("inspect restored backup: %w", err)
	}
	if status.ProjectID != manifest.ProjectID || status.ProtocolVersion != manifest.ProtocolVersion || status.CanonRoot != manifest.CanonRoot {
		return nil, fmt.Errorf("restored backup identity or canon root mismatch")
	}
	verification, err := staged.Verify()
	if err != nil {
		return nil, fmt.Errorf("verify restored backup: %w", err)
	}
	if !verification.OK {
		return nil, fmt.Errorf("verify restored backup: %s", strings.Join(verification.Problems, "; "))
	}
	_ = os.Remove(filepath.Join(stage, "meta", "core", "project.lock"))
	if err := os.Rename(stage, targetRoot); err != nil {
		return nil, err
	}
	complete = true
	return OpenProject(targetRoot)
}

func loadCoreBackupManifest(backupRoot string) (coreBackupManifest, error) {
	var manifest coreBackupManifest
	info, err := os.Lstat(backupRoot)
	if err != nil {
		return manifest, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return manifest, fmt.Errorf("backup source must be a real directory")
	}
	manifestPath := filepath.Join(backupRoot, "manifest.json")
	raw, err := readRegularFileLimited(manifestPath, maxCoreBackupManifestBytes)
	if err != nil {
		return manifest, err
	}
	if err := protocol.DecodeJSON(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("decode backup manifest: %w", err)
	}
	if manifest.Format != coreBackupFormat || manifest.BackupSchemaVersion != coreBackupSchemaVersion {
		return manifest, fmt.Errorf("unsupported backup format or schema version")
	}
	if manifest.CoreSchemaVersion < 0 || manifest.CoreSchemaVersion > coreSchemaVersion {
		return manifest, fmt.Errorf("unsupported backup core schema version %d", manifest.CoreSchemaVersion)
	}
	if manifest.ProtocolVersion != protocol.CurrentVersion {
		return manifest, fmt.Errorf("unsupported backup protocol version %q", manifest.ProtocolVersion)
	}
	if !projectIDPattern.MatchString(manifest.ProjectID) {
		return manifest, fmt.Errorf("invalid backup project id")
	}
	if len(manifest.Files) == 0 {
		return manifest, fmt.Errorf("backup manifest has no files")
	}
	seen := map[string]bool{}
	for _, item := range manifest.Files {
		if err := validateCoreBackupPath(item.Path); err != nil {
			return manifest, err
		}
		if seen[item.Path] {
			return manifest, fmt.Errorf("duplicate backup path %q", item.Path)
		}
		seen[item.Path] = true
		if item.Size < 0 || len(item.Digest) != 64 {
			return manifest, fmt.Errorf("invalid backup file metadata for %s", item.Path)
		}
	}
	return manifest, nil
}

func validateCoreBackupPayload(backupRoot string, manifest coreBackupManifest) (map[string]string, error) {
	expected := make(map[string]coreBackupFile, len(manifest.Files))
	for _, item := range manifest.Files {
		expected[item.Path] = item
	}
	payloadRoot := filepath.Join(backupRoot, "payload")
	actual := map[string]string{}
	err := filepath.WalkDir(payloadRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup payload contains symlink: %s", filePath)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup payload contains non-regular file: %s", filePath)
		}
		rel, err := filepath.Rel(payloadRoot, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if err := validateCoreBackupPath(rel); err != nil {
			return err
		}
		if _, ok := expected[rel]; !ok {
			return fmt.Errorf("backup payload contains unlisted file %s", rel)
		}
		actual[rel] = filePath
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(actual) != len(expected) {
		return nil, fmt.Errorf("backup payload file set does not match manifest")
	}
	for rel, item := range expected {
		filePath, ok := actual[rel]
		if !ok {
			return nil, fmt.Errorf("backup payload is missing %s", rel)
		}
		digest, size, err := digestRegularFile(filePath)
		if err != nil {
			return nil, err
		}
		if size != item.Size || digest != item.Digest {
			return nil, fmt.Errorf("backup digest mismatch for %s", rel)
		}
	}
	return actual, nil
}

func validateCoreBackupPath(rel string) error {
	if rel == "" || strings.Contains(rel, "\\") || strings.HasPrefix(rel, "/") {
		return fmt.Errorf("invalid backup path %q", rel)
	}
	clean := path.Clean(rel)
	if clean != rel || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("invalid backup path %q", rel)
	}
	if !strings.HasPrefix(clean, "meta/core/") || clean == "meta/core/project.lock" {
		return fmt.Errorf("backup path is outside persistent core authority: %q", rel)
	}
	return nil
}

func readRegularFileLimited(filePath string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %s", filePath)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("file exceeds max size: %s", filePath)
	}
	return os.ReadFile(filePath)
}

func digestRegularFile(filePath string) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("expected regular file: %s", filePath)
	}
	hasher := sha256.New()
	n, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), n, nil
}
