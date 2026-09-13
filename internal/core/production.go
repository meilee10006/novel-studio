package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var foundationArtifactNames = []string{
	"book_plan.json",
	"characters.json",
	"ending_contract.json",
	"foundation.json",
	"platform_profile.json",
	"style_profile.json",
	"world.json",
}

var chapterArtifactNames = []string{
	"chapter.md", "chapter_contract.json", "events.json",
	"self_review.json", "state_delta.json",
}

type IDMapping = domain.CoreIDMapping

type FoundationSubmission struct {
	Manifest  protocol.SubmissionManifest
	Artifacts map[string][]byte
}

type FoundationSettlement struct {
	Result       string      `json:"result"`
	NewCanonRoot string      `json:"new_canon_root,omitempty"`
	IDMappings   []IDMapping `json:"id_mappings,omitempty"`
	Violations   []string    `json:"violations,omitempty"`
	ReceiptPath  string      `json:"receipt_path,omitempty"`
}

type readyDocument struct {
	SchemaVersion   int    `json:"schema_version"`
	ProjectID       string `json:"project_id"`
	TaskID          string `json:"task_id"`
	TaskKind        string `json:"task_kind"`
	AttemptID       string `json:"attempt_id"`
	AttemptReason   string `json:"attempt_reason"`
	Target          string `json:"target"`
	BaseCanonRoot   string `json:"base_canon_root"`
	ProtocolVersion string `json:"protocol_version"`
	TaskDigest      string `json:"task_digest"`
	CompletionNonce string `json:"completion_nonce"`
	Status          string `json:"status"`
}

func (p *Project) Reconcile() error {
	if err := p.recoverPendingCommit(); err != nil {
		return err
	}
	projectState, err := p.store.LoadCoreProjectState()
	if err != nil {
		return err
	}
	if projectState == nil {
		return fmt.Errorf("project is not initialized")
	}
	capability, problem := capabilityStatus(projectState)
	if capability == "pending" {
		return nil
	}
	if capability != "passed" {
		return fmt.Errorf("capability check failed: %s", problem)
	}

	state, err := p.store.LoadCoreProductionState()
	if err != nil {
		return err
	}
	if state == nil {
		state = newCoreProductionState()
	}
	if state.ActiveTask == nil || state.ActiveAttempt == nil {
		if state.CanonRoot != "" {
			return fmt.Errorf("canon exists without an active task")
		}
		task := newTask(state, "foundation", "foundation", "")
		attempt, err := newAttempt(state, task, "initial", foundationArtifactNames, projectState.ProtocolVersion)
		if err != nil {
			return err
		}
		state.ActiveTask = task
		state.ActiveAttempt = attempt
		if err := p.store.SaveCoreProductionState(state); err != nil {
			return err
		}
	}
	return p.writeActiveAttempt(projectState, state)
}

func newCoreProductionState() *domain.CoreProductionState {
	return &domain.CoreProductionState{
		SchemaVersion:  coreSchemaVersion,
		NextTaskSeq:    1,
		NextAttemptSeq: 1,
		NextEntitySeq:  1,
	}
}

func newTask(state *domain.CoreProductionState, kind, target, baseRoot string) *domain.CoreTask {
	task := &domain.CoreTask{
		TaskID: fmt.Sprintf("task-%06d", state.NextTaskSeq), Kind: kind,
		Target: target, BaseCanonRoot: baseRoot,
	}
	state.NextTaskSeq++
	return task
}

