package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

const maxDesignSubmissionBytes = maxSubmissionBytes

var designBundleSlots = map[string]string{
	"creative_brief":   "creative_brief",
	"story_decisions":  "story_decisions",
	"story_concept":    "story_concept",
	"foundation":       "foundation",
	"characters":       "characters",
	"world":            "world",
	"book_plan":        "book_plan",
	"ending_contract":  "ending_contract",
	"style_profile":    "style_profile",
	"platform_profile": "platform_profile",
}

type designImportEnvelope struct {
	Kind     string          `json:"kind"`
	Artifact json.RawMessage `json:"artifact,omitempty"`
	Bundle   json.RawMessage `json:"bundle,omitempty"`
}

type preparedDesignImport struct {
	name      string
	kind      string
	ref       string
	digest    string
	canonical []byte
}

func (p *Project) ScanDesignSubmission(submissionID string) (domain.CoreDesignSubmissionRecord, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return domain.CoreDesignSubmissionRecord{}, err
	}
	defer release()
	return p.scanDesignSubmissionLocked(submissionID)
}

func (p *Project) scanDesignSubmissionLocked(submissionID string) (domain.CoreDesignSubmissionRecord, error) {
	if err := validateDesignSubmissionID(submissionID); err != nil {
		return domain.CoreDesignSubmissionRecord{}, err
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil {
		return domain.CoreDesignSubmissionRecord{}, err
	}
	if project == nil {
		return domain.CoreDesignSubmissionRecord{}, fmt.Errorf("project is not initialized")
	}
	record, err := p.store.LoadCoreDesignSubmissionRecord(submissionID)
	if err != nil {
		return domain.CoreDesignSubmissionRecord{}, err
	}
	if record == nil {
		record = &domain.CoreDesignSubmissionRecord{
			SchemaVersion: coreSchemaVersion,
			SubmissionID:  submissionID,
			State:         "PENDING",
		}
	}

	baseRel := filepath.Join("exchange", "design", "inbox", submissionID)
	manifestRel := filepath.Join(baseRel, "manifest.json")
	var manifest protocol.DesignManifest
	if err := protocol.ReadJSON(project.WorkspaceRoot, manifestRel, maxInboxManifestBytes, &manifest); err != nil {
		if errors.Is(err, os.ErrNotExist) && record.SnapshotDigest == "" {
			return p.saveDesignSubmissionRecord(record)
		}
		if record.SnapshotDigest != "" {
			return p.conflictDesignSubmission(record, "Drive content changed after local snapshot was locked")
		}
		return p.invalidateDesignSubmission(record, err.Error())
	}
	if err := validateDesignManifest(project, submissionID, manifest); err != nil {
		if record.SnapshotDigest != "" {
			return p.conflictDesignSubmission(record, "Drive content changed after local snapshot was locked")
		}
		return p.invalidateDesignSubmission(record, err.Error())
	}
	record.Operation = manifest.Operation

	files, scanDigest, err := readInboxCandidate(project.WorkspaceRoot, baseRel, manifest.Files, maxDesignSubmissionBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && record.SnapshotDigest == "" {
			return p.saveDesignSubmissionRecord(record)
		}
		if record.SnapshotDigest != "" {
			return p.conflictDesignSubmission(record, "Drive content changed after local snapshot was locked")
		}
		return p.invalidateDesignSubmission(record, err.Error())
	}
	observation, decision := advanceStableObservation(stableObservation{
		ObservedDigest: record.ObservedDigest,
		ObservedAt:     record.ObservedAt,
		SnapshotDigest: record.SnapshotDigest,
	}, scanDigest, time.Now().UTC(), p.submissionQuietPeriod)
	record.ObservedDigest = observation.ObservedDigest
	record.ObservedAt = observation.ObservedAt
	record.SnapshotDigest = observation.SnapshotDigest
	switch decision {
	case "LOCKED_SAME":
		return *record, nil
	case "LOCKED_CONFLICT":
		return p.conflictDesignSubmission(record, "Drive content changed after local snapshot was locked")
	case "PENDING":
		record.State = "PENDING"
		record.Problem = ""
		return p.saveDesignSubmissionRecord(record)
	case "READY_TO_SNAPSHOT":
		if err := p.store.SaveCoreDesignSnapshot(submissionID, files); err != nil {
			return domain.CoreDesignSubmissionRecord{}, err
		}
		record.SnapshotDigest = scanDigest
		record.State = "READY_TO_VALIDATE"
		record.Problem = ""
		return p.saveDesignSubmissionRecord(record)
	default:
		return domain.CoreDesignSubmissionRecord{}, fmt.Errorf("unknown stable observation decision %q", decision)
	}
}

func (p *Project) ProcessDesignSubmission(submissionID string) (domain.CoreDesignSubmissionResult, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	defer release()
	return p.processDesignSubmissionLocked(submissionID)
}

func (p *Project) processDesignSubmissionLocked(submissionID string) (domain.CoreDesignSubmissionResult, error) {
	record, err := p.scanDesignSubmissionLocked(submissionID)
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if record.State == "SETTLED" && !record.Conflict {
		return designResultFromRecord(record), nil
	}
	if record.State == "INVALID" {
		return designResultFromRecord(record), nil
	}
	if record.State == "APPLYING" {
		return p.finishPreparedDesignPromote(&record)
	}
	if record.State != "READY_TO_VALIDATE" {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("design submission is not ready to validate")
	}

	project, err := p.store.LoadCoreProjectState()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if project == nil {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("project is not initialized")
	}
	if project.DesignMode != domain.DesignModeRequired {
		return p.rejectDesignSubmission(&record, "design mode is not required")
	}
	capability, problem := capabilityStatus(project)
	if capability != "passed" {
		if problem == "" {
			problem = "capability is not passed"
		}
		return p.rejectDesignSubmission(&record, problem)
	}
	canonHead, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if canonHead != nil {
		return p.rejectDesignSubmission(&record, "design authority is frozen after Canon exists")
	}
	switch record.Operation {
	case "import":
		return p.processDesignImport(project, &record)
	case "promote":
		return p.processDesignPromote(project, &record)
	default:
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("unsupported design operation")
	}
}

func (p *Project) processDesignPromote(project *domain.CoreProjectState, record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error) {
	manifestRaw, err := p.store.ReadCoreDesignSnapshotFile(record.SubmissionID, "manifest.json")
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	var manifest protocol.DesignManifest
	if err := protocol.DecodeJSON(manifestRaw, &manifest); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if err := validateDesignManifest(project, record.SubmissionID, manifest); err != nil {
		return p.rejectDesignSubmission(record, err.Error())
	}
	promoteRaw, err := p.store.ReadCoreDesignSnapshotFile(record.SubmissionID, "promote.json")
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	all := map[string][]byte{"manifest.json": manifestRaw, "promote.json": promoteRaw}
	snapshotDigest, err := digestArtifactManifest(digestArtifacts(all))
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if snapshotDigest != record.SnapshotDigest {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("local design snapshot digest mismatch")
	}

	var request protocol.DesignPromoteRequest
	if err := protocol.DecodeJSON(promoteRaw, &request); err != nil {
		return p.rejectDesignSubmission(record, err.Error())
	}
	if request.SchemaVersion != protocol.MachineSchemaVersion ||
		request.ProjectID != project.ProjectID ||
		request.SubmissionID != record.SubmissionID ||
		request.ProtocolVersion != project.ProtocolVersion {
		return p.rejectDesignSubmission(record, "design promote request identity does not match locked submission")
	}
	if request.BundleRef == "" {
		return p.rejectDesignSubmission(record, "design promote request bundle_ref is required")
	}
	return p.promoteDesignLocked(request, record)
}

func (p *Project) promoteDesignLocked(request protocol.DesignPromoteRequest, record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error) {
	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	currentRoot := ""
	if head != nil {
		currentRoot = head.DesignRoot
	}

	if head == nil {
		if request.ExpectedDesignRoot != "" {
			return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
				SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
				Result: "STALE_DESIGN_HEAD", NewDesignRoot: currentRoot,
				Problem: "expected design root does not match current design head",
			}, "SETTLED")
		}
	} else {
		if head.Checkpoint != domain.DesignCheckpointStoryLocked {
			return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
				SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
				Result: "INVALID", PreviousDesignRoot: currentRoot,
				Problem: "current design head is not story_locked",
			}, "INVALID")
		}
		if request.ExpectedDesignRoot != head.DesignRoot {
			return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
				SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
				Result: "STALE_DESIGN_HEAD", NewDesignRoot: currentRoot,
				Problem: "expected design root does not match current design head",
			}, "SETTLED")
		}
	}

	if request.Checkpoint != domain.DesignCheckpointStoryLocked {
		return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
			SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
			Result: "INVALID", PreviousDesignRoot: currentRoot,
			Problem: "unsupported design checkpoint for this task",
		}, "INVALID")
	}
	if err := p.validateStoryLocked(request.BundleRef, request.Evidence); err != nil {
		return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
			SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
			Result: "INVALID", PreviousDesignRoot: currentRoot, Problem: err.Error(),
		}, "INVALID")
	}

	commit := domain.CoreDesignCommit{
		SchemaVersion:           coreSchemaVersion,
		Checkpoint:              domain.DesignCheckpointStoryLocked,
		ParentDesignRoot:        currentRoot,
		BundleRef:               request.BundleRef,
		Evidence:                cloneStringMap(request.Evidence),
		CheckpointPolicyVersion: storyLockedCheckpointPolicyVersion,
	}
	root, err := computeDesignRoot(commit)
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	record.State = "APPLYING"
	record.PreviousDesignRoot = currentRoot
	record.PreparedCommit = &commit
	record.PreparedDesignRoot = root
	record.Problem = ""
	record.Result = ""
	record.NewDesignRoot = ""
	record.ReceiptPath = ""
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	return p.finishPreparedDesignPromote(record)
}

