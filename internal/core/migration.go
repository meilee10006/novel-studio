package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

type MigrationResult struct {
	Result          string `json:"result"`
	FromSchema      int    `json:"from_schema"`
	ToSchema        int    `json:"to_schema"`
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

	receipt, err := p.store.LoadCoreMigrationReceipt(0, coreSchemaVersion)
	if err != nil {
		return MigrationResult{}, err
	}
	if state.SchemaVersion == coreSchemaVersion {
		if receipt != nil && receipt.State == "prepared" {
			return p.finishPreparedMigration(state, receipt)
		}
		root, err := p.currentCanonRoot()
		if err != nil {
			return MigrationResult{}, err
		}
		return MigrationResult{
			Result: "NOOP", FromSchema: coreSchemaVersion, ToSchema: coreSchemaVersion,
			CanonRootBefore: root, CanonRootAfter: root,
		}, nil
	}
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
		if backupDestination == "" {
			return MigrationResult{}, fmt.Errorf("migration backup destination is required")
		}
		backupPath, err := filepath.Abs(backupDestination)
		if err != nil {
			return MigrationResult{}, err
		}
		root, err := p.currentCanonRoot()
		if err != nil {
			return MigrationResult{}, err
		}
		if _, err := os.Lstat(backupPath); err == nil {
			if err := validateMigrationBackup(backupPath, state, root); err != nil {
				return MigrationResult{}, err
			}
		} else if os.IsNotExist(err) {
			backup, err := p.createBackupUnlocked(backupPath)
			if err != nil {
				return MigrationResult{}, err
			}
			if backup.CanonRoot != root {
				return MigrationResult{}, fmt.Errorf("migration backup canon root changed")
			}
		} else {
			return MigrationResult{}, err
		}
		receipt = &domain.CoreMigrationReceipt{
			SchemaVersion: coreSchemaVersion,
			State:         "prepared", ProjectID: state.ProjectID,
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
	return p.finishPreparedMigration(state, receipt)
}

func (p *Project) finishPreparedMigration(state *domain.CoreProjectState, receipt *domain.CoreMigrationReceipt) (MigrationResult, error) {
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
		BackupPath: receipt.BackupPath, ReceiptPath: filepath.Join(p.root, rel),
		CanonRootBefore: receipt.CanonRootBefore, CanonRootAfter: root,
	}, nil
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