func newAttempt(state *domain.CoreProductionState, task *domain.CoreTask, reason string, required []string, protocolVersion string) (*domain.CoreAttempt, error) {
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	files := append([]string(nil), required...)
	sort.Strings(files)
	attempt := &domain.CoreAttempt{
		AttemptID:         fmt.Sprintf("attempt-%06d", state.NextAttemptSeq),
		Reason:            reason,
		ProtocolVersion:   protocolVersion,
		CompletionNonce:   nonce,
		RequiredArtifacts: files,
	}
	state.NextAttemptSeq++
	digest, err := digestJSON(struct {
		Task              *domain.CoreTask `json:"task"`
		ProtocolVersion   string           `json:"protocol_version"`
		RequiredArtifacts []string         `json:"required_artifacts"`
	}{task, protocolVersion, files})
	if err != nil {
		return nil, err
	}
	attempt.TaskDigest = digest
	return attempt, nil
}
func (p *Project) writeActiveAttempt(projectState *domain.CoreProjectState, state *domain.CoreProductionState) error {
	if state.ActiveTask == nil || state.ActiveAttempt == nil {
		return fmt.Errorf("active task and attempt are required")
	}
	task := state.ActiveTask
	attempt := state.ActiveAttempt
	base := filepath.Join("exchange", "outbox", task.TaskID, attempt.AttemptID)
	taskDoc := map[string]any{
		"schema_version":     protocol.MachineSchemaVersion,
		"project_id":         projectState.ProjectID,
		"task_id":            task.TaskID,
		"task_kind":          task.Kind,
		"attempt_id":         attempt.AttemptID,
		"attempt_reason":     attempt.Reason,
		"target":             task.Target,
		"base_canon_root":    task.BaseCanonRoot,
		"protocol_version":   attempt.ProtocolVersion,
		"task_digest":        attempt.TaskDigest,
		"required_artifacts": attempt.RequiredArtifacts,
	}
	if err := writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join(base, "task.json"), taskDoc); err != nil {
		return err
	}
	for name, value := range map[string]any{
		"context.json":       map[string]any{},
		"constraints.json":   map[string]any{"required_artifacts": attempt.RequiredArtifacts},
		"canon_excerpt.json": map[string]any{"base_canon_root": task.BaseCanonRoot},
	} {
		if err := writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join(base, name), value); err != nil {
			return err
		}
	}
	if err := protocol.WriteUTF8Atomic(projectState.WorkspaceRoot, filepath.Join(base, "recent_prose.md"), nil, 0o644); err != nil {
		return err
	}
	return writeWorkspaceJSON(projectState.WorkspaceRoot, filepath.Join("exchange", "READY.json"), readyDocument{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       projectState.ProjectID,
		TaskID:          task.TaskID,
		TaskKind:        task.Kind,
		AttemptID:       attempt.AttemptID,
		AttemptReason:   attempt.Reason,
		Target:          task.Target,
		BaseCanonRoot:   task.BaseCanonRoot,
		ProtocolVersion: attempt.ProtocolVersion,
		TaskDigest:      attempt.TaskDigest,
		CompletionNonce: attempt.CompletionNonce,
		Status:          "ready",
	})
}
func (p *Project) SettleFoundation(sub FoundationSubmission) (FoundationSettlement, error) {
	projectState, err := p.store.LoadCoreProjectState()
	if err != nil || projectState == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return FoundationSettlement{}, err
	}
	state, err := p.store.LoadCoreProductionState()
	if err != nil || state == nil || state.ActiveTask == nil || state.ActiveAttempt == nil {
		if err == nil {
			err = fmt.Errorf("no active production attempt")
		}
		return FoundationSettlement{}, err
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	if task.Kind != "foundation" {
		return FoundationSettlement{}, fmt.Errorf("active task is %q, not foundation", task.Kind)
	}
	if err := validateSubmissionIdentity(projectState, task, attempt, sub.Manifest); err != nil {
		return FoundationSettlement{}, err
	}
	canonical, mappings, violations, err := validateAndCanonicalizeFoundation(sub.Artifacts, state)
	if err != nil {
		return FoundationSettlement{}, err
	}
	if len(violations) > 0 {
		return p.rejectFoundation(projectState, state, task, attempt, violations)
	}
	return p.acceptFoundation(projectState, state, task, attempt, sub, canonical, mappings)
}