func (p *Project) finishPreparedDesignPromote(record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error) {
	if record == nil || record.PreparedCommit == nil || record.PreparedDesignRoot == "" {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("prepared design promote is incomplete")
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if project == nil {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("project is not initialized")
	}
	if err := p.store.SaveCoreDesignCommit(record.PreparedDesignRoot, record.PreparedCommit); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}

	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	currentRoot := ""
	if head != nil {
		currentRoot = head.DesignRoot
	}
	switch currentRoot {
	case record.PreviousDesignRoot:
		next := &domain.CoreDesignHead{
			SchemaVersion: coreSchemaVersion,
			DesignRoot:    record.PreparedDesignRoot,
			Checkpoint:    record.PreparedCommit.Checkpoint,
		}
		if err := p.store.CompareAndSwapCoreDesignHead(record.PreviousDesignRoot, next); err != nil {
			head, loadErr := p.store.LoadCoreDesignHead()
			if loadErr != nil {
				return domain.CoreDesignSubmissionResult{}, loadErr
			}
			if head == nil || head.DesignRoot != record.PreparedDesignRoot {
				return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
					SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
					Result: "INVALID", PreviousDesignRoot: record.PreviousDesignRoot,
					Problem: "design head changed while prepared promote was recovering",
				}, "INVALID")
			}
		}
	case record.PreparedDesignRoot:
		// HEAD already advanced; continue receipt/result settlement.
	default:
		return p.settleDesignPromoteOutcome(record, domain.CoreDesignSubmissionResult{
			SchemaVersion: coreSchemaVersion, SubmissionID: record.SubmissionID,
			Result: "INVALID", PreviousDesignRoot: record.PreviousDesignRoot,
			Problem: "design head changed while prepared promote was recovering",
		}, "INVALID")
	}

	receipt, err := p.store.LoadCoreDesignReceipt(record.SubmissionID)
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if receipt == nil {
		receipt = &domain.CoreDesignReceipt{
			SchemaVersion:      coreSchemaVersion,
			SubmissionID:       record.SubmissionID,
			Operation:          "promote",
			Result:             "PROMOTED",
			SnapshotDigest:     record.SnapshotDigest,
			PreviousDesignRoot: record.PreviousDesignRoot,
			NewDesignRoot:      record.PreparedDesignRoot,
			CommittedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	receiptRel, err := p.store.SaveCoreDesignReceipt(receipt)
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	result := domain.CoreDesignSubmissionResult{
		SchemaVersion:      coreSchemaVersion,
		SubmissionID:       record.SubmissionID,
		Result:             "PROMOTED",
		PreviousDesignRoot: record.PreviousDesignRoot,
		NewDesignRoot:      record.PreparedDesignRoot,
		ReceiptRef:         receiptRel,
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "design", "result", record.SubmissionID+".json"), result); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	record.State = "SETTLED"
	record.Result = result.Result
	record.NewDesignRoot = result.NewDesignRoot
	record.ReceiptPath = receiptRel
	record.Problem = ""
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	return result, nil
}

