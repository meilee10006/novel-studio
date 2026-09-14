package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type MigrationResult struct {
	Result          string `json:"result"`
	FromSchema      int    `json:"from_schema"`
	ToSchema        int    `json:"to_schema"`
	FromProtocol    string `json:"from_protocol,omitempty"`
	ToProtocol      string `json:"to_protocol,omitempty"`
	BackupPath      string `json:"backup_path,omitempty"`
	ReceiptPath     string `json:"receipt_path,omitempty"`
	CanonRootBefore string `json:"canon_root_before,omitempty"`
	CanonRootAfter  string `json:"canon_root_after,omitempty"`
}

func (p *Project) Migrate(backupDestination string) (MigrationResult, error) {
	release, err := p.acquireProjectWriteLock()
	if err != nil {
		return MigrationResult{}, err
	}
	defer release()

	state, err := p.store.LoadCoreProjectState()
	if err != nil || state == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return MigrationResult{}, err
	}
	if state.SchemaVersion < 0 || state.SchemaVersion > coreSchemaVersion {
		return MigrationResult{}, fmt.Errorf("unsupported local core schema version %d", state.SchemaVersion)
	}
	if !protocol.IsKnownVersion(state.ProtocolVersion) {
		return MigrationResult{}, fmt.Errorf("unsupported local protocol version %q", state.ProtocolVersion)
	}

	schemaReceipt, err := p.store.LoadCoreMigrationReceipt(0, coreSchemaVersion)
	if err != nil {
		return MigrationResult{}, err
	}
	if schemaReceipt != nil && schemaReceipt.State == "prepared" {
		return p.finishPreparedSchemaMigration(state, schemaReceipt)
	}
	if state.SchemaVersion != coreSchemaVersion {
		return p.migrateSchemaLocked(state, backupDestination, schemaReceipt)
	}

	protocolReceipt, err := p.store.LoadCoreProtocolMigrationReceipt(protocol.LegacyVersion, protocol.CurrentVersion)
	if err != nil {
		return MigrationResult{}, err
	}
	if protocolReceipt != nil && protocolReceipt.State == "prepared" {
		return p.finishPreparedProtocolMigration(state, protocolReceipt)
	}
	if state.ProtocolVersion != protocol.CurrentVersion {
		return p.migrateProtocolLocked(state, backupDestination, protocolReceipt)
	}

	root, err := p.currentCanonRoot()
	if err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{
		Result: "NOOP", FromSchema: coreSchemaVersion, ToSchema: coreSchemaVersion,
		FromProtocol: protocol.CurrentVersion, ToProtocol: protocol.CurrentVersion,
		CanonRootBefore: root, CanonRootAfter: root,
	}, nil
}

func (p *Project) migrateSchemaLocked(state *domain.CoreProjectState, backupDestination string, receipt *domain.CoreMigrationReceipt) (MigrationResult, error) {
	if state.SchemaVersion != 0 {
		return MigrationResult{}, fmt.Errorf("no migration path from schema %d to %d", state.SchemaVersion, coreSchemaVersion)
	}
	backupDestination = strings.TrimSpace(backupDestination)
	if receipt != nil && receipt.State == "prepared" {
		if backupDestination != "" {
			want, err := filepath.Abs(backupDestination)
			if err != nil {
				return MigrationResult{}, err
			}
			if want != receipt.BackupPath {
				return MigrationResult{}, fmt.Errorf("prepared migration backup is %s, not %s", receipt.BackupPath, want)
			}
		}
		if err := validateMigrationBackup(receipt.BackupPath, state, receipt.CanonRootBefore); err != nil {
			return MigrationResult{}, err
		}
	} else {
		backupPath, root, err := p.prepareMigrationBackup(state, backupDestination)
		if err != nil {
			return MigrationResult{}, err
		}
		receipt = &domain.CoreMigrationReceipt{
			SchemaVersion: coreSchemaVersion, State: "prepared", ProjectID: state.ProjectID,
			FromSchema: 0, ToSchema: coreSchemaVersion,
			BackupPath: backupPath, CanonRootBefore: root,
		}
		if _, err := p.store.SaveCoreMigrationReceipt(receipt); err != nil {
			return MigrationResult{}, err
		}
	}
	state.SchemaVersion = coreSchemaVersion
	if err := p.store.SaveCoreProjectState(state); err != nil {
		return MigrationResult{}, err
	}
	return p.finishPreparedSchemaMigration(state, receipt)
}

