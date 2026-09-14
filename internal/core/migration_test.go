package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func TestOpenProjectRejectsUnknownFutureSchema(t *testing.T) {
	_, local, _ := newCapabilityPassedProject(t)
	projectPath := filepath.Join(local, "meta", "core", "project.json")
	var raw map[string]any
	readJSONFile(t, projectPath, &raw)
	raw["schema_version"] = coreSchemaVersion + 100
	writeJSONFile(t, projectPath, raw)

	_, err := OpenProject(local)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "schema") {
		t.Fatalf("OpenProject err=%v, want future schema rejection", err)
	}
}

func TestMigrateKnownLegacySchemaBacksUpAndIsIdempotent(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章迁移正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(local, "meta", "core", "project.json")
	var raw map[string]any
	readJSONFile(t, projectPath, &raw)
	raw["schema_version"] = 0
	writeJSONFile(t, projectPath, raw)

	legacy, err := OpenProject(local)
	if err != nil {
		t.Fatalf("OpenProject legacy: %v", err)
	}
	backupDir := filepath.Join(t.TempDir(), "pre-migration-backup")
	result, err := legacy.Migrate(backupDir)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if result.Result != "MIGRATED" || result.FromSchema != 0 || result.ToSchema != coreSchemaVersion {
		t.Fatalf("migration result=%+v", result)
	}
	if result.CanonRootBefore != before.CanonRoot || result.CanonRootAfter != before.CanonRoot {
		t.Fatalf("migration changed canon root: before=%s result=%+v", before.CanonRoot, result)
	}
	if result.BackupPath != backupDir || result.ReceiptPath == "" {
		t.Fatalf("migration paths=%+v", result)
	}
	if _, err := os.Stat(result.ReceiptPath); err != nil {
		t.Fatalf("migration receipt missing: %v", err)
	}

	var backedUp struct {
		SchemaVersion int `json:"schema_version"`
	}
	readJSONFile(t, filepath.Join(backupDir, "payload", "meta", "core", "project.json"), &backedUp)
	if backedUp.SchemaVersion != 0 {
		t.Fatalf("pre-migration backup schema=%d want 0", backedUp.SchemaVersion)
	}
	var current struct {
		SchemaVersion int `json:"schema_version"`
	}
	readJSONFile(t, projectPath, &current)
	if current.SchemaVersion != coreSchemaVersion {
		t.Fatalf("current schema=%d want %d", current.SchemaVersion, coreSchemaVersion)
	}

	verification, err := legacy.Verify()
	if err != nil || !verification.OK {
		t.Fatalf("Verify after migration=%+v err=%v", verification, err)
	}

	secondBackup := filepath.Join(t.TempDir(), "second-backup")
	second, err := legacy.Migrate(secondBackup)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if second.Result != "NOOP" || second.CanonRootAfter != before.CanonRoot {
		t.Fatalf("second migration=%+v", second)
	}
	if _, err := os.Stat(secondBackup); !os.IsNotExist(err) {
		t.Fatalf("idempotent migration created second backup: %v", err)
	}
}

func TestRestoreAcceptsKnownLegacySchemaBackup(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章旧版备份正文")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	projectPath := filepath.Join(local, "meta", "core", "project.json")
	var raw map[string]any
	readJSONFile(t, projectPath, &raw)
	raw["schema_version"] = 0
	writeJSONFile(t, projectPath, raw)

	legacy, err := OpenProject(local)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(t.TempDir(), "legacy-backup")
	if _, err := legacy.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored-legacy")
	restored, err := RestoreBackup(backupDir, target)
	if err != nil {
		t.Fatalf("RestoreBackup legacy: %v", err)
	}
	state, err := restored.store.LoadCoreProjectState()
	if err != nil || state == nil || state.SchemaVersion != 0 {
		t.Fatalf("restored legacy state=%+v err=%v", state, err)
	}
}

func TestMigratePreparedRecoveryRejectsTamperedBackup(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章迁移恢复正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}

	state, err := project.store.LoadCoreProjectState()
	if err != nil || state == nil {
		t.Fatalf("project state=%+v err=%v", state, err)
	}
	state.SchemaVersion = 0
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}
	legacy, err := OpenProject(local)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(t.TempDir(), "prepared-backup")
	if _, err := legacy.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	receipt := &domain.CoreMigrationReceipt{
		SchemaVersion: coreSchemaVersion, State: "prepared", ProjectID: state.ProjectID,
		FromSchema: 0, ToSchema: coreSchemaVersion, BackupPath: backupDir, CanonRootBefore: status.CanonRoot,
	}
	if _, err := legacy.store.SaveCoreMigrationReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	state.SchemaVersion = coreSchemaVersion
	if err := legacy.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}

	payloadProject := filepath.Join(backupDir, "payload", "meta", "core", "project.json")
	file, err := os.OpenFile(payloadProject, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = legacy.Migrate("")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "backup") {
		t.Fatalf("Migrate recovery err=%v, want tampered backup rejection", err)
	}
}

func TestLegacySchemaRequiresExplicitMigrationBeforeMutation(t *testing.T) {
	project, _, _, _ := acceptedFoundationProject(t)
	state, err := project.store.LoadCoreProjectState()
	if err != nil || state == nil {
		t.Fatalf("project state=%+v err=%v", state, err)
	}
	state.SchemaVersion = 0
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}

	checks := map[string]func() error{
		"reconcile": func() error { return project.Reconcile() },
		"scan submission": func() error {
			_, err := project.ScanActiveSubmission()
			return err
		},
		"settle snapshot": func() error {
			_, err := project.SettleActiveSnapshot()
			return err
		},
		"retry submission": func() error {
			_, err := project.RetryInvalidSubmission()
			return err
		},
		"scan control": func() error {
			_, err := project.ScanControlMessage("legacy-schema-control")
			return err
		},
		"process control": func() error {
			_, err := project.ProcessControlMessage("legacy-schema-control")
			return err
		},
		"settle foundation": func() error {
			_, err := project.SettleFoundation(FoundationSubmission{})
			return err
		},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			err := check()
			if err == nil || (!strings.Contains(strings.ToLower(err.Error()), "schema") && !strings.Contains(strings.ToLower(err.Error()), "migrat")) {
				t.Fatalf("err=%v, want explicit migration requirement", err)
			}
		})
	}
}