func (p *Project) settleDesignPromoteOutcome(record *domain.CoreDesignSubmissionRecord, result domain.CoreDesignSubmissionResult, state string) (domain.CoreDesignSubmissionResult, error) {
	project, err := p.store.LoadCoreProjectState()
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if project == nil {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("project is not initialized")
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "design", "result", record.SubmissionID+".json"), result); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	record.State = state
	record.Result = result.Result
	record.PreviousDesignRoot = result.PreviousDesignRoot
	record.NewDesignRoot = result.NewDesignRoot
	record.Problem = result.Problem
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	return result, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (p *Project) processDesignImport(project *domain.CoreProjectState, record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionResult, error) {
	manifestRaw, err := p.store.ReadCoreDesignSnapshotFile(record.SubmissionID, "manifest.json")
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	var manifest protocol.DesignManifest
	if err := protocol.DecodeJSON(manifestRaw, &manifest); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if err := validateDesignManifest(project, record.SubmissionID, manifest); err != nil {
		return p.rejectDesignSubmission(record, err.Error())
	}

	all := map[string][]byte{"manifest.json": manifestRaw}
	prepared := make([]preparedDesignImport, 0, len(manifest.Files))
	refs := make(map[string]string, len(manifest.Files))
	for _, name := range manifest.Files {
		raw, err := p.store.ReadCoreDesignSnapshotFile(record.SubmissionID, name)
		if err != nil {
			return domain.CoreDesignSubmissionResult{}, err
		}
		all[name] = raw
		item, err := p.prepareDesignImportFile(name, raw)
		if err != nil {
			return p.rejectDesignSubmission(record, err.Error())
		}
		prepared = append(prepared, item)
		refs[name] = item.ref
	}
	snapshotDigest, err := digestArtifactManifest(digestArtifacts(all))
	if err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	if snapshotDigest != record.SnapshotDigest {
		return domain.CoreDesignSubmissionResult{}, fmt.Errorf("local design snapshot digest mismatch")
	}
	for _, item := range prepared {
		switch item.kind {
		case "artifact":
			if err := p.store.SaveCoreDesignArtifact(item.digest, item.canonical); err != nil {
				return domain.CoreDesignSubmissionResult{}, err
			}
		case "bundle":
			if err := p.store.SaveCoreDesignBundle(item.digest, item.canonical); err != nil {
				return domain.CoreDesignSubmissionResult{}, err
			}
		default:
			return domain.CoreDesignSubmissionResult{}, fmt.Errorf("unsupported prepared design kind %q", item.kind)
		}
	}
	result := domain.CoreDesignSubmissionResult{
		SchemaVersion: coreSchemaVersion,
		SubmissionID:  record.SubmissionID,
		Result:        "IMPORTED",
		Refs:          refs,
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "design", "result", record.SubmissionID+".json"), result); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	record.State = "SETTLED"
	record.Result = result.Result
	record.Refs = refs
	record.Problem = ""
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	return result, nil
}

