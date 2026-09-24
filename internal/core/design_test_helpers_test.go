package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var designTestSubmissionSeq atomic.Uint64

func newRequiredDesignProjectForTest(t *testing.T) (*Project, string, string) {
	t.Helper()
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	project, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	writeCapabilityAckForTest(t, workspace)
	state, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	if state.DesignMode != domain.DesignModeRequired {
		t.Fatalf("fresh design_mode=%q", state.DesignMode)
	}
	if status, problem := capabilityStatus(state); status != "passed" {
		t.Fatalf("capability status=%q problem=%q", status, problem)
	}
	production, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if production != nil {
		t.Fatalf("fresh required design project unexpectedly has production state: %+v", production)
	}
	project.submissionQuietPeriod = 0
	return project, local, workspace
}

func importDesignArtifactForTest(t *testing.T, p *Project, artifactType string, inputs []string, payload any) string {
	t.Helper()
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, p, submissionID, "import", map[string]any{
		"artifact.json": artifactImportEnvelope(artifactType, inputs, payload),
	})
	lockDesignSubmissionForTest(t, p, submissionID)
	result, err := p.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "IMPORTED" || result.Refs["artifact.json"] == "" {
		t.Fatalf("artifact import result=%+v", result)
	}
	return result.Refs["artifact.json"]
}

func importDesignBundleForTest(t *testing.T, p *Project, selections map[string]string) string {
	t.Helper()
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, p, submissionID, "import", map[string]any{
		"bundle.json": map[string]any{
			"kind": "bundle",
			"bundle": map[string]any{
				"schema_version": 1,
				"selections":     selections,
			},
		},
	})
	lockDesignSubmissionForTest(t, p, submissionID)
	result, err := p.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "IMPORTED" || result.Refs["bundle.json"] == "" {
		t.Fatalf("bundle import result=%+v", result)
	}
	return result.Refs["bundle.json"]
}

func promoteForTest(t *testing.T, p *Project, submissionID, expectedRoot, checkpoint, bundleRef string, evidence map[string]string) domain.CoreDesignSubmissionResult {
	t.Helper()
	request := map[string]any{
		"schema_version":       1,
		"project_id":           "book-1",
		"submission_id":        submissionID,
		"protocol_version":     protocol.CurrentVersion,
		"expected_design_root": expectedRoot,
		"checkpoint":           checkpoint,
		"bundle_ref":           bundleRef,
		"evidence":             evidence,
	}
	writeDesignSubmissionForTest(t, p, submissionID, "promote", map[string]any{"promote.json": request})
	if _, err := p.ScanDesignSubmission(submissionID); err != nil {
		t.Fatal(err)
	}
	status, err := p.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" && status.State != "SETTLED" && status.State != "INVALID" {
		t.Fatalf("promote submission state=%+v", status)
	}
	result, err := p.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func validCreativeBriefPayload() map[string]any {
	return map[string]any{
		"purpose":              "验证长篇语义设计链",
		"target_genre":         "都市男频",
		"platform_constraints": []any{},
		"preferences":          []any{"故事主线优先"},
		"exclusions":           []any{"不把金手指当故事本身"},
		"author_principles":    []any{"先定故事再定手段"},
	}
}

func validStoryDecisionsPayload() map[string]any {
	return map[string]any{
		"decisions": []any{
			map[string]any{
				"id":        "story-decision-001",
				"status":    "locked",
				"statement": "故事主线优先于爽点机制",
				"blocking":  true,
			},
		},
	}
}

func validStoryConceptPayload() map[string]any {
	return map[string]any{
		"story":            "主角为解决迫近的个人危机被迫进入新的竞争环境，并逐步改变自己与核心关系。",
		"protagonist_goal": "解决眼前危机并获得持续生存空间",
		"central_conflict": "个人目标与竞争环境的规则持续冲突",
		"story_engine":     "每次推进目标都会制造新的选择、代价和关系变化",
		"change_path":      "主角从被动求生变成能主动选择并承担后果",
		"ending_direction": "核心危机和长期关系获得明确收束",
	}
}

func nextDesignSubmissionIDForTest() string {
	return fmt.Sprintf("design-test-%06d", designTestSubmissionSeq.Add(1))
}