func (p *Project) finishPreparedSchemaMigration(state *domain.CoreProjectState, receipt *domain.CoreMigrationReceipt) (MigrationResult, error) {
	if receipt == nil || receipt.State != "prepared" || receipt.FromSchema != 0 || receipt.ToSchema != coreSchemaVersion {
		return MigrationResult{}, fmt.Errorf("invalid prepared migration receipt")
	}
	if state.ProjectID != receipt.ProjectID || state.SchemaVersion != coreSchemaVersion {
		return MigrationResult{}, fmt.Errorf("prepared migration state does not match project")
	}
	legacyState := *state
	legacyState.SchemaVersion = receipt.FromSchema
	if err := validateMigrationBackup(receipt.BackupPath, &legacyState, receipt.CanonRootBefore); err != nil {
		return MigrationResult{}, err
	}
	root, err := p.currentCanonRoot()
	if err != nil {
		return MigrationResult{}, err
	}
	if root != receipt.CanonRootBefore {
		return MigrationResult{}, fmt.Errorf("migration changed canon root: before %s after %s", receipt.CanonRootBefore, root)
	}
	receipt.State = "committed"
	receipt.CanonRootAfter = root
	receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rel, err := p.store.SaveCoreMigrationReceipt(receipt)
	if err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{
		Result: "MIGRATED", FromSchema: receipt.FromSchema, ToSchema: receipt.ToSchema,
		FromProtocol: state.ProtocolVersion, ToProtocol: state.ProtocolVersion,
		BackupPath: receipt.BackupPath, ReceiptPath: filepath.Join(p.root, rel),
		CanonRootBefore: receipt.CanonRootBefore, CanonRootAfter: root,
	}, nil
}

func (p *Project) migrateProtocolLocked(state *domain.CoreProjectState, backupDestination string, existing *domain.CoreProtocolMigrationReceipt) (MigrationResult, error) {
	if state.ProtocolVersion != protocol.LegacyVersion {
		return MigrationResult{}, fmt.Errorf("no protocol migration path from %q to %q", state.ProtocolVersion, protocol.CurrentVersion)
	}
	if existing != nil && existing.State == "committed" {
		return MigrationResult{}, fmt.Errorf("protocol migration receipt is committed while project is still legacy")
	}
	backupPath, root, err := p.prepareMigrationBackup(state, backupDestination)
	if err != nil {
		return MigrationResult{}, err
	}
	nonce, err := randomNonce()
	if err != nil {
		return MigrationResult{}, err
	}
	receipt := &domain.CoreProtocolMigrationReceipt{
		SchemaVersion: coreSchemaVersion, State: "prepared", ProjectID: state.ProjectID,
		CoreSchemaVersion: state.SchemaVersion,
		FromProtocol:      state.ProtocolVersion, ToProtocol: protocol.CurrentVersion,
		BackupPath: backupPath, CanonRootBefore: root,
		CapabilityNonce: nonce, MarkdownProbe: "novel-core-capability:" + nonce + "\n",
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return MigrationResult{}, err
	}
	if production != nil && production.ActiveTask != nil && production.ActiveAttempt != nil {
		next := cloneProductionState(production)
		attempt, err := newAttempt(next, next.ActiveTask, "protocol_upgrade", next.ActiveAttempt.RequiredArtifacts, protocol.CurrentVersion)
		if err != nil {
			return MigrationResult{}, err
		}
		receipt.NextAttempt = attempt
		receipt.NextAttemptSeq = next.NextAttemptSeq
		if next.ActiveBlock != nil {
			block := *next.ActiveBlock
			block.AttemptID = attempt.AttemptID
			receipt.NextBlock = &block
		}
	}
	if _, err := p.store.SaveCoreProtocolMigrationReceipt(receipt); err != nil {
		return MigrationResult{}, err
	}
	return p.finishPreparedProtocolMigration(state, receipt)
}

func (p *Project) finishPreparedProtocolMigration(state *domain.CoreProjectState, receipt *domain.CoreProtocolMigrationReceipt) (MigrationResult, error) {
	if receipt == nil || receipt.State != "prepared" || receipt.FromProtocol != protocol.LegacyVersion || receipt.ToProtocol != protocol.CurrentVersion {
		return MigrationResult{}, fmt.Errorf("invalid prepared protocol migration receipt")
	}
	if state.ProjectID != receipt.ProjectID || state.SchemaVersion != receipt.CoreSchemaVersion {
		return MigrationResult{}, fmt.Errorf("prepared protocol migration state does not match project")
	}
	legacyState := *state
	legacyState.ProtocolVersion = receipt.FromProtocol
	if err := validateMigrationBackup(receipt.BackupPath, &legacyState, receipt.CanonRootBefore); err != nil {
		return MigrationResult{}, err
	}
	if state.ProtocolVersion != receipt.FromProtocol && state.ProtocolVersion != receipt.ToProtocol {
		return MigrationResult{}, fmt.Errorf("project protocol does not match prepared migration")
	}
	state.ProtocolVersion = receipt.ToProtocol
	state.CapabilityNonce = receipt.CapabilityNonce
	state.MarkdownProbe = receipt.MarkdownProbe
	if err := p.store.SaveCoreProjectState(state); err != nil {
		return MigrationResult{}, err
	}
	if receipt.NextAttempt != nil {
		production, err := p.store.LoadCoreProductionState()
		if err != nil || production == nil || production.ActiveTask == nil {
			if err == nil {
				err = fmt.Errorf("prepared protocol migration production state is missing")
			}
			return MigrationResult{}, err
		}
		if production.NextAttemptSeq > receipt.NextAttemptSeq && (production.ActiveAttempt == nil || production.ActiveAttempt.AttemptID != receipt.NextAttempt.AttemptID) {
			return MigrationResult{}, fmt.Errorf("production advanced beyond prepared protocol migration")
		}
		attempt := *receipt.NextAttempt
		production.ActiveAttempt = &attempt
		production.NextAttemptSeq = receipt.NextAttemptSeq
		if receipt.NextBlock != nil {
			block := *receipt.NextBlock
			production.ActiveBlock = &block
		} else {
			production.ActiveBlock = nil
		}
		if err := p.store.SaveCoreProductionState(production); err != nil {
			return MigrationResult{}, err
		}
	}
	if err := p.writeProtocolUpgradeWorkspace(state); err != nil {
		return MigrationResult{}, err
	}
	root, err := p.currentCanonRoot()
	if err != nil {
		return MigrationResult{}, err
	}
	if root != receipt.CanonRootBefore {
		return MigrationResult{}, fmt.Errorf("protocol migration changed canon root: before %s after %s", receipt.CanonRootBefore, root)
	}
	receipt.State = "committed"
	receipt.CanonRootAfter = root
	receipt.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rel, err := p.store.SaveCoreProtocolMigrationReceipt(receipt)
	if err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{
		Result: "MIGRATED", FromSchema: state.SchemaVersion, ToSchema: state.SchemaVersion,
		FromProtocol: receipt.FromProtocol, ToProtocol: receipt.ToProtocol,
		BackupPath: receipt.BackupPath, ReceiptPath: filepath.Join(p.root, rel),
		CanonRootBefore: receipt.CanonRootBefore, CanonRootAfter: root,
	}, nil
}

