package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"github.com/chenhongyang/novel-studio/internal/store"
)

type Project struct {
	root                  string
	store                 *store.CoreStore
	submissionQuietPeriod time.Duration
	commitFault           func(string) error
	controlFault          func(string) error
}

type Status struct {
	Root                 string   `json:"root"`
	Initialized          bool     `json:"initialized"`
	Warnings             []string `json:"warnings,omitempty"`
	ProjectID            string   `json:"project_id,omitempty"`
	ProtocolVersion      string   `json:"protocol_version,omitempty"`
	Capability           string   `json:"capability,omitempty"`
	CapabilityProblem    string   `json:"capability_problem,omitempty"`
	DesignMode           string   `json:"design_mode,omitempty"`
	DesignHead           string   `json:"design_head,omitempty"`
	DesignCheckpoint     string   `json:"design_checkpoint,omitempty"`
	FoundationDesignRoot string   `json:"foundation_design_root,omitempty"`
	CanonRoot            string   `json:"canon_root,omitempty"`
	ActiveTaskKind       string   `json:"active_task_kind,omitempty"`
	ActiveTarget         string   `json:"active_target,omitempty"`
	ActiveAttemptID      string   `json:"active_attempt_id,omitempty"`
	BlockID              string   `json:"block_id,omitempty"`
	ExportReady          bool     `json:"export_ready"`
	ExportProblems       []string `json:"export_problems,omitempty"`
}

type Verification struct {
	Root     string   `json:"root"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems,omitempty"`
}

type workspaceStatus struct {
	SchemaVersion        int      `json:"schema_version"`
	ProjectID            string   `json:"project_id"`
	ProtocolVersion      string   `json:"protocol_version"`
	Capability           string   `json:"capability"`
	CapabilityProblem    string   `json:"capability_problem,omitempty"`
	DesignMode           string   `json:"design_mode,omitempty"`
	DesignHead           string   `json:"design_head,omitempty"`
	DesignCheckpoint     string   `json:"design_checkpoint,omitempty"`
	FoundationDesignRoot string   `json:"foundation_design_root,omitempty"`
	CanonRoot            string   `json:"canon_root,omitempty"`
	ActiveTaskKind       string   `json:"active_task_kind,omitempty"`
	ActiveTarget         string   `json:"active_target,omitempty"`
	ActiveAttemptID      string   `json:"active_attempt_id,omitempty"`
	BlockID              string   `json:"block_id,omitempty"`
	RevisionReplay       bool     `json:"revision_replay,omitempty"`
	ExportReady          bool     `json:"export_ready"`
	ExportProblems       []string `json:"export_problems,omitempty"`
}

func (p *Project) writeWorkspaceStatus(project *domain.CoreProjectState, production *domain.CoreProductionState) error {
	if project == nil {
		return fmt.Errorf("project is not initialized")
	}
	capability, problem := capabilityStatus(project)
	out := workspaceStatus{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID, ProtocolVersion: project.ProtocolVersion,
		Capability: capability, CapabilityProblem: problem, DesignMode: project.DesignMode,
	}
	designHead, designCheckpoint, foundationDesignRoot, err := p.designStatusProjection(project)
	if err != nil {
		return err
	}
	out.DesignHead = designHead
	out.DesignCheckpoint = designCheckpoint
	out.FoundationDesignRoot = foundationDesignRoot
	if production != nil {
		out.CanonRoot = production.CanonRoot
		if production.ActiveTask != nil {
			out.ActiveTaskKind = production.ActiveTask.Kind
			out.ActiveTarget = production.ActiveTask.Target
		}
		if production.ActiveAttempt != nil {
			out.ActiveAttemptID = production.ActiveAttempt.AttemptID
		}
		if production.ActiveBlock != nil {
			out.BlockID = production.ActiveBlock.BlockID
		}
		out.RevisionReplay = production.RevisionReplay != nil
	}
	ready, problems, err := p.deterministicExportReadiness(production)
	if err != nil {
		return err
	}
	out.ExportReady, out.ExportProblems = ready, problems
	return writeWorkspaceJSON(project.WorkspaceRoot, "exchange/STATUS.json", out)
}

