package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupRestoreRoundTripPreservesCanon(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章备份正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	backup, err := project.CreateBackup(backupDir)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if backup.CanonRoot != settled.NewCanonRoot || backup.Files == 0 {
		t.Fatalf("backup=%+v settled=%+v", backup, settled)
	}
	restoredDir := filepath.Join(t.TempDir(), "restored")
	restored, err := RestoreBackup(backupDir, restoredDir)
	if err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	status, err := restored.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonRoot != settled.NewCanonRoot {
		t.Fatalf("restored status=%+v want root=%s", status, settled.NewCanonRoot)
	}
	verification, err := restored.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !verification.OK {
		t.Fatalf("restored verify=%+v", verification)
	}
}

func TestRestoreRejectsTamperedBackup(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章备份正文")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	if _, err := project.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	chapter := filepath.Join(backupDir, "payload", "meta", "core", "canon", "artifacts", "chapters", "000001", "chapter.md")
	if err := os.WriteFile(chapter, []byte("被篡改的备份正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored")
	_, err := RestoreBackup(backupDir, target)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "digest") {
		t.Fatalf("RestoreBackup err=%v, want digest failure", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("failed restore created target: %v", statErr)
	}
}

func TestRestoreRefusesExistingTarget(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章备份正文")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	if _, err := project.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := RestoreBackup(backupDir, target)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "exist") {
		t.Fatalf("RestoreBackup err=%v, want existing-target failure", err)
	}
	raw, readErr := os.ReadFile(marker)
	if readErr != nil || string(raw) != "keep" {
		t.Fatalf("existing target modified: raw=%q err=%v", raw, readErr)
	}
}
