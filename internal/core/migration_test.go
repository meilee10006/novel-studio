package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
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

func TestOpenProjectRejectsUnknownProtocolVersion(t *testing.T) {
	_, local, _ := newCapabilityPassedProject(t)
	statePath := filepath.Join(local, "meta", "core", "project.json")
	var state map[string]any
	readJSONFile(t, statePath, &state)
	state["protocol_version"] = "99.0"
	writeJSONFile(t, statePath, state)
	_, err := OpenProject(local)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "protocol") {
		t.Fatalf("OpenProject err=%v, want unknown protocol rejection", err)
	}
}

func TestLegacyProtocolRequiresExplicitMigrationBeforeMutation(t *testing.T) {
	for _, from := range []string{protocol.LegacyVersion, protocol.PreviousVersion} {
		t.Run(from, func(t *testing.T) {
			project, local, _ := newCapabilityPassedProject(t)
			state, err := project.store.LoadCoreProjectState()
			if err != nil || state == nil {
				t.Fatalf("project state=%+v err=%v", state, err)
			}
			state.ProtocolVersion = from
			if err := project.store.SaveCoreProjectState(state); err != nil {
				t.Fatal(err)
			}
			legacy, err := OpenProject(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := legacy.Reconcile(); err == nil ||
				!strings.Contains(strings.ToLower(err.Error()), "migration") {
				t.Fatalf("Reconcile err=%v, want explicit protocol migration requirement", err)
			}
		})
	}
}

func TestMigrateKnownLegacyProtocolPreservesCanonAndRebindsAttempt(t *testing.T) {
	for _, from := range []string{protocol.LegacyVersion, protocol.PreviousVersion} {
		t.Run(from, func(t *testing.T) {
			project, local, workspace, ready := acceptedFoundationProject(t)
			before, err := project.Status()
			if err != nil {
				t.Fatal(err)
			}
			state, err := project.store.LoadCoreProjectState()
			if err != nil || state == nil {
				t.Fatalf("project state=%+v err=%v", state, err)
			}
			state.ProtocolVersion = from
			state.DesignMode = domain.DesignModeLegacy
			if err := project.store.SaveCoreProjectState(state); err != nil {
				t.Fatal(err)
			}
			var workspaceProject map[string]any
			workspaceProjectPath := filepath.Join(workspace, "project.json")
			readJSONFile(t, workspaceProjectPath, &workspaceProject)
			workspaceProject["protocol_version"] = from
			writeJSONFile(t, workspaceProjectPath, workspaceProject)

			legacy, err := OpenProject(local)
			if err != nil {
				t.Fatalf("OpenProject legacy protocol: %v", err)
			}
			backupDir := filepath.Join(t.TempDir(), "pre-protocol-backup")
			result, err := legacy.Migrate(backupDir)
			if err != nil {
				t.Fatalf("Migrate protocol: %v", err)
			}
			if result.Result != "MIGRATED" || result.FromProtocol != from ||
				result.ToProtocol != protocol.CurrentVersion {
				t.Fatalf("protocol migration result=%+v", result)
			}
			if result.CanonRootBefore != before.CanonRoot ||
				result.CanonRootAfter != before.CanonRoot {
				t.Fatalf("protocol migration changed canon root: before=%s result=%+v", before.CanonRoot, result)
			}

			state, err = legacy.store.LoadCoreProjectState()
			if err != nil || state == nil || state.ProtocolVersion != protocol.CurrentVersion ||
				state.DesignMode != domain.DesignModeLegacy {
				t.Fatalf("migrated project state=%+v err=%v", state, err)
			}
			production, err := legacy.store.LoadCoreProductionState()
			if err != nil || production == nil || production.ActiveAttempt == nil {
				t.Fatalf("production=%+v err=%v", production, err)
			}
			if production.ActiveAttempt.AttemptID == ready.AttemptID ||
				production.ActiveAttempt.ProtocolVersion != protocol.CurrentVersion ||
				production.ActiveAttempt.Reason != "protocol_upgrade" {
				t.Fatalf("protocol migration did not rebind attempt: old=%+v new=%+v", ready, production.ActiveAttempt)
			}
			if production.ActiveTask == nil || production.ActiveTask.TaskID != ready.TaskID ||
				production.ActiveTask.Target != ready.Target ||
				production.ActiveTask.BaseCanonRoot != ready.BaseCanonRoot {
				t.Fatalf("protocol migration changed active task: ready=%+v production=%+v", ready, production)
			}
			if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
				t.Fatalf("old READY survived protocol migration: %v", err)
			}
			var challenge capabilityChallenge
			readJSONFile(t, filepath.Join(workspace, "setup", "capability-challenge.json"), &challenge)
			if challenge.ProtocolVersion != protocol.CurrentVersion ||
				challenge.Nonce != state.CapabilityNonce {
				t.Fatalf("challenge=%+v state=%+v", challenge, state)
			}

			target := filepath.Join(t.TempDir(), "restored-legacy-protocol")
			restored, err := RestoreBackup(backupDir, target)
			if err != nil {
				t.Fatalf("RestoreBackup legacy protocol: %v", err)
			}
			restoredState, err := restored.store.LoadCoreProjectState()
			if err != nil || restoredState == nil || restoredState.ProtocolVersion != from {
				t.Fatalf("restored legacy protocol state=%+v err=%v", restoredState, err)
			}

			writeCapabilityAckForTest(t, workspace)
			if err := legacy.Reconcile(); err != nil {
				t.Fatalf("Reconcile after protocol capability ack: %v", err)
			}
			next := readReady(t, workspace)
			if next.AttemptID != production.ActiveAttempt.AttemptID ||
				next.ProtocolVersion != protocol.CurrentVersion ||
				next.TaskID != ready.TaskID {
				t.Fatalf("READY after protocol migration=%+v production=%+v", next, production.ActiveAttempt)
			}
		})
	}
}