func (p *Project) designStatusProjection(
	project *domain.CoreProjectState,
) (designHead, designCheckpoint, foundationDesignRoot string, err error) {
	if project == nil || project.DesignMode != domain.DesignModeRequired {
		return "", "", "", nil
	}
	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		return "", "", "", err
	}
	if head != nil {
		designHead = head.DesignRoot
		designCheckpoint = head.Checkpoint
	}
	receipts, err := p.store.ListCoreReceipts()
	if err != nil {
		return "", "", "", err
	}
	for _, receipt := range receipts {
		if receipt.Result == "ACCEPTED" && receipt.FoundationDesignRoot != "" {
			foundationDesignRoot = receipt.FoundationDesignRoot
			break
		}
	}
	return designHead, designCheckpoint, foundationDesignRoot, nil
}

func (p *Project) deterministicExportReadiness(production *domain.CoreProductionState) (bool, []string, error) {
	if production == nil || production.CanonRoot == "" {
		return false, nil, nil
	}
	raw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return false, nil, err
	}
	var state domain.CoreCanonState
	if err := protocol.DecodeJSON(raw, &state); err != nil {
		return false, nil, err
	}
	problems := endingConstraintProblems(state.Longform, state.LatestChapter)
	if state.LatestChapter <= 0 {
		problems = append(problems, "no accepted chapters")
	}
	if production.RevisionReplay != nil {
		problems = append(problems, "revision replay is still active")
	}
	sort.Strings(problems)
	return len(problems) == 0, problems, nil
}

func OpenProject(root string) (*Project, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("project root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open project %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root %q is not a directory", abs)
	}
	st := store.NewCoreStore(abs)
	state, err := st.LoadCoreProjectState()
	if err != nil {
		return nil, fmt.Errorf("load core project metadata: %w", err)
	}
	if state != nil && (state.SchemaVersion < 0 || state.SchemaVersion > coreSchemaVersion) {
		return nil, fmt.Errorf("unsupported local core schema version %d", state.SchemaVersion)
	}
	if state != nil && !protocol.IsKnownVersion(state.ProtocolVersion) {
		return nil, fmt.Errorf("unsupported local protocol version %q", state.ProtocolVersion)
	}
	return &Project{root: abs, store: st, submissionQuietPeriod: 250 * time.Millisecond}, nil
}

func (p *Project) Status() (Status, error) {
	out := Status{Root: p.root}
	state, err := p.store.LoadCoreProjectState()
	if err != nil {
		return Status{}, fmt.Errorf("load core project metadata: %w", err)
	}
	if state != nil {
		out.Initialized = true
		out.ProjectID = state.ProjectID
		out.ProtocolVersion = state.ProtocolVersion
		out.DesignMode = state.DesignMode
		out.Capability, out.CapabilityProblem = capabilityStatus(state)
		designHead, designCheckpoint, foundationDesignRoot, err := p.designStatusProjection(state)
		if err != nil {
			return Status{}, fmt.Errorf("load design status: %w", err)
		}
		out.DesignHead = designHead
		out.DesignCheckpoint = designCheckpoint
		out.FoundationDesignRoot = foundationDesignRoot
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		return Status{}, fmt.Errorf("load core production state: %w", err)
	}
	if production != nil {
		out.CanonRoot = production.CanonRoot
		if production.ActiveTask != nil {
			out.ActiveTaskKind = production.ActiveTask.Kind
			out.ActiveTarget = production.ActiveTask.Target
		}
		if production.ActiveAttempt != nil {
			out.ActiveAttemptID = production.ActiveAttempt.AttemptID
		}
		if production.ActiveBlock != nil {
			out.BlockID = production.ActiveBlock.BlockID
		}
	}
	out.ExportReady, out.ExportProblems, err = p.deterministicExportReadiness(production)
	if err != nil {
		return Status{}, fmt.Errorf("compute export readiness: %w", err)
	}
	return out, nil
}

func (p *Project) Verify() (Verification, error) {
	release, err := p.acquireProjectReadLock()
	if err != nil {
		return Verification{}, err
	}
	defer release()
	status, err := p.Status()
	if err != nil {
		return Verification{}, err
	}
	problems := append([]string(nil), status.Warnings...)
	if status.Capability == "invalid" {
		problems = append(problems, "capability check: "+status.CapabilityProblem)
	}
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		problems = append(problems, "canon head: "+err.Error())
	} else if head != nil {
		recomputed, err := p.RecomputeCanonRoot()
		if err != nil {
			problems = append(problems, "canon root: "+err.Error())
		} else if recomputed != head.Root {
			problems = append(problems, fmt.Sprintf("canon root mismatch: recomputed %s, recorded %s", recomputed, head.Root))
		}
		problems = append(problems, p.verifyReceiptChain(head)...)
	}
	projectState, err := p.store.LoadCoreProjectState()
	if err != nil {
		problems = append(problems, "project state: "+err.Error())
	} else if projectState != nil {
		problems = append(problems, p.verifyDesignStore(projectState)...)
	}
	sort.Strings(problems)
	return Verification{Root: p.root, OK: len(problems) == 0, Problems: problems}, nil
}