func (p *Project) writeProtocolUpgradeWorkspace(state *domain.CoreProjectState) error {
	if err := writeWorkspaceJSON(state.WorkspaceRoot, "project.json", workspaceProject{
		ProjectID: state.ProjectID, ProtocolVersion: state.ProtocolVersion, Mode: "chatgpt_app_drive",
	}); err != nil {
		return err
	}
	if err := protocol.WriteUTF8Atomic(state.WorkspaceRoot, "CHATGPT_PROTOCOL.md", []byte(protocol.RenderChatGPTProtocol(state.ProjectID)), 0o644); err != nil {
		return err
	}
	challenge := capabilityChallenge{
		ProjectID: state.ProjectID, ProtocolVersion: state.ProtocolVersion, Nonce: state.CapabilityNonce,
		AckPath: "setup/capability-ack.json", MarkdownPath: "setup/capability-write-test.md", MarkdownProbe: state.MarkdownProbe,
	}
	if err := writeWorkspaceJSON(state.WorkspaceRoot, "setup/capability-challenge.json", challenge); err != nil {
		return err
	}
	for _, rel := range []string{"setup/capability-ack.json", "setup/capability-write-test.md", "exchange/READY.json"} {
		if err := os.Remove(filepath.Join(state.WorkspaceRoot, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (p *Project) prepareMigrationBackup(state *domain.CoreProjectState, backupDestination string) (string, string, error) {
	backupDestination = strings.TrimSpace(backupDestination)
	if backupDestination == "" {
		return "", "", fmt.Errorf("migration backup destination is required")
	}
	backupPath, err := filepath.Abs(backupDestination)
	if err != nil {
		return "", "", err
	}
	root, err := p.currentCanonRoot()
	if err != nil {
		return "", "", err
	}
	if _, err := os.Lstat(backupPath); err == nil {
		if err := validateMigrationBackup(backupPath, state, root); err != nil {
			return "", "", err
		}
	} else if os.IsNotExist(err) {
		backup, err := p.createBackupUnlocked(backupPath)
		if err != nil {
			return "", "", err
		}
		if backup.CanonRoot != root {
			return "", "", fmt.Errorf("migration backup canon root changed")
		}
	} else {
		return "", "", err
	}
	return backupPath, root, nil
}

func (p *Project) currentCanonRoot() (string, error) {
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return "", err
	}
	if head == nil {
		return "", nil
	}
	recomputed, err := p.RecomputeCanonRoot()
	if err != nil {
		return "", err
	}
	if recomputed != head.Root {
		return "", fmt.Errorf("canon root mismatch before migration")
	}
	if problems := p.verifyReceiptChain(head); len(problems) > 0 {
		return "", fmt.Errorf("receipt chain before migration: %s", strings.Join(problems, "; "))
	}
	return head.Root, nil
}

func validateMigrationBackup(backupPath string, state *domain.CoreProjectState, canonRoot string) error {
	manifest, err := loadCoreBackupManifest(backupPath)
	if err != nil {
		return fmt.Errorf("validate migration backup: %w", err)
	}
	if manifest.CoreSchemaVersion != state.SchemaVersion || manifest.ProjectID != state.ProjectID ||
		manifest.ProtocolVersion != state.ProtocolVersion || manifest.CanonRoot != canonRoot {
		return fmt.Errorf("migration backup does not match current legacy project")
	}
	if _, err := validateCoreBackupPayload(backupPath, manifest); err != nil {
		return fmt.Errorf("validate migration backup: %w", err)
	}
	return nil
}
