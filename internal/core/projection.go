package core

import (
	"fmt"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type NotionProjection struct {
	SchemaVersion   int    `json:"schema_version"`
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	CanonRoot       string `json:"canon_root,omitempty"`
	Revision        int    `json:"revision,omitempty"`
	LatestChapter   int    `json:"latest_chapter,omitempty"`
	ActiveTask      string `json:"active_task,omitempty"`
	ActiveTarget    string `json:"active_target,omitempty"`
	ActiveAttemptID string `json:"active_attempt_id,omitempty"`
	Blocked         bool   `json:"blocked,omitempty"`
	RevisionReplay  bool   `json:"revision_replay,omitempty"`
}

func (p *Project) WriteNotionProjection() error {
	release, err := p.acquireProjectReadLock()
	if err != nil {
		return err
	}
	defer release()
	return p.writeNotionProjectionLocked()
}

func (p *Project) writeNotionProjectionLocked() error {
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return err
	}
	projection := NotionProjection{
		SchemaVersion: protocol.MachineSchemaVersion,
		ProjectID:     project.ProjectID, ProtocolVersion: project.ProtocolVersion,
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if production != nil {
		projection.CanonRoot = production.CanonRoot
		projection.Revision = production.Revision
		projection.Blocked = production.ActiveBlock != nil
		projection.RevisionReplay = production.RevisionReplay != nil
		if production.ActiveTask != nil {
			projection.ActiveTask = production.ActiveTask.Kind
			projection.ActiveTarget = production.ActiveTask.Target
		}
		if production.ActiveAttempt != nil {
			projection.ActiveAttemptID = production.ActiveAttempt.AttemptID
		}
	}
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return err
	}
	if head != nil {
		raw, err := p.store.ReadCoreCanonStateBytes()
		if err != nil {
			return err
		}
		var canon domain.CoreCanonState
		if err := protocol.DecodeJSON(raw, &canon); err != nil {
			return err
		}
		projection.CanonRoot = head.Root
		projection.Revision = canon.Revision
		projection.LatestChapter = canon.LatestChapter
	}
	return writeWorkspaceJSON(project.WorkspaceRoot, "projection/notion.json", projection)
}