func (p *Project) verifyDesignStore(project *domain.CoreProjectState) []string {
	if project == nil {
		return []string{"design store project state is missing"}
	}
	var problems []string

	artifactDigests, err := p.store.ListCoreDesignArtifactDigests()
	if err != nil {
		problems = append(problems, "design artifacts: "+err.Error())
		artifactDigests = nil
	}
	artifacts := make(map[string]domain.CoreDesignArtifact, len(artifactDigests))
	for _, fileDigest := range artifactDigests {
		raw, err := p.store.ReadCoreDesignArtifact(fileDigest)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design artifact %s: %v", fileDigest, err))
			continue
		}
		artifact, _, ref, err := canonicalDesignArtifact(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design artifact %s: %v", fileDigest, err))
			continue
		}
		artifactType, digest, err := parseDesignArtifactRef(ref)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design artifact %s ref: %v", fileDigest, err))
			continue
		}
		if digest != fileDigest {
			problems = append(problems, fmt.Sprintf(
				"design artifact digest mismatch: file %s, recomputed %s",
				fileDigest, digest,
			))
		}
		if artifactType != artifact.ArtifactType {
			problems = append(problems, fmt.Sprintf(
				"design artifact type mismatch: ref %s, object %s",
				artifactType, artifact.ArtifactType,
			))
		}
		artifacts[ref] = artifact
	}

	bundleDigests, err := p.store.ListCoreDesignBundleDigests()
	if err != nil {
		problems = append(problems, "design bundles: "+err.Error())
		bundleDigests = nil
	}
	bundles := make(map[string]domain.CoreDesignBundle, len(bundleDigests))
	for _, fileDigest := range bundleDigests {
		raw, err := p.store.ReadCoreDesignBundle(fileDigest)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design bundle %s: %v", fileDigest, err))
			continue
		}
		bundle, _, ref, err := canonicalDesignBundle(raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design bundle %s: %v", fileDigest, err))
			continue
		}
		digest, err := parseDesignBundleRef(ref)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design bundle %s ref: %v", fileDigest, err))
			continue
		}
		if digest != fileDigest {
			problems = append(problems, fmt.Sprintf(
				"design bundle digest mismatch: file %s, recomputed %s",
				fileDigest, digest,
			))
		}
		bundles[ref] = bundle
	}

	entries, err := p.store.ListCoreDesignCommits()
	if err != nil {
		problems = append(problems, "design commits: "+err.Error())
		entries = nil
	}
	commits := make(map[string]domain.CoreDesignCommit, len(entries))
	for _, entry := range entries {
		commits[entry.Root] = entry.Commit
	}

	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		problems = append(problems, "design head: "+err.Error())
		head = nil
	}

	coreReceipts, err := p.store.ListCoreReceipts()
	if err != nil {
		problems = append(problems, "foundation receipts: "+err.Error())
		coreReceipts = nil
	}

	required := project.DesignMode == domain.DesignModeRequired
	if !required {
		if head != nil || len(entries) != 0 {
			problems = append(problems, "legacy project unexpectedly contains authoritative design history")
		}
		for _, receipt := range coreReceipts {
			if receipt.FoundationDesignRoot != "" {
				problems = append(problems, fmt.Sprintf(
					"legacy foundation receipt %s unexpectedly contains foundation design root %s",
					receipt.AttemptID, receipt.FoundationDesignRoot,
				))
			}
		}
		sort.Strings(problems)
		return problems
	}

	for ref, artifact := range artifacts {
		for _, relation := range []struct {
			name string
			refs []string
		}{
			{name: "input", refs: artifact.Inputs},
			{name: "source", refs: artifact.Sources},
		} {
			for _, dependency := range relation.refs {
				if _, ok := artifacts[dependency]; !ok {
					problems = append(problems, fmt.Sprintf(
						"design artifact %s %s ref does not exist: %s",
						ref, relation.name, dependency,
					))
				}
			}
		}
		if artifact.Supersedes != "" {
			previous, ok := artifacts[artifact.Supersedes]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"design artifact %s supersedes ref does not exist: %s",
					ref, artifact.Supersedes,
				))
			} else if previous.ArtifactType != artifact.ArtifactType {
				problems = append(problems, fmt.Sprintf(
					"design artifact %s supersedes type mismatch: %s vs %s",
					ref, artifact.ArtifactType, previous.ArtifactType,
				))
			}
		}
	}

	for ref, bundle := range bundles {
		for slot, selectedRef := range bundle.Selections {
			artifact, ok := artifacts[selectedRef]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"design bundle %s selection %s does not exist: %s",
					ref, slot, selectedRef,
				))
				continue
			}
			if artifact.ArtifactType != slot {
				problems = append(problems, fmt.Sprintf(
					"design bundle %s slot %s selects artifact type %s",
					ref, slot, artifact.ArtifactType,
				))
			}
		}
		if err := validateBundleClosure(p, bundle); err != nil {
			problems = append(problems, fmt.Sprintf("design bundle %s closure: %v", ref, err))
		}
	}

	for _, entry := range entries {
		recomputed, err := computeDesignRoot(entry.Commit)
		if err != nil {
			problems = append(problems, fmt.Sprintf("design commit %s: %v", entry.Root, err))
		} else if recomputed != entry.Root {
			problems = append(problems, fmt.Sprintf(
				"design commit root mismatch: file %s, recomputed %s",
				entry.Root, recomputed,
			))
		}
		if entry.Commit.Checkpoint != domain.DesignCheckpointStoryLocked &&
			entry.Commit.Checkpoint != domain.DesignCheckpointFoundationReady {
			problems = append(problems, fmt.Sprintf(
				"design commit %s has invalid checkpoint %q",
				entry.Root, entry.Commit.Checkpoint,
			))
		}
		if _, ok := bundles[entry.Commit.BundleRef]; !ok {
			problems = append(problems, fmt.Sprintf(
				"design commit %s bundle ref does not exist: %s",
				entry.Root, entry.Commit.BundleRef,
			))
		}
		for name, evidenceRef := range entry.Commit.Evidence {
			if _, ok := artifacts[evidenceRef]; !ok {
				problems = append(problems, fmt.Sprintf(
					"design commit %s evidence %s does not exist: %s",
					entry.Root, name, evidenceRef,
				))
			}
		}
		if entry.Commit.ParentDesignRoot != "" {
			if _, ok := commits[entry.Commit.ParentDesignRoot]; !ok {
				problems = append(problems, fmt.Sprintf(
					"design commit %s parent does not exist: %s",
					entry.Root, entry.Commit.ParentDesignRoot,
				))
			}
		}
	}

	canonHead, err := p.store.LoadCoreCanonHead()
	if err != nil {
		problems = append(problems, "design/canon head: "+err.Error())
	}
	canonExists := canonHead != nil
	if head == nil {
		if canonExists {
			problems = append(problems, "design head is missing while canon exists")
		}
	} else {
		commit, ok := commits[head.DesignRoot]
		if !ok {
			problems = append(problems, fmt.Sprintf(
				"design head points to missing commit: %s",
				head.DesignRoot,
			))
		} else if head.Checkpoint != commit.Checkpoint {
			problems = append(problems, fmt.Sprintf(
				"design head checkpoint mismatch: head %s, commit %s",
				head.Checkpoint, commit.Checkpoint,
			))
		}
	}

	reachable := map[string]bool{}
	if head != nil {
		current := head.DesignRoot
		for current != "" {
			if reachable[current] {
				problems = append(problems, "design commit history contains a cycle at "+current)
				break
			}
			reachable[current] = true
			commit, ok := commits[current]
			if !ok {
				problems = append(problems, "design commit history is broken at "+current)
				break
			}
			current = commit.ParentDesignRoot
		}
	}
	for _, entry := range entries {
		if !reachable[entry.Root] {
			problems = append(problems, "design commit is outside authoritative history: "+entry.Root)
		}
	}

	designReceipts, err := p.store.ListCoreDesignReceipts()
	if err != nil {
		problems = append(problems, "design receipts: "+err.Error())
	} else {
		for _, receipt := range designReceipts {
			if receipt.Result != "PROMOTED" {
				continue
			}
			commit, ok := commits[receipt.NewDesignRoot]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"design receipt %s new root does not exist: %s",
					receipt.SubmissionID, receipt.NewDesignRoot,
				))
				continue
			}
			if receipt.PreviousDesignRoot != commit.ParentDesignRoot {
				problems = append(problems, fmt.Sprintf(
					"design receipt %s previous root mismatch: receipt %s, commit %s",
					receipt.SubmissionID, receipt.PreviousDesignRoot, commit.ParentDesignRoot,
				))
			}
		}
	}

	if canonExists {
		if head == nil || head.Checkpoint != domain.DesignCheckpointFoundationReady {
			problems = append(problems, "required canon exists without foundation_ready design head")
		}
		var accepted []domain.CoreReceipt
		for _, receipt := range coreReceipts {
			if receipt.Result == "ACCEPTED" && receipt.FoundationDesignRoot != "" {
				accepted = append(accepted, receipt)
			}
		}
		if len(accepted) != 1 {
			problems = append(problems, fmt.Sprintf(
				"required canon has %d accepted foundation receipts with foundation design root",
				len(accepted),
			))
		} else if head == nil || accepted[0].FoundationDesignRoot != head.DesignRoot {
			headRoot := ""
			if head != nil {
				headRoot = head.DesignRoot
			}
			problems = append(problems, fmt.Sprintf(
				"foundation design root mismatch: receipt %s, design head %s",
				accepted[0].FoundationDesignRoot, headRoot,
			))
		}
	}

	sort.Strings(problems)
	return problems
}