func (p *Project) prepareDesignImportFile(name string, raw []byte) (preparedDesignImport, error) {
	var envelope designImportEnvelope
	if err := protocol.DecodeJSON(raw, &envelope); err != nil {
		return preparedDesignImport{}, fmt.Errorf("%s: %w", name, err)
	}
	switch envelope.Kind {
	case "artifact":
		if len(envelope.Artifact) == 0 || string(envelope.Artifact) == "null" || len(envelope.Bundle) != 0 {
			return preparedDesignImport{}, fmt.Errorf("%s: invalid artifact import envelope", name)
		}
		artifact, canonical, ref, err := canonicalDesignArtifact(envelope.Artifact)
		if err != nil {
			return preparedDesignImport{}, fmt.Errorf("%s: %w", name, err)
		}
		if err := p.validateImportedArtifactReferences(artifact); err != nil {
			return preparedDesignImport{}, fmt.Errorf("%s: %w", name, err)
		}
		_, digest, err := parseDesignArtifactRef(ref)
		if err != nil {
			return preparedDesignImport{}, err
		}
		return preparedDesignImport{name: name, kind: "artifact", ref: ref, digest: digest, canonical: canonical}, nil
	case "bundle":
		if len(envelope.Bundle) == 0 || string(envelope.Bundle) == "null" || len(envelope.Artifact) != 0 {
			return preparedDesignImport{}, fmt.Errorf("%s: invalid bundle import envelope", name)
		}
		bundle, canonical, ref, err := canonicalDesignBundle(envelope.Bundle)
		if err != nil {
			return preparedDesignImport{}, fmt.Errorf("%s: %w", name, err)
		}
		if err := p.validateImportedBundleSelections(bundle); err != nil {
			return preparedDesignImport{}, fmt.Errorf("%s: %w", name, err)
		}
		digest, err := parseDesignBundleRef(ref)
		if err != nil {
			return preparedDesignImport{}, err
		}
		return preparedDesignImport{name: name, kind: "bundle", ref: ref, digest: digest, canonical: canonical}, nil
	default:
		return preparedDesignImport{}, fmt.Errorf("%s: unsupported design import kind %q", name, envelope.Kind)
	}
}