func TestPreparedProtocolMigrationRecoversIdempotently(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	state, err := project.store.LoadCoreProjectState()
	if err != nil || state == nil {
		t.Fatalf("project state=%+v err=%v", state, err)
	}
	state.ProtocolVersion = protocol.LegacyVersion
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}
	legacy, err := OpenProject(local)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(t.TempDir(), "prepared-protocol-backup")
	if _, err := legacy.CreateBackup(backupDir); err != nil {
		t.Fatal(err)
	}
	nonce, err := randomNonce()
	if err != nil {
		t.Fatal(err)
	}
	production, err := legacy.store.LoadCoreProductionState()
	if err != nil || production == nil || production.ActiveTask == nil || production.ActiveAttempt == nil {
		t.Fatalf("production=%+v err=%v", production, err)
	}
	next := cloneProductionState(production)
	attempt, err := newAttempt(next, next.ActiveTask, "protocol_upgrade", next.ActiveAttempt.RequiredArtifacts, protocol.CurrentVersion)
	if err != nil {
		t.Fatal(err)
	}
	receipt := &domain.CoreProtocolMigrationReceipt{
		SchemaVersion: coreSchemaVersion, State: "prepared", ProjectID: state.ProjectID,
		CoreSchemaVersion: state.SchemaVersion,
		FromProtocol:      protocol.LegacyVersion, ToProtocol: protocol.CurrentVersion,
		BackupPath: backupDir, CanonRootBefore: before.CanonRoot,
		CapabilityNonce: nonce, MarkdownProbe: "novel-core-capability:" + nonce + "\n",
		NextAttempt: attempt, NextAttemptSeq: next.NextAttemptSeq,
	}
	if _, err := legacy.store.SaveCoreProtocolMigrationReceipt(receipt); err != nil {
		t.Fatal(err)
	}

	state.ProtocolVersion = protocol.CurrentVersion
	state.CapabilityNonce = receipt.CapabilityNonce
	state.MarkdownProbe = receipt.MarkdownProbe
	if err := legacy.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}
	production.ActiveAttempt = attempt
	production.NextAttemptSeq = next.NextAttemptSeq
	if err := legacy.store.SaveCoreProductionState(production); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(workspace, "exchange", "READY.json"))

	result, err := legacy.Migrate("")
	if err != nil {
		t.Fatalf("recover prepared protocol migration: %v", err)
	}
	if result.Result != "MIGRATED" || result.CanonRootAfter != before.CanonRoot {
		t.Fatalf("recovered result=%+v", result)
	}
	stored, err := legacy.store.LoadCoreProtocolMigrationReceipt(protocol.LegacyVersion, protocol.CurrentVersion)
	if err != nil || stored == nil || stored.State != "committed" {
		t.Fatalf("stored protocol receipt=%+v err=%v", stored, err)
	}
	current, err := legacy.store.LoadCoreProductionState()
	if err != nil || current == nil || current.ActiveAttempt == nil || current.ActiveAttempt.AttemptID != attempt.AttemptID || current.ActiveAttempt.AttemptID == ready.AttemptID {
		t.Fatalf("production after recovery=%+v err=%v", current, err)
	}
	second, err := legacy.Migrate(filepath.Join(t.TempDir(), "unused"))
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if second.Result != "NOOP" || second.CanonRootAfter != before.CanonRoot {
		t.Fatalf("second migration=%+v", second)
	}
}