func (p *Project) verifyReceiptChain(head *domain.CoreCanonHead) []string {
	var problems []string
	production, err := p.store.LoadCoreProductionState()
	if err != nil {
		problems = append(problems, "receipt chain production state: "+err.Error())
	} else if production == nil {
		problems = append(problems, "receipt chain production state is missing")
	} else if production.CanonRoot != head.Root {
		problems = append(problems, fmt.Sprintf("receipt chain active root mismatch: production %s, head %s", production.CanonRoot, head.Root))
	}
	receipts, err := p.store.ListCoreReceipts()
	if err != nil {
		return append(problems, "receipt chain: "+err.Error())
	}
	byRoot := map[string][]domain.CoreReceipt{}
	for _, receipt := range receipts {
		if receipt.Result != "ACCEPTED" || receipt.NewRoot == "" {
			continue
		}
		byRoot[receipt.NewRoot] = append(byRoot[receipt.NewRoot], receipt)
	}
	seen := map[string]bool{}
	current := head.Root
	first := true
	for current != "" {
		if seen[current] {
			problems = append(problems, "receipt chain contains a cycle at "+current)
			break
		}
		seen[current] = true
		candidates := byRoot[current]
		if len(candidates) != 1 {
			problems = append(problems, fmt.Sprintf("receipt chain root %s has %d accepted receipts", current, len(candidates)))
			break
		}
		receipt := candidates[0]
		if first && receipt.PreviousRoot != head.ParentRoot {
			problems = append(problems, fmt.Sprintf("receipt chain head parent mismatch: receipt %s, head %s", receipt.PreviousRoot, head.ParentRoot))
		}
		current = receipt.PreviousRoot
		first = false
	}
	return problems
}

