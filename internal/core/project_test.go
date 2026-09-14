package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectStatusEmptyProject(t *testing.T) {
	root := t.TempDir()
	project, err := OpenProject(root)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Initialized {
		t.Fatalf("Initialized=true, want false")
	}
	if status.Root != root {
		t.Fatalf("Root=%q want %q", status.Root, root)
	}
}

func TestProviderFreeStatusIgnoresLegacyProgress(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"novel_name":"legacy","phase":"writing","total_chapters":99,"completed_chapters":[1]}`)
	if err := os.WriteFile(filepath.Join(root, "meta", "progress.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	project, err := OpenProject(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Initialized {
		t.Fatalf("legacy progress leaked into provider-free status: %+v", status)
	}
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !verification.OK {
		t.Fatalf("legacy progress polluted provider-free verify: %+v", verification)
	}
}