func writeDesignSubmissionForTest(t *testing.T, p *Project, submissionID, operation string, files map[string]any) {
	t.Helper()
	state, err := p.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(state.WorkspaceRoot, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for name, value := range files {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	manifest := protocol.DesignManifest{
		SchemaVersion: protocol.MachineSchemaVersion, ProjectID: state.ProjectID, SubmissionID: submissionID,
		ProtocolVersion: state.ProtocolVersion, Operation: operation, Files: names,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func validStoryLockInputsForTest(
	t *testing.T,
	p *Project,
) (bundleRef string, evidence map[string]string) {
	t.Helper()
	briefRef := importDesignArtifactForTest(t, p, "creative_brief", nil, validCreativeBriefPayload())
	decisionsRef := importDesignArtifactForTest(t, p, "story_decisions", nil, validStoryDecisionsPayload())
	conceptRef := importDesignArtifactForTest(
		t, p, "story_concept", []string{briefRef, decisionsRef}, validStoryConceptPayload(),
	)
	bundleRef = importDesignBundleForTest(t, p, map[string]string{
		"creative_brief":  briefRef,
		"story_decisions": decisionsRef,
		"story_concept":   conceptRef,
	})
	reviewRef := importDesignArtifactForTest(t, p, "story_review", nil, map[string]any{
		"review_type":               "story_review",
		"subject_ref":               conceptRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{briefRef, decisionsRef},
	})
	approvalRef := importDesignArtifactForTest(t, p, "author_confirmation", nil, map[string]any{
		"subject_ref": conceptRef,
		"decision":    "APPROVED",
	})
	return bundleRef, map[string]string{
		"story_review":        reviewRef,
		"author_confirmation": approvalRef,
	}
}

type foundationDesignFixture struct {
	StoryRoot      string
	FoundationRoot string
	BriefRef       string
	DecisionsRef   string
	ConceptRef     string
	BundleRef      string
	ReviewRef      string
}

func makeStoryLockedProjectForTest(t *testing.T) (*Project, string, string, foundationDesignFixture) {
	t.Helper()
	project, local, workspace := newRequiredDesignProjectForTest(t)
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
		t, project, nextDesignSubmissionIDForTest(), "",
		domain.DesignCheckpointStoryLocked, bundleRef,
		map[string]string{
			"story_review":        reviewRef,
			"author_confirmation": approvalRef,
		},
	)
	if result.Result != "PROMOTED" || result.NewDesignRoot == "" {
		t.Fatalf("story lock result=%+v", result)
	}
	return project, local, workspace, foundationDesignFixture{
		StoryRoot:    result.NewDesignRoot,
		BriefRef:     briefRef,
		DecisionsRef: decisionsRef,
		ConceptRef:   conceptRef,
		BundleRef:    bundleRef,
		ReviewRef:    reviewRef,
	}
}

func importValidFoundationDesignForTest(
	t *testing.T,
	p *Project,
	briefRef, decisionsRef, conceptRef string,
) (bundleRef string, reviewRef string) {
	t.Helper()
	charactersRef := importDesignArtifactForTest(
		t, p, "characters", []string{conceptRef},
		decodeFoundationPayloadForTest(t, "characters.json"),
	)
	worldRef := importDesignArtifactForTest(
		t, p, "world", []string{conceptRef},
		decodeFoundationPayloadForTest(t, "world.json"),
	)
	endingRef := importDesignArtifactForTest(
		t, p, "ending_contract", []string{conceptRef},
		decodeFoundationPayloadForTest(t, "ending_contract.json"),
	)
	foundationRef := importDesignArtifactForTest(
		t, p, "foundation", []string{conceptRef, charactersRef, worldRef},
		decodeFoundationPayloadForTest(t, "foundation.json"),
	)
	bookPlanRef := importDesignArtifactForTest(
		t, p, "book_plan", []string{conceptRef, charactersRef, worldRef, endingRef},
		decodeFoundationPayloadForTest(t, "book_plan.json"),
	)
	styleRef := importDesignArtifactForTest(
		t, p, "style_profile", []string{briefRef},
		decodeFoundationPayloadForTest(t, "style_profile.json"),
	)
	platformRef := importDesignArtifactForTest(
		t, p, "platform_profile", []string{briefRef},
		decodeFoundationPayloadForTest(t, "platform_profile.json"),
	)
	bundleRef = importDesignBundleForTest(t, p, map[string]string{
		"creative_brief":   briefRef,
		"story_decisions":  decisionsRef,
		"story_concept":    conceptRef,
		"foundation":       foundationRef,
		"characters":       charactersRef,
		"world":            worldRef,
		"book_plan":        bookPlanRef,
		"ending_contract":  endingRef,
		"style_profile":    styleRef,
		"platform_profile": platformRef,
	})
	reviewRef = importFoundationReadinessReviewForTest(t, p, bundleRef)
	return bundleRef, reviewRef
}

func importFoundationReadinessReviewForTest(
	t *testing.T,
	p *Project,
	bundleRef string,
) string {
	t.Helper()
	return importDesignArtifactForTest(t, p, "foundation_readiness_review", nil, map[string]any{
		"review_type":               "foundation_readiness_review",
		"subject_ref":               bundleRef,
		"policy_version":            1,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{},
	})
}

func decodeFoundationPayloadForTest(t *testing.T, name string) any {
	t.Helper()
	raw, ok := validFoundationArtifacts()[name]
	if !ok {
		t.Fatalf("unknown foundation artifact %q", name)
	}
	var payload any
	if err := protocol.DecodeJSON(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if name == "book_plan.json" {
		root, ok := payload.(map[string]any)
		if !ok {
			t.Fatalf("book plan payload=%T", payload)
		}
		root["whole_book_skeleton"] = map[string]any{
			"stages": []any{
				map[string]any{
					"id":         "stage-1",
					"objective":  "建立核心冲突与主角目标",
					"transition": "主角主动进入下一阶段",
				},
				map[string]any{
					"id":         "stage-2",
					"objective":  "升级冲突并完成主线收束",
					"transition": "核心冲突进入最终解决",
				},
			},
			"ending_connection": "第二阶段直接连接既定结局收束",
		}
	}
	return payload
}

func readDesignBundleForTest(t *testing.T, p *Project, ref string) domain.CoreDesignBundle {
	t.Helper()
	bundle, err := p.loadDesignBundleRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func readDesignArtifactForTest(t *testing.T, p *Project, ref string) domain.CoreDesignArtifact {
	t.Helper()
	artifact, err := p.loadDesignArtifactRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func replaceRefForTest(items []string, oldRef, newRef string) []string {
	out := append([]string(nil), items...)
	for i, ref := range out {
		if ref == oldRef {
			out[i] = newRef
		}
	}
	return out
}

func mustDesignHeadForTest(t *testing.T, p *Project) *domain.CoreDesignHead {
	t.Helper()
	head, err := p.store.LoadCoreDesignHead()
	if err != nil {
		t.Fatal(err)
	}
	if head == nil {
		t.Fatal("design head is nil")
	}
	return head
}

func importDesignArtifactWithSourcesForTest(
	t *testing.T,
	p *Project,
	artifactType string,
	inputs, sources []string,
	payload any,
) string {
	t.Helper()
	if inputs == nil {
		inputs = []string{}
	}
	if sources == nil {
		sources = []string{}
	}
	submissionID := nextDesignSubmissionIDForTest()
	writeDesignSubmissionForTest(t, p, submissionID, "import", map[string]any{
		"artifact.json": map[string]any{
			"kind": "artifact",
			"artifact": map[string]any{
				"schema_version": 1,
				"artifact_type":  artifactType,
				"inputs":         inputs,
				"sources":        sources,
				"payload":        payload,
			},
		},
	})
	lockDesignSubmissionForTest(t, p, submissionID)
	result, err := p.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Result != "IMPORTED" || result.Refs["artifact.json"] == "" {
		t.Fatalf("artifact import result=%+v", result)
	}
	return result.Refs["artifact.json"]
}

func makeFoundationReadyProjectForTest(
	t *testing.T,
) (*Project, string, string, foundationDesignFixture) {
	t.Helper()
	project, local, workspace, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, reviewRef := importValidFoundationDesignForTest(
		t,
		project,
		fixture.BriefRef,
		fixture.DecisionsRef,
		fixture.ConceptRef,
	)
	result := promoteForTest(
		t,
		project,
		nextDesignSubmissionIDForTest(),
		fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady,
		bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "PROMOTED" || result.NewDesignRoot == "" {
		t.Fatalf("foundation ready result=%+v", result)
	}
	fixture.FoundationRoot = result.NewDesignRoot
	fixture.BundleRef = bundleRef
	fixture.ReviewRef = reviewRef
	return project, local, workspace, fixture
}

func foundationAcceptedReceiptsForTest(
	receipts []domain.CoreReceipt,
) []domain.CoreReceipt {
	var out []domain.CoreReceipt
	for _, receipt := range receipts {
		if receipt.Result == "ACCEPTED" && receipt.PreviousRoot == "" {
			out = append(out, receipt)
		}
	}
	return out
}