func (p *Project) validateImportedArtifactReferences(artifact domain.CoreDesignArtifact) error {
	for _, ref := range append(append([]string{}, artifact.Inputs...), artifact.Sources...) {
		if _, err := p.loadDesignArtifactRef(ref); err != nil {
			return err
		}
	}
	if artifact.Supersedes != "" {
		supersededType, _, err := parseDesignArtifactRef(artifact.Supersedes)
		if err != nil {
			return err
		}
		if supersededType != artifact.ArtifactType {
			return fmt.Errorf("supersedes artifact type %q does not match %q", supersededType, artifact.ArtifactType)
		}
		if _, err := p.loadDesignArtifactRef(artifact.Supersedes); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) validateImportedBundleSelections(bundle domain.CoreDesignBundle) error {
	for slot, ref := range bundle.Selections {
		wantType, ok := designBundleSlots[slot]
		if !ok {
			return fmt.Errorf("unknown design bundle slot %q", slot)
		}
		gotType, _, err := parseDesignArtifactRef(ref)
		if err != nil {
			return err
		}
		if gotType != wantType {
			return fmt.Errorf("design bundle slot %q requires artifact type %q", slot, wantType)
		}
		if _, err := p.loadDesignArtifactRef(ref); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) loadDesignArtifactRef(ref string) (domain.CoreDesignArtifact, error) {
	_, digest, err := parseDesignArtifactRef(ref)
	if err != nil {
		return domain.CoreDesignArtifact{}, err
	}
	raw, err := p.store.ReadCoreDesignArtifact(digest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domain.CoreDesignArtifact{}, fmt.Errorf("design artifact ref does not exist: %s", ref)
		}
		return domain.CoreDesignArtifact{}, err
	}
	artifact, _, actualRef, err := canonicalDesignArtifact(raw)
	if err != nil {
		return domain.CoreDesignArtifact{}, err
	}
	if actualRef != ref {
		return domain.CoreDesignArtifact{}, fmt.Errorf("design artifact ref does not match stored object: %s", ref)
	}
	return artifact, nil
}

func validateDesignManifest(project *domain.CoreProjectState, submissionID string, manifest protocol.DesignManifest) error {
	if manifest.SchemaVersion != protocol.MachineSchemaVersion {
		return fmt.Errorf("unsupported design manifest schema version %d", manifest.SchemaVersion)
	}
	if manifest.ProjectID != project.ProjectID || manifest.SubmissionID != submissionID || manifest.ProtocolVersion != project.ProtocolVersion {
		return fmt.Errorf("design submission identity does not match project")
	}
	if manifest.Operation != "import" && manifest.Operation != "promote" {
		return fmt.Errorf("unsupported design operation")
	}
	seen := make(map[string]bool, len(manifest.Files))
	for _, name := range manifest.Files {
		if name == "" || name == "." || name == ".." || name == "manifest.json" || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || seen[name] {
			return fmt.Errorf("invalid design file list")
		}
		seen[name] = true
	}
	if len(manifest.Files) == 0 {
		return fmt.Errorf("design submission has no files")
	}
	if manifest.Operation == "promote" && (len(manifest.Files) != 1 || manifest.Files[0] != "promote.json") {
		return fmt.Errorf("promote design submission must contain only promote.json")
	}
	return nil
}

func validateDesignSubmissionID(submissionID string) error {
	if submissionID == "" || submissionID == "." || submissionID == ".." || filepath.Base(submissionID) != submissionID || strings.ContainsAny(submissionID, "/\\") {
		return fmt.Errorf("invalid design submission id %q", submissionID)
	}
	return nil
}

func (p *Project) saveDesignSubmissionRecord(record *domain.CoreDesignSubmissionRecord) (domain.CoreDesignSubmissionRecord, error) {
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionRecord{}, err
	}
	return *record, nil
}

