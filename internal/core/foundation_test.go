package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type readyView struct {
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
	BlockID         string `json:"block_id,omitempty"`
}

func TestPrepareFoundationArtifactsDoesNotConsumeEntitySequence(t *testing.T) {
	state := newCoreProductionState()
	before := *state

	prepared, violations, err := prepareFoundationArtifacts(validFoundationArtifacts(), state)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("violations=%v", violations)
	}
	if len(prepared.Canonical) != len(foundationArtifactNames) {
		t.Fatalf("canonical=%v", prepared.Canonical)
	}
	if !reflect.DeepEqual(before, *state) {
		t.Fatalf("prevalidation mutated state: before=%+v after=%+v", before, *state)
	}
}

func TestLegacyFoundationStillAcceptsBookPlanWithoutWholeBookSkeleton(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()

	settlement, err := project.SettleFoundation(FoundationSubmission{
		Manifest:  manifestForReady(ready, artifacts),
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if settlement.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", settlement)
	}
}

func TestFoundationAttemptCreatedOnceAfterCapabilityPassed(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	first := readReady(t, workspace)
	if first.TaskKind != "foundation" || first.AttemptReason != "initial" {
		t.Fatalf("unexpected READY: %+v", first)
	}
	if err := project.Reconcile(); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	second := readReady(t, workspace)
	if second.TaskID != first.TaskID || second.AttemptID != first.AttemptID {
		t.Fatalf("foundation attempt changed across reconcile: first=%+v second=%+v", first, second)
	}
	matches, err := filepath.Glob(filepath.Join(workspace, "exchange", "outbox", first.TaskID, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("foundation attempt dirs=%v want exactly one", matches)
	}
}

func TestRejectedFoundationDoesNotMintIDsOrCanon(t *testing.T) {
	project, local, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	artifacts["characters.json"] = []byte(`{"characters":[{"local_id":"same","name":"A"},{"local_id":"same","name":"B"}]}`)
	result, err := project.SettleFoundation(FoundationSubmission{
		Manifest:  manifestForReady(ready, artifacts),
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatalf("SettleFoundation: %v", err)
	}
	if result.Result != "REWRITE" || result.NewCanonRoot != "" || len(result.IDMappings) != 0 {
		t.Fatalf("unexpected rejected settlement: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(local, "meta", "core", "canon", "head.json")); !os.IsNotExist(err) {
		t.Fatalf("canon head exists after rejection: %v", err)
	}
	next := readReady(t, workspace)
	if next.TaskKind != "foundation" || next.TaskID != ready.TaskID || next.AttemptID == ready.AttemptID || next.AttemptReason != "rewrite" {
		t.Fatalf("expected foundation rewrite READY, got %+v", next)
	}
}

func TestAcceptedFoundationMintsTypedIDsRewritesRefsAndBuildsCanon(t *testing.T) {
	project, local, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	result, err := project.SettleFoundation(FoundationSubmission{
		Manifest:  manifestForReady(ready, artifacts),
		Artifacts: artifacts,
	})
	if err != nil {
		t.Fatalf("SettleFoundation: %v", err)
	}
	if result.Result != "ACCEPTED" || result.NewCanonRoot == "" {
		t.Fatalf("unexpected accepted settlement: %+v", result)
	}
	charID := mappingID(result.IDMappings, "character", "same")
	locationID := mappingID(result.IDMappings, "location", "same")
	if charID == "" || locationID == "" || charID == locationID {
		t.Fatalf("typed mappings invalid: %+v", result.IDMappings)
	}

	canonical, err := os.ReadFile(filepath.Join(local, "meta", "core", "canon", "artifacts", "foundation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var foundation map[string]any
	if err := json.Unmarshal(canonical, &foundation); err != nil {
		t.Fatal(err)
	}
	if got := nestedCanonID(foundation, "protagonist"); got != charID {
		t.Fatalf("protagonist canon_id=%q want %q", got, charID)
	}
	if got := nestedCanonID(foundation, "opening_location"); got != locationID {
		t.Fatalf("opening_location canon_id=%q want %q", got, locationID)
	}
	recomputed, err := project.RecomputeCanonRoot()
	if err != nil {
		t.Fatalf("RecomputeCanonRoot: %v", err)
	}
	if recomputed != result.NewCanonRoot {
		t.Fatalf("recomputed root=%q want %q", recomputed, result.NewCanonRoot)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonRoot != result.NewCanonRoot || status.ActiveTaskKind != "chapter" || status.ActiveTarget != "chapter:1" {
		t.Fatalf("unexpected post-foundation status: %+v", status)
	}
	chapterReady := readReady(t, workspace)
	if chapterReady.TaskKind != "chapter" || chapterReady.Target != "chapter:1" || chapterReady.BaseCanonRoot != result.NewCanonRoot {
		t.Fatalf("unexpected chapter READY: %+v", chapterReady)
	}

	receiptRaw, err := os.ReadFile(result.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		PreviousRoot string `json:"previous_root"`
		NewRoot      string `json:"new_root"`
	}
	if err := json.Unmarshal(receiptRaw, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.PreviousRoot != "" || receipt.NewRoot != result.NewCanonRoot {
		t.Fatalf("receipt roots=%+v settlement=%+v", receipt, result)
	}
	var receiptObject map[string]any
	readJSONFile(t, result.ReceiptPath, &receiptObject)
	for _, forbidden := range []string{"provider", "model", "usage"} {
		if _, ok := receiptObject[forbidden]; ok {
			t.Fatalf("receipt must not fabricate %s: %v", forbidden, receiptObject)
		}
	}

	canonPath := filepath.Join(local, "meta", "core", "canon", "artifacts", "book_plan.json")
	if err := os.WriteFile(canonPath, []byte(`{"direction":"tampered"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tamperedRoot, err := project.RecomputeCanonRoot()
	if err != nil {
		t.Fatalf("RecomputeCanonRoot after tamper: %v", err)
	}
	if tamperedRoot == result.NewCanonRoot {
		t.Fatalf("tampered canon unexpectedly kept root %q", tamperedRoot)
	}
}
func newCapabilityPassedProject(t *testing.T) (*Project, string, string) {
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
	ack := map[string]any{
		"project_id": challenge.ProjectID, "protocol_version": challenge.ProtocolVersion, "nonce": challenge.Nonce,
		"capabilities": map[string]bool{"read": true, "write_utf8_json": true, "write_utf8_md": true},
	}
	writeJSONFile(t, filepath.Join(workspace, "setup", "capability-ack.json"), ack)
	if err := os.WriteFile(filepath.Join(workspace, "setup", "capability-write-test.md"), []byte(challenge.MarkdownProbe), 0o644); err != nil {
		t.Fatal(err)
	}
	return project, local, workspace
}
func validFoundationArtifacts() map[string][]byte {
	return map[string][]byte{
		"foundation.json":       []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":"same"}}`),
		"characters.json":       []byte(`{"characters":[{"local_id":"same","name":"主角"}]}`),
		"world.json":            []byte(`{"entities":[{"entity_type":"location","local_id":"same","name":"起点"}]}`),
		"book_plan.json":        []byte(`{"direction":"完成主线"}`),
		"ending_contract.json":  []byte(`{"main_resolution":"主线得到明确收束"}`),
		"style_profile.json":    []byte(`{"language":"zh-CN"}`),
		"platform_profile.json": []byte(`{"platform":"fanqie"}`),
	}
}

func manifestForReady(ready readyView, artifacts map[string][]byte) protocol.SubmissionManifest {
	files := make([]string, 0, len(artifacts))
	for name := range artifacts {
		files = append(files, name)
	}
	return protocol.SubmissionManifest{
		SchemaVersion: protocol.MachineSchemaVersion,
		ProjectID:     ready.ProjectID, TaskID: ready.TaskID, AttemptID: ready.AttemptID,
		BaseCanonRoot: ready.BaseCanonRoot, ProtocolVersion: ready.ProtocolVersion,
		TaskDigest: ready.TaskDigest, CompletionNonce: ready.CompletionNonce, Files: files,
	}
}
func readReady(t *testing.T, workspace string) readyView {
	t.Helper()
	var ready readyView
	readJSONFile(t, filepath.Join(workspace, "exchange", "READY.json"), &ready)
	return ready
}

func mappingID(mappings []IDMapping, entityType, localID string) string {
	for _, m := range mappings {
		if m.EntityType == entityType && m.LocalID == localID {
			return m.CanonID
		}
	}
	return ""
}

func nestedCanonID(v map[string]any, key string) string {
	nested, _ := v[key].(map[string]any)
	id, _ := nested["canon_id"].(string)
	return id
}

func readJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}
func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestChapterTaskContextCarriesCanonicalFoundationReference(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	result, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
	if err != nil || result.Result != "ACCEPTED" {
		t.Fatalf("foundation=%+v err=%v", result, err)
	}
	characterID := mappingID(result.IDMappings, "character", "same")
	locationID := mappingID(result.IDMappings, "location", "same")
	if characterID == "" || locationID == "" {
		t.Fatalf("foundation mappings=%+v", result.IDMappings)
	}
	next := readReady(t, workspace)
	contextPath := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "context.json")
	raw, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"foundation_reference", characterID, locationID, "zh-CN", "fanqie", "主角", "起点"} {
		if !strings.Contains(text, want) {
			t.Errorf("chapter context missing canonical foundation reference %q: %s", want, text)
		}
	}
}

func TestChapterFoundationReferenceDoesNotCopyUnboundedFoundationMetadata(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	large := strings.Repeat("很长的非必要设定", 6000)
	artifacts["characters.json"] = []byte(`{"characters":[{"local_id":"same","name":"主角","biography":"` + large + `"}]}`)
	artifacts["world.json"] = []byte(`{"entities":[{"entity_type":"location","local_id":"same","name":"起点","lore":"` + large + `"}]}`)
	artifacts["style_profile.json"] = []byte(`{"language":"zh-CN","notes":"` + large + `"}`)
	artifacts["platform_profile.json"] = []byte(`{"platform":"fanqie","notes":"` + large + `"}`)
	result, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
	if err != nil || result.Result != "ACCEPTED" {
		t.Fatalf("large metadata foundation should still settle: result=%+v err=%v", result, err)
	}
	next := readReady(t, workspace)
	contextPath := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "context.json")
	raw, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= taskContextBudget {
		t.Fatalf("context bytes=%d budget=%d", len(raw), taskContextBudget)
	}
	if strings.Contains(string(raw), "很长的非必要设定") {
		t.Fatal("foundation_reference copied unbounded descriptive metadata")
	}
	for _, want := range []string{"character-000001", "location-000002", "主角", "起点", "zh-CN", "fanqie"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("compact foundation reference missing %q: %s", want, raw)
		}
	}
}
