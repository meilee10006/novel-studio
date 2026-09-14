package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type ChapterSettlement struct {
	Result         string      `json:"result"`
	NewCanonRoot   string      `json:"new_canon_root,omitempty"`
	IDMappings     []IDMapping `json:"id_mappings,omitempty"`
	Violations     []string    `json:"violations,omitempty"`
	ReceiptPath    string      `json:"receipt_path,omitempty"`
	BlockID        string      `json:"block_id,omitempty"`
	PlanningStatus string      `json:"planning_status,omitempty"`
}

func (p *Project) SettleActiveSnapshot() (ChapterSettlement, error) {
	release, err := p.acquireProjectMutationLock()
	if err != nil {
		return ChapterSettlement{}, err
	}
	defer release()
	return p.settleActiveSnapshotLocked()
}

func (p *Project) settleActiveSnapshotLocked() (ChapterSettlement, error) {
	project, state, err := p.activeProduction()
	if err != nil {
		return ChapterSettlement{}, err
	}
	task, attempt := state.ActiveTask, state.ActiveAttempt
	if state.ActiveBlock != nil && state.ActiveBlock.AttemptID == attempt.AttemptID {
		return ChapterSettlement{Result: "BLOCKED", BlockID: state.ActiveBlock.BlockID}, nil
	}
	if task.Kind != "chapter" && task.Kind != "revision" {
		return ChapterSettlement{}, fmt.Errorf("active task is %q, not chapter or revision", task.Kind)
	}
	journal, err := p.store.LoadCoreCommitJournal(attempt.AttemptID)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if journal != nil && journal.State != "committed" {
		return p.applyChapterCommit(project, journal)
	}
	record, err := p.store.LoadCoreSubmissionRecord(attempt.AttemptID)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if record == nil || record.State != "READY_TO_VALIDATE" {
		return ChapterSettlement{}, fmt.Errorf("active submission is not ready to validate")
	}
	files, manifest, err := p.readSnapshot(attempt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if err := validateSubmissionIdentity(project, task, attempt, manifest); err != nil {
		return ChapterSettlement{}, err
	}
	all := make(map[string][]byte, len(files)+1)
	for name, data := range files {
		all[name] = data
	}
	manifestRaw, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, "manifest.json")
	if err != nil {
		return ChapterSettlement{}, err
	}
	all["manifest.json"] = manifestRaw
	snapshotDigest, err := digestArtifactManifest(digestArtifacts(all))
	if err != nil {
		return ChapterSettlement{}, err
	}
	if snapshotDigest != record.SnapshotDigest {
		return ChapterSettlement{}, fmt.Errorf("local snapshot digest mismatch")
	}
	chapterFiles := make(map[string][]byte, len(chapterArtifactNames))
	for _, name := range chapterArtifactNames {
		chapterFiles[name] = files[name]
	}
	canonical, mappings, violations, chapter, err := validateAndCanonicalizeChapter(chapterFiles, state, task)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(violations) == 0 {
		referenceViolations, err := p.validateCanonicalEntityReferences(canonical)
		if err != nil {
			return ChapterSettlement{}, err
		}
		violations = append(violations, referenceViolations...)
	}
	if len(violations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, violations)
	}
	canonRaw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return ChapterSettlement{}, err
	}
	var canonState domain.CoreCanonState
	if err := protocol.DecodeJSON(canonRaw, &canonState); err != nil {
		return ChapterSettlement{}, err
	}
	validationCanon := canonState
	if task.Kind == "revision" {
		baseCanon, _, err := p.loadCanonSnapshotAtRoot(task.BaseCanonRoot)
		if err != nil {
			return ChapterSettlement{}, err
		}
		validationCanon = baseCanon
	}
	nextLongform, longformViolations, err := validateLongformState(validationCanon.Longform, canonical, chapter)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(longformViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, longformViolations)
	}
	planningStatus, nextPlanning, planningCanonical, planningRepair, planningViolations, err := validateChapterPlanning(files, validationCanon.Planning, chapter, requiresArtifact(attempt, "planning_patch.json"))
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(planningViolations) > 0 && requiresArtifact(attempt, "planning_patch.json") {
		return p.rejectChapter(project, state, task, attempt, record, files, planningViolations, planningStatus)
	}
	if planningStatus == "accepted" {
		canonical["planning_patch.json"] = planningCanonical
	}
	block, blockViolations, err := detectAuthorDecision(files, task, attempt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	if len(blockViolations) > 0 {
		return p.rejectChapter(project, state, task, attempt, record, files, blockViolations)
	}
	if block != nil {
		return p.blockChapter(project, state, task, attempt, record, files, block)
	}
	return p.acceptChapter(project, state, task, attempt, record, files, canonical, mappings, chapter, nextLongform, nextPlanning, planningStatus, planningRepair)
}