func validateSubmissionIdentity(project *domain.CoreProjectState, task *domain.CoreTask, attempt *domain.CoreAttempt, manifest protocol.SubmissionManifest) error {
	if manifest.SchemaVersion != protocol.MachineSchemaVersion {
		return fmt.Errorf("unsupported submission schema version %d", manifest.SchemaVersion)
	}
	if manifest.ProjectID != project.ProjectID || manifest.TaskID != task.TaskID || manifest.AttemptID != attempt.AttemptID {
		return fmt.Errorf("submission identity does not match active attempt")
	}
	if manifest.BaseCanonRoot != task.BaseCanonRoot || manifest.ProtocolVersion != attempt.ProtocolVersion || manifest.TaskDigest != attempt.TaskDigest || manifest.CompletionNonce != attempt.CompletionNonce {
		return fmt.Errorf("submission binding does not match active attempt")
	}
	got := append([]string(nil), manifest.Files...)
	want := append([]string(nil), attempt.RequiredArtifacts...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("submission file list does not match active attempt")
	}
	return nil
}
func validateAndCanonicalizeFoundation(artifacts map[string][]byte, state *domain.CoreProductionState) (map[string][]byte, []IDMapping, []string, error) {
	if err := exactArtifactSet(artifacts, foundationArtifactNames); err != nil {
		return nil, nil, nil, err
	}
	values := make(map[string]any, len(artifacts))
	for _, name := range foundationArtifactNames {
		var value any
		if err := protocol.DecodeJSON(artifacts[name], &value); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		values[name] = value
	}
	var violations []string
	requireStringField(values["foundation.json"], "title", "foundation.title", &violations)
	requireStringField(values["book_plan.json"], "direction", "book_plan.direction", &violations)
	requireStringField(values["ending_contract.json"], "main_resolution", "ending_contract.main_resolution", &violations)
	requireStringField(values["style_profile.json"], "language", "style_profile.language", &violations)
	requireStringField(values["platform_profile.json"], "platform", "platform_profile.platform", &violations)

	defs := collectFoundationEntities(values, &violations)
	if len(violations) > 0 {
		return nil, nil, violations, nil
	}
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mappings := make([]IDMapping, 0, len(keys))
	lookup := make(map[string]string, len(keys))
	seq := state.NextEntitySeq
	for _, key := range keys {
		def := defs[key]
		id := fmt.Sprintf("%s-%06d", safeIDPrefix(def.entityType), seq)
		seq++
		lookup[key] = id
		mappings = append(mappings, IDMapping{EntityType: def.entityType, LocalID: def.localID, CanonID: id})
	}
	applyDefinitionIDs(values, lookup)
	for _, value := range values {
		rewriteFoundationRefs(value, lookup, &violations)
	}
	if len(violations) > 0 {
		return nil, nil, violations, nil
	}
	canonical := make(map[string][]byte, len(values))
	for name, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return nil, nil, nil, err
		}
		canonical[name] = data
	}
	return canonical, mappings, nil, nil
}

type foundationEntityDef struct{ entityType, localID string }

func collectFoundationEntities(values map[string]any, violations *[]string) map[string]foundationEntityDef {
	defs := map[string]foundationEntityDef{}
	if root, ok := values["characters.json"].(map[string]any); ok {
		items, _ := root["characters"].([]any)
		if len(items) == 0 {
			*violations = append(*violations, "characters must contain at least one character")
		}
		for _, item := range items {
			collectEntityDef(item, "character", defs, violations)
		}
	} else {
		*violations = append(*violations, "characters.json must be an object")
	}
	if root, ok := values["world.json"].(map[string]any); ok {
		items, _ := root["entities"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			entityType, _ := m["entity_type"].(string)
			collectEntityDef(item, entityType, defs, violations)
		}
	} else {
		*violations = append(*violations, "world.json must be an object")
	}
	return defs
}

func collectEntityDef(value any, entityType string, defs map[string]foundationEntityDef, violations *[]string) {
	m, ok := value.(map[string]any)
	if !ok {
		*violations = append(*violations, "entity definition must be an object")
		return
	}
	entityType = strings.TrimSpace(entityType)
	localID, _ := m["local_id"].(string)
	localID = strings.TrimSpace(localID)
	if entityType == "" || localID == "" {
		*violations = append(*violations, "entity definition requires entity_type and local_id")
		return
	}
	key := entityType + "\x00" + localID
	if _, exists := defs[key]; exists {
		*violations = append(*violations, "duplicate local id for entity type: "+entityType+"/"+localID)
		return
	}
	defs[key] = foundationEntityDef{entityType: entityType, localID: localID}
}
func requireStringField(value any, field, label string, violations *[]string) {
	m, ok := value.(map[string]any)
	if !ok {
		*violations = append(*violations, label+" container must be an object")
		return
	}
	s, _ := m[field].(string)
	if strings.TrimSpace(s) == "" {
		*violations = append(*violations, label+" is required")
	}
}

