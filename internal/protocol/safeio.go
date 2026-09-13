package protocol

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func ReadUTF8(root, rel string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("maxBytes must be positive")
	}
	path, err := safePath(root, rel, false)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds max size %d", rel, maxBytes)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%s is not valid UTF-8", rel)
	}
	return data, nil
}

func ReadJSON(root, rel string, maxBytes int64, dst any) error {
	data, err := ReadUTF8(root, rel, maxBytes)
	if err != nil {
		return err
	}
	return DecodeJSON(data, dst)
}

func WriteUTF8Atomic(root, rel string, data []byte, mode os.FileMode) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%s is not valid UTF-8", rel)
	}
	path, err := safePath(root, rel, true)
	if err != nil {
		return err
	}
	if err := ensureSafeParents(root, filepath.Dir(path)); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink destination %s", rel)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".novel-core-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func safePath(root, rel string, forWrite bool) (string, error) {
	if !allowedExtension(rel) {
		return "", fmt.Errorf("unsupported protocol path %q", rel)
	}
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative: %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %q", rel)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if forWrite {
		if err := os.MkdirAll(rootAbs, 0o755); err != nil {
			return "", err
		}
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	path := filepath.Join(rootResolved, clean)
	inside, err := filepath.Rel(rootResolved, path)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %q", rel)
	}
	if err := rejectExistingSymlinks(rootResolved, clean); err != nil {
		return "", err
	}
	return path, nil
}

func rejectExistingSymlinks(root, rel string) error {
	cur := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed in protocol path %q", rel)
		}
	}
	return nil
}

func ensureSafeParents(root, targetDir string) error {
	rootAbs, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("parent escapes workspace")
	}
	cur := rootAbs
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if os.IsNotExist(err) {
			if err := os.Mkdir(cur, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = os.Lstat(cur)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe parent in protocol path")
		}
	}
	return nil
}

func allowedExtension(rel string) bool {
	ext := strings.ToLower(filepath.Ext(rel))
	return ext == ".json" || ext == ".md" || ext == ".txt"
}
