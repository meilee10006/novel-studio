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
	var challenge struct {
		ProjectID       string `json:"project_id"`
		ProtocolVersion string `json:"protocol_version"`
		Nonce           string `json:"nonce"`
		MarkdownProbe   string `json:"markdown_probe"`
	}
	readJSONFile(t, filepath.Join(workspace, "setup", "capability-challenge.json"), &challenge)
	writeJSONFile(t, filepath.Join(workspace, "setup", "capability-ack.json"), map[string]any{
		"project_id": challenge.ProjectID, "protocol_version": challenge.ProtocolVersion, "nonce": challenge.Nonce,
		"capabilities": map[string]bool{"read": true, "write_utf8_json": true, "write_utf8_md": true},
	})
	if err := os.WriteFile(filepath.Join(workspace, "setup", "capability-write-test.md"), []byte(challenge.MarkdownProbe), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	state.DesignMode = domain.DesignModeRequired
	if err := project.store.SaveCoreProjectState(state); err != nil {
		t.Fatal(err)
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
	lockDesignSubmissionForTest(t, p, submissionID)
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