func applyDefinitionIDs(values map[string]any, lookup map[string]string) {
	if root, ok := values["characters.json"].(map[string]any); ok {
		if items, ok := root["characters"].([]any); ok {
			for _, item := range items {
				replaceDefinitionID(item, "character", lookup)
			}
		}
	}
	if root, ok := values["world.json"].(map[string]any); ok {
		if items, ok := root["entities"].([]any); ok {
			for _, item := range items {
				m, _ := item.(map[string]any)
				entityType, _ := m["entity_type"].(string)
				replaceDefinitionID(item, entityType, lookup)
			}
		}
	}
}

func replaceDefinitionID(value any, entityType string, lookup map[string]string) {
	m, ok := value.(map[string]any)
	if !ok {
		return
	}
	localID, _ := m["local_id"].(string)
	if id := lookup[strings.TrimSpace(entityType)+"\x00"+strings.TrimSpace(localID)]; id != "" {
		m["canon_id"] = id
		delete(m, "local_id")
	}
}

func rewriteFoundationRefs(value any, lookup map[string]string, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		entityType, _ := x["entity_type"].(string)
		localRef, hasRef := x["local_ref"].(string)
		if hasRef {
			key := strings.TrimSpace(entityType) + "\x00" + strings.TrimSpace(localRef)
			if id := lookup[key]; id != "" {
				x["canon_id"] = id
				delete(x, "local_ref")
			} else {
				*violations = append(*violations, "unknown local reference: "+strings.TrimSpace(entityType)+"/"+strings.TrimSpace(localRef))
			}
		}
		for _, child := range x {
			rewriteFoundationRefs(child, lookup, violations)
		}
	case []any:
		for _, child := range x {
			rewriteFoundationRefs(child, lookup, violations)
		}
	}
}

func exactArtifactSet(artifacts map[string][]byte, expected []string) error {
	got := make([]string, 0, len(artifacts))
	for name := range artifacts {
		got = append(got, name)
	}
	want := append([]string(nil), expected...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("artifact set does not match task contract")
	}
	return nil
}

func safeIDPrefix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entity"
	}
	return out
}
func (p *Project) rejectFoundation(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, violations []string) (FoundationSettlement, error) {
	sort.Strings(violations)
	validationDigest, _ := digestJSON(map[string]any{"result": "REWRITE", "violations": violations})
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: state.CanonRoot, TaskDigest: attempt.TaskDigest,
		ValidationDigest: validationDigest, Result: "REWRITE", NewRoot: state.CanonRoot,
		CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return FoundationSettlement{}, err
	}
	next, err := newAttempt(state, task, "rewrite", foundationArtifactNames, project.ProtocolVersion)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.ActiveAttempt = next
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return FoundationSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion, "task_id": task.TaskID,
		"attempt_id": attempt.AttemptID, "result": "REWRITE", "violations": violations,
	}); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return FoundationSettlement{}, err
	}
	return FoundationSettlement{Result: "REWRITE", Violations: violations, ReceiptPath: filepath.Join(p.root, rel)}, nil
}