func (p *Project) readSnapshot(attempt *domain.CoreAttempt) (map[string][]byte, protocol.SubmissionManifest, error) {
	manifestRaw, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, "manifest.json")
	if err != nil {
		return nil, protocol.SubmissionManifest{}, err
	}
	var manifest protocol.SubmissionManifest
	if err := protocol.DecodeJSON(manifestRaw, &manifest); err != nil {
		return nil, protocol.SubmissionManifest{}, err
	}
	files := make(map[string][]byte, len(manifest.Files))
	for _, name := range manifest.Files {
		data, err := p.store.ReadCoreSnapshotFile(attempt.AttemptID, name)
		if err != nil {
			return nil, protocol.SubmissionManifest{}, err
		}
		files[name] = data
	}
	return files, manifest, nil
}

func validateAndCanonicalizeChapter(files map[string][]byte, state *domain.CoreProductionState, task *domain.CoreTask) (map[string][]byte, []IDMapping, []string, int, error) {
	if err := exactArtifactSet(files, chapterArtifactNames); err != nil {
		return nil, nil, nil, 0, err
	}
	chapter, err := chapterNumberFromTarget(task.Target)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	body := strings.TrimSpace(string(files["chapter.md"]))
	var violations []string
	if body == "" {
		violations = append(violations, "chapter body is empty")
	}
	values := map[string]any{}
	for _, name := range []string{"chapter_contract.json", "events.json", "state_delta.json", "self_review.json"} {
		var value any
		if err := protocol.DecodeJSON(files[name], &value); err != nil {
			return nil, nil, nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		values[name] = value
	}
	contract, ok := values["chapter_contract.json"].(map[string]any)
	if !ok {
		violations = append(violations, "chapter_contract.json must be an object")
	} else {
		declared, _ := contract["chapter"].(float64)
		if int(declared) != chapter || declared != float64(chapter) {
			violations = append(violations, "chapter_contract.chapter does not match task target")
		}
		pov, _ := contract["declared_pov"].(string)
		if strings.TrimSpace(pov) == "" {
			violations = append(violations, "chapter_contract.declared_pov is required")
		}
	}
	eventsRoot, ok := values["events.json"].(map[string]any)
	if !ok {
		violations = append(violations, "events.json must be an object")
		return nil, nil, violations, chapter, nil
	}
	events, ok := eventsRoot["events"].([]any)
	if !ok {
		violations = append(violations, "events.json.events must be an array")
		return nil, nil, violations, chapter, nil
	}
	seen := map[string]bool{}
	for _, item := range events {
		m, ok := item.(map[string]any)
		if !ok {
			violations = append(violations, "story event must be an object")
			continue
		}
		localID, _ := m["local_id"].(string)
		localID = strings.TrimSpace(localID)
		if localID == "" || seen[localID] {
			violations = append(violations, "story event local_id must be present and unique")
			continue
		}
		seen[localID] = true
		kind, _ := m["kind"].(string)
		anchor, _ := m["evidence_anchor"].(string)
		if kind != "offscreen" && (strings.TrimSpace(anchor) == "" || !strings.Contains(body, anchor)) {
			violations = append(violations, "visible story event evidence anchor is absent from chapter")
		}
	}
	if _, ok := values["state_delta.json"].(map[string]any); !ok {
		violations = append(violations, "state_delta.json must be an object")
	}
	if _, ok := values["self_review.json"].(map[string]any); !ok {
		violations = append(violations, "self_review.json must be an object")
	}
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}

	mappings := make([]IDMapping, 0, len(events))
	lookup := make(map[string]string, len(events))
	seq := state.NextEntitySeq
	for _, item := range events {
		m := item.(map[string]any)
		localID := strings.TrimSpace(m["local_id"].(string))
		canonID := fmt.Sprintf("story-event-%06d", seq)
		seq++
		lookup[localID] = canonID
		mappings = append(mappings, IDMapping{EntityType: "story_event", LocalID: localID, CanonID: canonID})
		m["canon_id"] = canonID
		delete(m, "local_id")
	}
	rewriteEventRefs(values["state_delta.json"], lookup, &violations)
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}

	canonical := map[string][]byte{"chapter.md": files["chapter.md"]}
	for _, name := range []string{"chapter_contract.json", "events.json", "state_delta.json", "self_review.json"} {
		data, err := json.Marshal(values[name])
		if err != nil {
			return nil, nil, nil, 0, err
		}
		canonical[name] = data
	}
	return canonical, mappings, nil, chapter, nil
}
func rewriteEventRefs(value any, lookup map[string]string, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		if local, ok := x["event_ref"].(string); ok {
			if canonID := lookup[strings.TrimSpace(local)]; canonID != "" {
				x["event_canon_id"] = canonID
				delete(x, "event_ref")
			} else {
				*violations = append(*violations, "state delta references unknown story event: "+local)
			}
		}
		for _, child := range x {
			rewriteEventRefs(child, lookup, violations)
		}
	case []any:
		for _, child := range x {
			rewriteEventRefs(child, lookup, violations)
		}
	}
}

