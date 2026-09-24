package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

var designSubmissionIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

func (p *Project) Serve(ctx context.Context, scanInterval time.Duration) error {
	if ctx == nil {
		return fmt.Errorf("serve context is required")
	}
	if scanInterval <= 0 {
		return fmt.Errorf("serve scan interval must be positive")
	}
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return err
	}
	defer release()

	if err := p.servePassLocked(); err != nil {
		return err
	}
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := p.servePassLocked(); err != nil {
				return err
			}
		}
	}
}

func (p *Project) servePassLocked() error {
	if err := p.reconcileLocked(); err != nil {
		return err
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return err
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if project.DesignMode == domain.DesignModeRequired &&
		(production == nil || production.CanonRoot == "") {
		if err := p.serveDesignInboxLocked(project); err != nil {
			return err
		}
	}
	if err := p.reconcileLocked(); err != nil {
		return err
	}
	production, err = p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if production == nil || production.ActiveTask == nil || production.ActiveAttempt == nil {
		return nil
	}
	if err := p.serveControlsLocked(); err != nil {
		return err
	}
	if err := p.serveSubmissionLocked(); err != nil {
		return err
	}
	_ = p.writeNotionProjectionLocked()
	return nil
}

func (p *Project) serveDesignInboxLocked(project *domain.CoreProjectState) error {
	if project == nil || project.DesignMode != domain.DesignModeRequired {
		return nil
	}
	capability, _ := capabilityStatus(project)
	if capability != "passed" {
		return nil
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if production != nil && production.CanonRoot != "" {
		return nil
	}
	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		return err
	}
	if head != nil && head.Checkpoint == domain.DesignCheckpointFoundationReady {
		return nil
	}

	dir := filepath.Join(project.WorkspaceRoot, "exchange", "design", "inbox")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() ||
			!designSubmissionIDPattern.MatchString(entry.Name()) {
			continue
		}
		ids = append(ids, entry.Name())
	}
	sort.Strings(ids)
	for _, submissionID := range ids {
		status, err := p.scanDesignSubmissionLocked(submissionID)
		if err != nil {
			return err
		}
		if status.State == "READY_TO_VALIDATE" {
			if _, err := p.processDesignSubmissionLocked(submissionID); err != nil {
				return err
			}
			head, err := p.store.LoadCoreDesignHead()
			if err != nil {
				return err
			}
			if head != nil && head.Checkpoint == domain.DesignCheckpointFoundationReady {
				return nil
			}
		}
	}
	return nil
}

func (p *Project) serveControlsLocked() error {
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return err
	}
	dir := filepath.Join(project.WorkspaceRoot, "exchange", "control", "inbox")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		if controlMessageIDPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	for _, messageID := range ids {
		status, err := p.scanControlMessageLocked(messageID)
		if err != nil {
			return err
		}
		if status.State == "READY_TO_VALIDATE" {
			if _, err := p.processControlMessageLocked(messageID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Project) serveSubmissionLocked() error {
	_, state, err := p.activeProduction()
	if err != nil {
		return err
	}
	if state.ActiveBlock != nil {
		return nil
	}
	status, err := p.scanActiveSubmissionLocked()
	if err != nil {
		return err
	}
	switch status.State {
	case "PENDING", "SETTLED":
		return nil
	case "INVALID":
		if status.Result != "" {
			return nil
		}
		_, err := p.retryInvalidSubmissionLocked()
		return err
	case "READY_TO_VALIDATE":
		_, state, err = p.activeProduction()
		if err != nil {
			return err
		}
		switch state.ActiveTask.Kind {
		case "foundation":
			files, manifest, err := p.readSnapshot(state.ActiveAttempt)
			if err != nil {
				return err
			}
			_, err = p.settleFoundationLocked(FoundationSubmission{Manifest: manifest, Artifacts: files})
			return err
		case "chapter", "revision":
			_, err := p.settleActiveSnapshotLocked()
			return err
		default:
			return fmt.Errorf("serve does not support active task kind %q", state.ActiveTask.Kind)
		}
	default:
		return fmt.Errorf("serve encountered unknown submission state %q", status.State)
	}
}
