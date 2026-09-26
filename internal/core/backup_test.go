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

func TestBackupRestoreRoundTripPreservesChapterQualityArtifacts(t *testing.T) {
	project, workspace, ready := planningChapterReadyProject(t, 3, 1)
	patch := []byte(`{"next_arc":{"id":"arc-2","start_chapter":4,"end_chapter":6,"goal":"第二弧目标"}}`)
	artifacts := validChapterArtifacts(1)
	artifacts["planning_patch.json"] = patch
	artifacts["arc_rehearsal.json"] = validArcRehearsal(t, patch)

	settled := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settled.Result != "ACCEPTED" || settled.PlanningStatus != "accepted" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	if verification, err := project.Verify(); err != nil || !verification.OK {
		t.Fatalf("source verify=%+v err=%v", verification, err)
	}

	backupDir := filepath.Join(t.TempDir(), "quality-backup")
	if _, err := project.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreBackup(backupDir, filepath.Join(t.TempDir(), "quality-restored"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"chapters/000001/chapter_plan.json",
		"chapters/000001/chapter_review.json",
		"chapters/000001/planning_patch.json",
		"chapters/000001/arc_rehearsal.json",
	} {
		if _, err := restored.store.ReadCoreCanonArtifact(name); err != nil {
			t.Fatalf("restored quality artifact %s: %v", name, err)
		}
	}
	if verification, err := restored.Verify(); err != nil || !verification.OK {
		t.Fatalf("restored verify=%+v err=%v", verification, err)
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

func TestBackupRestoreRoundTripPreservesDesignAndCanonRoots(t *testing.T) {
	project, _, _, designRoot := acceptedRequiredDesignProjectForTest(t)
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(t.TempDir(), "backup")
	if _, err := project.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreBackup(
		backupDir,
		filepath.Join(t.TempDir(), "restored"),
	)
	if err != nil {
		t.Fatal(err)
	}

	after, err := restored.Status()
	if err != nil {
		t.Fatal(err)
	}
	if before.CanonRoot != after.CanonRoot ||
		after.DesignHead != designRoot ||
		after.FoundationDesignRoot != designRoot {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
	verification, err := restored.Verify()
	if err != nil || !verification.OK {
		t.Fatalf("verification=%+v err=%v", verification, err)
	}
}

func TestBackupRejectsTamperedDesignStoreBeforeStaging(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	ref := currentStoryConceptRefForTest(t, project)
	_, digest, err := parseDesignArtifactRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, "meta", "core", "design", "objects", digest+".json")
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(t.TempDir(), "backup")
	_, err = project.CreateBackup(backupDir)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "verify design store") {
		t.Fatalf("CreateBackup err=%v", err)
	}
	if _, statErr := os.Stat(backupDir); !os.IsNotExist(statErr) {
		t.Fatalf("failed backup created destination: %v", statErr)
	}
}

func TestRestoreRejectsTamperedDesignBackupEvenWhenManifestMatches(t *testing.T) {
	project, _, _, _ := acceptedRequiredDesignProjectForTest(t)
	ref := currentStoryConceptRefForTest(t, project)
	_, digest, err := parseDesignArtifactRef(ref)
	if err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(t.TempDir(), "backup")
	if _, err := project.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	rel := filepath.ToSlash(filepath.Join("meta", "core", "design", "objects", digest+".json"))
	payloadPath := filepath.Join(backupDir, "payload", filepath.FromSlash(rel))
	tampered := []byte(`{"tampered":true}`)
	if err := os.WriteFile(payloadPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	var manifest coreBackupManifest
	readJSONFile(t, filepath.Join(backupDir, "manifest.json"), &manifest)
	var found bool
	for i := range manifest.Files {
		if manifest.Files[i].Path != rel {
			continue
		}
		manifest.Files[i].Digest = sha256Bytes(tampered)
		manifest.Files[i].Size = int64(len(tampered))
		found = true
		break
	}
	if !found {
		t.Fatalf("design object %s not found in backup manifest", rel)
	}
	writeJSONFile(t, filepath.Join(backupDir, "manifest.json"), manifest)

	target := filepath.Join(t.TempDir(), "restored")
	_, err = RestoreBackup(backupDir, target)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "design") {
		t.Fatalf("RestoreBackup err=%v, want design integrity failure", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("failed restore created target: %v", statErr)
	}
}
