package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func TestDesignImportLocksSnapshotAndReturnsCoreRefs(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	state, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	state.DesignMode = domain.DesignModeRequired
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
	}
	project.submissionQuietPeriod = 0

	writeDesignImport(t, workspace, "design-001", map[string]any{
		"concept.json": map[string]any{
			"kind": "artifact",
			"artifact": map[string]any{
				"schema_version": 1,
				"artifact_type":  "story_concept",
				"inputs":         []string{},
				"sources":        []string{},
				"payload": map[string]any{
					"story":            "测试故事",
					"protagonist_goal": "完成目标",
					"central_conflict": "持续冲突",
					"story_engine":     "持续选择",
					"change_path":      "长期变化",
					"ending_direction": "明确收束",
				},
			},
		},
	})

	if _, err := project.ScanDesignSubmission("design-001"); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission("design-001")
	if err != nil || status.State != "READY_TO_VALIDATE" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	result, err := project.ProcessDesignSubmission("design-001")
	if err != nil || result.Result != "IMPORTED" || result.Refs["concept.json"] == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func writeDesignImport(t *testing.T, workspace, submissionID string, envelopes map[string]any) {
	t.Helper()
	base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(envelopes))
	for name, envelope := range envelopes {
		raw, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, name)
	}
	manifest := protocol.DesignManifest{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       "book-1",
		SubmissionID:    submissionID,
		ProtocolVersion: protocol.CurrentVersion,
		Operation:       "import",
		Files:           files,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDesignImportSameSubmissionReplayIsIdempotent(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	writeDesignImport(t, workspace, "design-replay", map[string]any{
		"brief.json": artifactImportEnvelope("creative_brief", nil, map[string]any{"purpose": "test"}),
	})
	lockDesignSubmissionForTest(t, project, "design-replay")
	first, err := project.ProcessDesignSubmission("design-replay")
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.ProcessDesignSubmission("design-replay")
	if err != nil {
		t.Fatal(err)
	}
	if first.Result != "IMPORTED" || second.Result != first.Result || second.Refs["brief.json"] != first.Refs["brief.json"] {
		t.Fatalf("replay mismatch: first=%+v second=%+v", first, second)
	}
}

func TestDesignImportLockedMutationBecomesConflict(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	writeDesignImport(t, workspace, "design-mutated", map[string]any{
		"concept.json": artifactImportEnvelope("story_concept", nil, map[string]any{"story": "v1"}),
	})
	lockDesignSubmissionForTest(t, project, "design-mutated")

	base := filepath.Join(workspace, "exchange", "design", "inbox", "design-mutated")
	raw, err := json.Marshal(artifactImportEnvelope("story_concept", nil, map[string]any{"story": "v2"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "concept.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission("design-mutated")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !status.Conflict {
		t.Fatalf("mutated locked design submission=%+v", status)
	}
}

func TestDesignImportRejectsSymlink(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	writeDesignImport(t, workspace, "design-symlink", map[string]any{
		"concept.json": artifactImportEnvelope("story_concept", nil, map[string]any{"story": "v1"}),
	})
	base := filepath.Join(workspace, "exchange", "design", "inbox", "design-symlink")
	target := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(target, []byte(`{"outside":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(base, "concept.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(base, "concept.json")); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission("design-symlink")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" {
		t.Fatalf("symlink status=%+v", status)
	}
}

func TestDesignImportRejectsUnknownFile(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	writeDesignImport(t, workspace, "design-unknown", map[string]any{
		"concept.json": artifactImportEnvelope("story_concept", nil, map[string]any{"story": "v1"}),
	})
	base := filepath.Join(workspace, "exchange", "design", "inbox", "design-unknown")
	if err := os.WriteFile(filepath.Join(base, "extra.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission("design-unknown")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" {
		t.Fatalf("unknown-file status=%+v", status)
	}
}

func TestDesignImportRejectsOversizeTotal(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	submissionID := "design-oversize"
	base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{"a.json", "b.json", "c.json", "d.json", "e.json"}
	chunk := []byte(strings.Repeat("x", 7<<20))
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(base, name), chunk, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := protocol.DesignManifest{
		SchemaVersion: protocol.MachineSchemaVersion, ProjectID: "book-1", SubmissionID: submissionID,
		ProtocolVersion: protocol.CurrentVersion, Operation: "import", Files: files,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !strings.Contains(status.Problem, "max total size") {
		t.Fatalf("oversize status=%+v", status)
	}
}

func lockDesignSubmissionForTest(t *testing.T, project *Project, submissionID string) domain.CoreDesignSubmissionRecord {
	t.Helper()
	if _, err := project.ScanDesignSubmission(submissionID); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" {
		t.Fatalf("design submission not locked: %+v", status)
	}
	return status
}

func artifactImportEnvelope(artifactType string, inputs []string, payload any) map[string]any {
	if inputs == nil {
		inputs = []string{}
	}
	return map[string]any{
		"kind": "artifact",
		"artifact": map[string]any{
			"schema_version": 1,
			"artifact_type":  artifactType,
			"inputs":         inputs,
			"sources":        []string{},
			"payload":        payload,
		},
	}
}

func TestDesignImportAcceptsExistingHardInputsAndBundleRefs(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())
	conceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload())
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	if !strings.HasPrefix(bundleRef, "design_bundle@sha256:") {
		t.Fatalf("bundle ref=%q", bundleRef)
	}
}

func TestDesignImportRejectsSubmissionLocalHardInput(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefArtifact := map[string]any{
		"schema_version": 1,
		"artifact_type":  "creative_brief",
		"inputs":         []string{},
		"sources":        []string{},
		"payload":        validCreativeBriefPayload(),
	}
	briefRaw, err := json.Marshal(briefArtifact)
	if err != nil {
		t.Fatal(err)
	}
	_, _, briefRef, err := canonicalDesignArtifact(briefRaw)
	if err != nil {
		t.Fatal(err)
	}
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, project, submissionID, "import", map[string]any{
		"brief.json":   map[string]any{"kind": "artifact", "artifact": briefArtifact},
		"concept.json": artifactImportEnvelope("story_concept", []string{briefRef}, validStoryConceptPayload()),
	})
	lockDesignSubmissionForTest(t, project, submissionID)
	result, err := project.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "does not exist") {
		t.Fatalf("submission-local ref result=%+v", result)
	}
	_, digest, err := parseDesignArtifactRef(briefRef)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.store.ReadCoreDesignArtifact(digest); !os.IsNotExist(err) {
		t.Fatalf("invalid batch partially saved first object: err=%v", err)
	}
}

func TestDesignImportSupersedesMustExistAndMatchType(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, project, submissionID, "import", map[string]any{
		"next.json": map[string]any{
			"kind": "artifact",
			"artifact": map[string]any{
				"schema_version": 1,
				"artifact_type":  "story_concept",
				"inputs":         []string{},
				"sources":        []string{},
				"supersedes":     briefRef,
				"payload":        validStoryConceptPayload(),
			},
		},
	})
	lockDesignSubmissionForTest(t, project, submissionID)
	result, err := project.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "does not match") {
		t.Fatalf("supersedes mismatch result=%+v", result)
	}
}

func TestDesignImportBundleRejectsSlotTypeMismatch(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, project, submissionID, "import", map[string]any{
		"bundle.json": map[string]any{
			"kind": "bundle",
			"bundle": map[string]any{
				"schema_version": 1,
				"selections":     map[string]string{"story_concept": briefRef},
			},
		},
	})
	lockDesignSubmissionForTest(t, project, submissionID)
	result, err := project.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "requires artifact type") {
		t.Fatalf("slot mismatch result=%+v", result)
	}
}

func TestStoryLockedRequiresExactReviewAndAuthorConfirmation(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)

	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())
	conceptRef := importDesignArtifactForTest(
		t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload(),
	)
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})

	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type":               "story_review",
		"subject_ref":               conceptRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef,
		"decision":    "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-001", "",
		domain.DesignCheckpointStoryLocked,
		bundleRef,
		map[string]string{
			"story_review":        reviewRef,
			"author_confirmation": approvalRef,
		},
	)
	if result.Result != "PROMOTED" || result.NewDesignRoot == "" {
		t.Fatalf("result=%+v", result)
	}
}

func TestStoryLockedRejectsReviewForDifferentConcept(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())

	oldPayload := validStoryConceptPayload()
	oldPayload["story"] = "旧故事版本"
	oldConceptRef := importDesignArtifactForTest(
		t, project, "story_concept", []string{briefRef, decisionsRef}, oldPayload,
	)
	currentConceptRef := importDesignArtifactForTest(
		t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload(),
	)
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   currentConceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type":               "story_review",
		"subject_ref":               oldConceptRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": currentConceptRef,
		"decision":    "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-stale-review", "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{
			"story_review":        reviewRef,
			"author_confirmation": approvalRef,
		},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "story_review") {
		t.Fatalf("result=%+v", result)
	}
}

func TestStoryLockedRejectsAuthorConfirmationForDifferentConcept(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())

	oldPayload := validStoryConceptPayload()
	oldPayload["story"] = "旧故事版本"
	oldConceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, oldPayload)
	currentConceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload())
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   currentConceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type":               "story_review",
		"subject_ref":               currentConceptRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": oldConceptRef,
		"decision":    "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-stale-confirmation", "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{
			"story_review":        reviewRef,
			"author_confirmation": approvalRef,
		},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "author_confirmation") {
		t.Fatalf("result=%+v", result)
	}
}

func TestStoryLockedRejectsBlockingOpenDecision(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsPayload := validStoryDecisionsPayload()
	decisionsPayload["decisions"] = []any{
		map[string]any{
			"id":        "story-decision-001",
			"status":    "open",
			"statement": "仍待作者确认",
			"blocking":  true,
		},
	}
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, decisionsPayload)
	conceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload())
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type": "story_review", "subject_ref": conceptRef, "policy_version": 1, "verdict": "PASS",
		"findings": []any{}, "created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef, "decision": "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-blocking-open", "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{"story_review": reviewRef, "author_confirmation": approvalRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "blocking") {
		t.Fatalf("result=%+v", result)
	}
}

func TestStoryLockedRejectsInvalidCreativeBriefShape(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefPayload := validCreativeBriefPayload()
	briefPayload["preferences"] = []any{"故事主线优先", "故事主线优先"}
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, briefPayload)
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())
	conceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload())
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type": "story_review", "subject_ref": conceptRef, "policy_version": 1, "verdict": "PASS",
		"findings": []any{}, "created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef, "decision": "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-bad-brief", "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{"story_review": reviewRef, "author_confirmation": approvalRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "creative_brief") {
		t.Fatalf("result=%+v", result)
	}
}

func TestStoryLockedRejectsEmptyStoryConceptField(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())
	conceptPayload := validStoryConceptPayload()
	conceptPayload["central_conflict"] = ""
	conceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, conceptPayload)
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type": "story_review", "subject_ref": conceptRef, "policy_version": 1, "verdict": "PASS",
		"findings": []any{}, "created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef, "decision": "APPROVED",
	})

	result := promoteForTest(
		t, project, "design-promote-story-empty-concept", "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{"story_review": reviewRef, "author_confirmation": approvalRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "story_concept") {
		t.Fatalf("result=%+v", result)
	}
}

func TestDesignPromoteRejectsStaleExpectedRoot(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	bundleRef, evidence := validStoryLockInputsForTest(t, project)

	first := promoteForTest(
		t, project, "design-promote-cas-1", "",
		domain.DesignCheckpointStoryLocked, bundleRef, evidence,
	)
	if first.Result != "PROMOTED" {
		t.Fatalf("first=%+v", first)
	}

	stale := promoteForTest(
		t, project, "design-promote-cas-2", "",
		domain.DesignCheckpointStoryLocked, bundleRef, evidence,
	)
	if stale.Result != "STALE_DESIGN_HEAD" {
		t.Fatalf("stale=%+v", stale)
	}
	if stale.NewDesignRoot != first.NewDesignRoot {
		t.Fatalf("stale current root=%q want %q", stale.NewDesignRoot, first.NewDesignRoot)
	}
}

func TestDesignPromoteReplayUsesSettledSubmissionResult(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	bundleRef, evidence := validStoryLockInputsForTest(t, project)

	first := promoteForTest(
		t, project, "design-promote-replay", "",
		domain.DesignCheckpointStoryLocked, bundleRef, evidence,
	)
	second := promoteForTest(
		t, project, "design-promote-replay", "",
		domain.DesignCheckpointStoryLocked, bundleRef, evidence,
	)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replay changed result: first=%+v second=%+v", first, second)
	}
}

func TestStoryLockedCanAdvanceToNewStoryLockedWithCurrentExpectedRoot(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	firstBundle, firstEvidence := validStoryLockInputsForTest(t, project)
	first := promoteForTest(
		t, project, "design-promote-relock-1", "",
		domain.DesignCheckpointStoryLocked, firstBundle, firstEvidence,
	)
	if first.Result != "PROMOTED" {
		t.Fatalf("first=%+v", first)
	}

	briefRef := importDesignArtifactForTest(t, project, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, project, "story_decisions", nil, validStoryDecisionsPayload())
	nextConcept := validStoryConceptPayload()
	nextConcept["story"] = "重新锁定后的故事版本"
	conceptRef := importDesignArtifactForTest(t, project, "story_concept", []string{briefRef, decisionsRef}, nextConcept)
	bundleRef := importDesignBundleForTest(t, project, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, project, "story_review", nil, map[string]any{
		"review_type":               "story_review",
		"subject_ref":               conceptRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, project, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef,
		"decision":    "APPROVED",
	})
	second := promoteForTest(
		t, project, "design-promote-relock-2", first.NewDesignRoot,
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{
			"story_review":        reviewRef,
			"author_confirmation": approvalRef,
		},
	)
	if second.Result != "PROMOTED" || second.PreviousDesignRoot != first.NewDesignRoot {
		t.Fatalf("second=%+v first=%+v", second, first)
	}
	commit, err := project.store.LoadCoreDesignCommit(second.NewDesignRoot)
	if err != nil {
		t.Fatal(err)
	}
	if commit == nil || commit.ParentDesignRoot != first.NewDesignRoot {
		t.Fatalf("second commit=%+v", commit)
	}
}