func (p *Project) invalidateDesignSubmission(record *domain.CoreDesignSubmissionRecord, problem string) (domain.CoreDesignSubmissionRecord, error) {
	record.State = "INVALID"
	record.Problem = problem
	return p.saveDesignSubmissionRecord(record)
}

func (p *Project) conflictDesignSubmission(record *domain.CoreDesignSubmissionRecord, problem string) (domain.CoreDesignSubmissionRecord, error) {
	record.State = "INVALID"
	record.Conflict = true
	record.Problem = problem
	return p.saveDesignSubmissionRecord(record)
}

func (p *Project) rejectDesignSubmission(record *domain.CoreDesignSubmissionRecord, problem string) (domain.CoreDesignSubmissionResult, error) {
	record.State = "INVALID"
	record.Problem = problem
	if err := p.store.SaveCoreDesignSubmissionRecord(record); err != nil {
		return domain.CoreDesignSubmissionResult{}, err
	}
	return designResultFromRecord(*record), nil
}

func designResultFromRecord(record domain.CoreDesignSubmissionRecord) domain.CoreDesignSubmissionResult {
	result := record.Result
	if result == "" && record.State == "INVALID" {
		result = "INVALID"
	}
	return domain.CoreDesignSubmissionResult{
		SchemaVersion:      coreSchemaVersion,
		SubmissionID:       record.SubmissionID,
		Result:             result,
		Refs:               record.Refs,
		PreviousDesignRoot: record.PreviousDesignRoot,
		NewDesignRoot:      record.NewDesignRoot,
		ReceiptRef:         record.ReceiptPath,
		Problem:            record.Problem,
	}
}
