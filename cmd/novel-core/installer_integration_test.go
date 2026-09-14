package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptAcceptsGitHubStyleReleaseJSON(t *testing.T) {
	out, installDir, err := runInstallerFixture(t, false)
	if err != nil {
		t.Fatalf("install script failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "SHA-256") {
		t.Fatalf("install output did not confirm checksum verification:\n%s", out)
	}
	installed := filepath.Join(installDir, "novel-core")
	if info, err := os.Stat(installed); err != nil || info.Size() == 0 {
		t.Fatalf("installed binary missing: info=%v err=%v", info, err)
	}
}

func TestInstallScriptRejectsChecksumMismatch(t *testing.T) {
	out, installDir, err := runInstallerFixture(t, true)
	if err == nil {
		t.Fatalf("install script accepted tampered checksum:\n%s", out)
	}
	if !strings.Contains(string(out), "SHA-256 校验失败") {
		t.Fatalf("unexpected checksum failure output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(installDir, "novel-core")); !os.IsNotExist(err) {
		t.Fatalf("installer left binary after checksum failure: %v", err)
	}
}

func runInstallerFixture(t *testing.T, corruptChecksum bool) ([]byte, string, error) {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	tmp := t.TempDir()
	assetName := "novel-studio_9.9.9_Linux_x86_64.tar.gz"
	assetPath := filepath.Join(tmp, assetName)
	writeInstallerFixtureArchive(t, assetPath)

	raw, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	checksum := fmt.Sprintf("%x", sum)
	if corruptChecksum {
		checksum = strings.Repeat("0", 64)
	}
	checksumPath := filepath.Join(tmp, "novel-studio_checksums.txt")
	if err := os.WriteFile(checksumPath, []byte(fmt.Sprintf("%s  %s\n", checksum, assetName)), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeBin := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	curlPath := filepath.Join(fakeBin, "curl")
	curlScript := fmt.Sprintf(`#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -H) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  */releases/tags/v9.9.9)
    cat <<'JSON'
{
  "tag_name": "v9.9.9",
  "assets": [
    {"browser_download_url": "https://example.invalid/%s"},
    {"browser_download_url": "https://example.invalid/novel-studio_checksums.txt"}
  ]
}
JSON
    ;;
  */%s) cp %q "$out" ;;
  */novel-studio_checksums.txt) cp %q "$out" ;;
  *) echo "unexpected curl URL: $url" >&2; exit 22 ;;
esac
`, assetName, assetName, assetPath, checksumPath)
	if err := os.WriteFile(curlPath, []byte(curlScript), 0o755); err != nil {
		t.Fatal(err)
	}

	installDir := filepath.Join(tmp, "install")
	cmd := exec.Command("sh", filepath.Join(root, "scripts", "install.sh"), "v9.9.9")
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+":/usr/bin:/bin",
		"NOVEL_CORE_INSTALL_DIR="+installDir,
		"NOVEL_CORE_VERSION=",
	)
	out, err := cmd.CombinedOutput()
	return out, installDir, err
}

func writeInstallerFixtureArchive(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("#!/bin/sh\necho 'novel-core fixture'\n")
	if err := tw.WriteHeader(&tar.Header{Name: "novel-core", Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		body string
	}{{"README.md", "fixture readme\n"}, {"LICENSE", "Apache License Version 2.0\n"}} {
		data := []byte(item.body)
		if err := tw.WriteHeader(&tar.Header{Name: item.name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