type capabilityAck struct {
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	Nonce           string `json:"nonce"`
	Capabilities    struct {
		Read          bool `json:"read"`
		WriteUTF8JSON bool `json:"write_utf8_json"`
		WriteUTF8MD   bool `json:"write_utf8_md"`
	} `json:"capabilities"`
}

func capabilityStatus(state *domain.CoreProjectState) (string, string) {
	var ack capabilityAck
	err := protocol.ReadJSON(state.WorkspaceRoot, "setup/capability-ack.json", 64<<10, &ack)
	if errors.Is(err, os.ErrNotExist) {
		return "pending", ""
	}
	if err != nil {
		return "invalid", err.Error()
	}
	if ack.ProjectID != state.ProjectID || ack.ProtocolVersion != state.ProtocolVersion || ack.Nonce != state.CapabilityNonce {
		return "invalid", "capability acknowledgement identity or nonce mismatch"
	}
	if !ack.Capabilities.Read || !ack.Capabilities.WriteUTF8JSON || !ack.Capabilities.WriteUTF8MD {
		return "invalid", "required plain-file capabilities were not acknowledged"
	}
	probe, err := protocol.ReadUTF8(state.WorkspaceRoot, "setup/capability-write-test.md", 64<<10)
	if errors.Is(err, os.ErrNotExist) {
		return "pending", ""
	}
	if err != nil {
		return "invalid", err.Error()
	}
	if string(probe) != state.MarkdownProbe {
		return "invalid", "capability markdown probe mismatch"
	}
	return "passed", ""
}
