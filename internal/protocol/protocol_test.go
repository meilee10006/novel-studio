package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeTextRejectsTraversalAbsoluteSymlinkAndOversize(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"../escape.json", filepath.Join(outside, "secret.json"), "escape/secret.json"} {
		if _, err := ReadUTF8(root, rel, 1024); err == nil {
			t.Fatalf("ReadUTF8(%q) unexpectedly succeeded", rel)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "large.md"), []byte(strings.Repeat("x", 33)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUTF8(root, "large.md", 32); err == nil {
		t.Fatal("oversize input unexpectedly accepted")
	}
}

func TestAtomicWriteRejectsSymlinkParentAndNonProtocolExtension(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := WriteUTF8Atomic(root, "escape/pwned.json", []byte("{}"), 0o644); err == nil {
		t.Fatal("write through symlink unexpectedly accepted")
	}
	if err := WriteUTF8Atomic(root, "script.sh", []byte("echo no"), 0o644); err == nil {
		t.Fatal("non-protocol extension unexpectedly accepted")
	}
}

func TestChatGPTProtocolCarriesCurrentVersion(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	if !strings.Contains(text, CurrentVersion) || !strings.Contains(text, "book-1") {
		t.Fatalf("protocol text missing version/project: %q", text)
	}
}

func TestProtocolTextRejectsInvalidUTF8AndExcessiveJSONShape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUTF8(root, "bad.json", 1024); err == nil {
		t.Fatal("invalid UTF-8 unexpectedly accepted")
	}

	deep := strings.Repeat("[", MaxJSONDepth+1) + "0" + strings.Repeat("]", MaxJSONDepth+1)
	var value any
	if err := DecodeJSON([]byte(deep), &value); err == nil {
		t.Fatal("excessive JSON depth unexpectedly accepted")
	}

	wide := "[" + strings.Repeat("0,", MaxJSONArrayLength) + "0]"
	if err := DecodeJSON([]byte(wide), &value); err == nil {
		t.Fatal("excessive JSON array unexpectedly accepted")
	}
}