func (p *Project) canonicalCharacterIDs() (map[string]bool, error) {
	charactersRaw, err := p.store.ReadCoreCanonArtifact("characters.json")
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := protocol.DecodeJSON(charactersRaw, &root); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	items, _ := root["characters"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if id := cleanString(item["canon_id"]); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}

func (p *Project) canonicalLocationIDs() (map[string]bool, error) {
	worldRaw, err := p.store.ReadCoreCanonArtifact("world.json")
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := protocol.DecodeJSON(worldRaw, &root); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, raw := range anySlice(root["entities"]) {
		item, _ := raw.(map[string]any)
		if cleanString(item["entity_type"]) != "location" {
			continue
		}
		if id := cleanString(item["canon_id"]); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}

func (p *Project) validateCanonicalEntityReferences(canonical map[string][]byte) ([]string, error) {
	ids, err := p.canonicalCharacterIDs()
	if err != nil {
		return nil, err
	}
	locations, err := p.canonicalLocationIDs()
	if err != nil {
		return nil, err
	}
	var violations []string
	var contract map[string]any
	if err := protocol.DecodeJSON(canonical["chapter_contract.json"], &contract); err != nil {
		return nil, err
	}
	if pov := cleanString(contract["declared_pov"]); !ids[pov] {
		violations = append(violations, "chapter_contract.declared_pov is not a canonical character")
	}

	var eventsRoot map[string]any
	if err := protocol.DecodeJSON(canonical["events.json"], &eventsRoot); err != nil {
		return nil, err
	}
	for _, raw := range anySlice(eventsRoot["events"]) {
		item, _ := raw.(map[string]any)
		for _, field := range []string{"actors", "observers"} {
			for _, id := range stringSlice(item[field]) {
				if !ids[id] {
					violations = append(violations, "story event "+field+" references unknown canonical character: "+id)
				}
			}
		}
	}

	var delta map[string]any
	if err := protocol.DecodeJSON(canonical["state_delta.json"], &delta); err != nil {
		return nil, err
	}
	for _, raw := range anySlice(delta["changes"]) {
		change, _ := raw.(map[string]any)
		kind := cleanString(change["kind"])
		characterID := cleanString(change["character_id"])
		if kind == "location" && characterID == "" {
			violations = append(violations, "location change requires character_id")
		} else if characterID != "" && !ids[characterID] {
			violations = append(violations, kind+" change references unknown canonical character: "+characterID)
		}
		if kind == "location" {
			locationID := cleanString(change["location_id"])
			if locationID == "" {
				violations = append(violations, "location change requires location_id")
			} else if !locations[locationID] {
				violations = append(violations, "location change references unknown canonical location: "+locationID)
			}
		}
		if kind == "relationship" {
			left, right, ok := relationshipCharacterIDs(cleanString(change["relationship_id"]))
			if !ok {
				violations = append(violations, "relationship change requires two canonical character ids")
			} else {
				if !ids[left] {
					violations = append(violations, "relationship change references unknown canonical character: "+left)
				}
				if !ids[right] {
					violations = append(violations, "relationship change references unknown canonical character: "+right)
				}
			}
		}
		if source, ok := change["source"].(map[string]any); ok {
			if from := cleanString(source["from_character_id"]); from != "" && !ids[from] {
				violations = append(violations, "knowledge source references unknown canonical character: "+from)
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func relationshipCharacterIDs(id string) (string, string, bool) {
	for _, sep := range []string{"|", ":"} {
		if strings.Count(id, sep) != 1 {
			continue
		}
		parts := strings.SplitN(id, sep, 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left != "" && right != "" {
			return left, right, true
		}
	}
	return "", "", false
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

func chapterNumberFromTarget(target string) (int, error) {
	const prefix = "chapter:"
	if !strings.HasPrefix(target, prefix) {
		return 0, fmt.Errorf("invalid chapter target %q", target)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(target, prefix))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid chapter target %q", target)
	}
	return n, nil
}
func (p *Project) rejectChapter(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, files map[string][]byte, violations []string, planningStatuses ...string) (ChapterSettlement, error) {
	sort.Strings(violations)
	planningStatus := ""
	if len(planningStatuses) > 0 {
		planningStatus = planningStatuses[0]
	}
	validationDigest, err := digestJSON(map[string]any{"result": "REWRITE", "violations": violations, "planning_status": planningStatus})
	if err != nil {
		return ChapterSettlement{}, err
	}
	receipt := &domain.CoreReceipt{
		SchemaVersion: coreSchemaVersion, ProjectID: project.ProjectID,
		TaskID: task.TaskID, AttemptID: attempt.AttemptID,
		PreviousRoot: state.CanonRoot, TaskDigest: attempt.TaskDigest,
		SubmissionDigest: record.SnapshotDigest, ArtifactDigests: digestArtifacts(files),
		ValidationDigest: validationDigest, Result: "REWRITE", PlanningStatus: planningStatus, NewRoot: state.CanonRoot,
		CommittedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	rel, err := p.store.SaveCoreReceipt(receipt)
	if err != nil {
		return ChapterSettlement{}, err
	}
	next, err := newAttempt(state, task, "rewrite", attempt.RequiredArtifacts, project.ProtocolVersion)
	if err != nil {
		return ChapterSettlement{}, err
	}
	state.ActiveAttempt = next
	if err := p.store.SaveCoreProductionState(state); err != nil {
		return ChapterSettlement{}, err
	}
	record.State = "SETTLED"
	record.Result = "REWRITE"
	record.NewCanonRoot = state.CanonRoot
	record.ReceiptPath = rel
	record.Problem = ""
	if err := p.store.SaveCoreSubmissionRecord(record); err != nil {
		return ChapterSettlement{}, err
	}
	if err := writeWorkspaceJSON(project.WorkspaceRoot, filepath.Join("exchange", "result", attempt.AttemptID+".json"), map[string]any{
		"schema_version": protocol.MachineSchemaVersion,
		"task_id":        task.TaskID, "attempt_id": attempt.AttemptID,
		"result": "REWRITE", "violations": violations,
	}); err != nil {
		return ChapterSettlement{}, err
	}
	if err := p.writeActiveAttempt(project, state); err != nil {
		return ChapterSettlement{}, err
	}
	return ChapterSettlement{
		Result: "REWRITE", Violations: violations, PlanningStatus: planningStatus,
		ReceiptPath: filepath.Join(p.root, rel),
	}, nil
}

func (p *Project) acceptChapter(project *domain.CoreProjectState, state *domain.CoreProductionState, task *domain.CoreTask, attempt *domain.CoreAttempt, record *domain.CoreSubmissionRecord, received, canonical map[string][]byte, mappings []IDMapping, chapter int, longform domain.CoreLongformState, planning domain.CorePlanningState, planningStatus string, planningRepair bool) (ChapterSettlement, error) {
	journal, err := p.prepareChapterCommit(project, state, task, attempt, record, received, canonical, mappings, chapter, longform, planning, planningStatus, planningRepair)
	if err != nil {
		return ChapterSettlement{}, err
	}
	return p.applyChapterCommit(project, journal)
}

func digestCanonArtifact(name string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		canonical, err := canonicalJSON(data)
		if err != nil {
			return "", err
		}
		return sha256Bytes(canonical), nil
	case ".md", ".txt":
		return sha256Bytes(data), nil
	default:
		return "", fmt.Errorf("unsupported canon artifact %q", name)
	}
}