func TestMigrateSchemaOneToTwoPreservesProductionBytesAndSetsLegacyMode(t *testing.T) {
	project, local, _, _ := acceptedFoundationProject(t)
	state, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	state.SchemaVersion = 1
	state.DesignMode = ""
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}
	productionPath := filepath.Join(local, "meta", "core", "production.json")
	before, err := os.ReadFile(productionPath)
	if err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(t.TempDir(), "schema-one")
	result, err := project.Migrate(backup)
	if err != nil {
		t.Fatal(err)
	}
	if result.FromSchema != 1 || result.ToSchema != coreSchemaVersion {
		t.Fatalf("result=%+v", result)
	}
	afterState, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	if afterState.SchemaVersion != coreSchemaVersion || afterState.DesignMode != domain.DesignModeLegacy {
		t.Fatalf("state=%+v", afterState)
	}
	after, err := os.ReadFile(productionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("schema migration changed production bytes\nbefore=%s\nafter=%s", before, after)
	}
}

func TestCurrentReleaseMigratesInTwoExplicitStages(t *testing.T) {
	project, _, _, ready := acceptedFoundationProject(t)

	state, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	state.SchemaVersion = 1
	state.ProtocolVersion = protocol.PreviousVersion
	state.DesignMode = ""
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}

	before, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	beforeTaskID := before.ActiveTask.TaskID
	beforeTarget := before.ActiveTask.Target
	beforeRoot := before.CanonRoot

	schemaBackup := filepath.Join(t.TempDir(), "before-schema-2")
	first, err := project.Migrate(schemaBackup)
	if err != nil {
		t.Fatal(err)
	}
	if first.FromSchema != 1 || first.ToSchema != 2 {
		t.Fatalf("schema migration=%+v", first)
	}

	middle, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	if middle.DesignMode != domain.DesignModeLegacy ||
		middle.ProtocolVersion != protocol.PreviousVersion {
		t.Fatalf("middle state=%+v", middle)
	}

	protocolBackup := filepath.Join(t.TempDir(), "before-protocol-1-1")
	second, err := project.Migrate(protocolBackup)
	if err != nil {
		t.Fatal(err)
	}
	if second.FromProtocol != protocol.PreviousVersion ||
		second.ToProtocol != protocol.CurrentVersion {
		t.Fatalf("protocol migration=%+v", second)
	}

	after, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if after.ActiveTask.TaskID != beforeTaskID ||
		after.ActiveTask.Target != beforeTarget ||
		after.CanonRoot != beforeRoot {
		t.Fatalf("migration changed business progress: before=%+v after=%+v", before, after)
	}
	if after.ActiveAttempt.AttemptID == ready.AttemptID {
		t.Fatal("protocol migration did not rebind active attempt")
	}
}