func (p *Project) acceptFoundation(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, sub FoundationSubmission, artifacts map[string][]byte, mappings []IDMapping) (FoundationSettlement, error) {
	artifactDigests := digestArtifacts(artifacts)
	submissionDigest, err := digestArtifactManifest(artifactDigests)
	if err != nil {
		return FoundationSettlement{}, err
	}
	newRevision := state.Revision + 1
	canonState := &domain.CoreCanonState{SchemaVersion: coreSchemaVersion, Revision: newRevision, ProjectID: project.ProjectID, LastTaskID: task.TaskID, LastAttemptID: attempt.AttemptID}
	stateDigest, err := digestJSON(canonState)
	if err != nil {
		return FoundationSettlement{}, err
	}
	newRoot, err := computeCanonRoot(newRevision, state.CanonRoot, stateDigest, artifactDigests)
	if err != nil {
		return FoundationSettlement{}, err
	}
	head := &domain.CoreCanonHead{SchemaVersion: coreSchemaVersion, Revision: newRevision, ParentRoot: state.CanonRoot, Root: newRoot, StateDigest: stateDigest, ArtifactDigests: artifactDigests}
	if err := p.store.SaveCoreCanon(canonState, head, artifacts); err != nil {
		return FoundationSettlement{}, err
	}
	validationDigest, _ := digestJSON(map[string]any{"result": "ACCEPTED", "violations": []string{}})
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID, PreviousRoot: state.CanonRoot,
		TaskDigest: attempt.TaskDigest, SubmissionDigest: submissionDigest, ArtifactDigests: artifactDigests,
		ValidationDigest: validationDigest, Result: "ACCEPTED", NewRoot: newRoot,
		IDMappings: mappings, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.Revision, state.CanonRoot = newRevision, newRoot
	state.NextEntitySeq += len(mappings)
	chapterTask := newTask(state, "chapter", "chapter:1", newRoot)
	chapterAttempt, err := newAttempt(state, chapterTask, "initial", chapterArtifactNames, project.ProtocolVersion)
	if err != nil {
		return FoundationSettlement{}, err
	}
	state.ActiveTask, state.ActiveAttempt = chapterTask, chapterAttempt
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return FoundationSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion, "task_id": task.TaskID,
		"attempt_id": attempt.AttemptID, "result": "ACCEPTED", "new_canon_root": newRoot, "id_mappings": mappings,
	}); err != nil {
		return FoundationSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return FoundationSettlement{}, err
	}
	return FoundationSettlement{Result: "ACCEPTED", NewCanonRoot: newRoot, IDMappings: mappings, ReceiptPath: filepath.Join(p.root, rel)}, nil
}
func digestArtifacts(artifacts map[string][]byte) map[string]string {
	out := make(map[string]string, len(artifacts))
	for name, data := range artifacts {
		out[name] = sha256Bytes(data)
	}
	return out
}

func digestArtifactManifest(digests map[string]string) (string, error) {
	type item struct {
		Path   string `json:"path"`
		Digest string `json:"digest"`
	}
	items := make([]item, 0, len(digests))
	for path, digest := range digests {
		items = append(items, item{path, digest})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return digestJSON(items)
}

func computeCanonRoot(revision int, parentRoot, stateDigest string, artifactDigests map[string]string) (string, error) {
	return digestJSON(struct {
		SchemaVersion   int               `json:"schema_version"`
		Revision        int               `json:"revision"`
		ParentRoot      string            `json:"parent_root"`
		StateDigest     string            `json:"state_digest"`
		ArtifactDigests map[string]string `json:"artifact_digests"`
	}{coreSchemaVersion, revision, parentRoot, stateDigest, artifactDigests})
}

func digestJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	canonical, err := canonicalJSON(data)
	if err != nil {
		return "", err
	}
	return sha256Bytes(canonical), nil
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (p *Project) RecomputeCanonRoot() (string, error) {
	head, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return "", err
	}
	if head == nil {
		return "", fmt.Errorf("canon head does not exist")
	}
	stateRaw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return "", err
	}
	stateCanonical, err := canonicalJSON(stateRaw)
	if err != nil {
		return "", err
	}
	artifactDigests := make(map[string]string, len(head.ArtifactDigests))
	for name := range head.ArtifactDigests {
		raw, err := p.store.ReadCoreCanonArtifact(name)
		if err != nil {
			return "", err
		}
		digest, err := digestCanonArtifact(name, raw)
		if err != nil {
			return "", err
		}
		artifactDigests[name] = digest
	}
	return computeCanonRoot(head.Revision, head.ParentRoot, sha256Bytes(stateCanonical), artifactDigests)
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	if err := protocol.DecodeJSON(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
