package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chenhongyang/novel-studio/internal/core"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type protocolChainReady struct {
	ProjectID       string `json:"project_id"`
	TaskID          string `json:"task_id"`
	TaskKind        string `json:"task_kind"`
	AttemptID       string `json:"attempt_id"`
	Target          string `json:"target"`
	BaseCanonRoot   string `json:"base_canon_root"`
	ProtocolVersion string `json:"protocol_version"`
	TaskDigest      string `json:"task_digest"`
	CompletionNonce string `json:"completion_nonce"`
	Status          string `json:"status"`
	BlockID         string `json:"block_id"`
}

type protocolChainStatus struct {
	ProjectID       string `json:"project_id"`
	ProtocolVersion string `json:"protocol_version"`
	CanonRoot       string `json:"canon_root"`
	ActiveTarget    string `json:"active_target"`
	ActiveAttemptID string `json:"active_attempt_id"`
	BlockID         string `json:"block_id"`
	RevisionReplay  bool   `json:"revision_replay"`
}

func TestSemanticDesignProtocolChainCreatesFirstCanonWithoutFoundationTaskReady(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := core.InitProject(core.InitOptions{
		ProjectID:     "semantic-e2e",
		LocalRoot:     local,
		WorkspaceRoot: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeProtocolCapabilityAck(t, workspace)

	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("READY exists before Canon: %v", err)
	}

	briefRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-brief", "creative_brief",
		nil, protocolCreativeBriefPayload(),
	)
	decisionsRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-decisions", "story_decisions",
		nil, protocolStoryDecisionsPayload(),
	)
	conceptRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-concept", "story_concept",
		[]string{briefRef, decisionsRef},
		protocolStoryConceptPayload(),
	)
	storyBundle := driveImportBundleForProtocolChain(
		t, project, workspace, "e2e-story-bundle",
		map[string]string{
			"creative_brief":  briefRef,
			"story_decisions": decisionsRef,
			"story_concept":   conceptRef,
		},
	)
	storyReview := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-story-review", "story_review", nil,
		map[string]any{
			"review_type":               "story_review",
			"subject_ref":               conceptRef,
			"policy_version":            1,
			"verdict":                   "PASS",
			"findings":                  []any{},
			"created_from_context_refs": []any{briefRef, decisionsRef},
		},
	)
	authorConfirmation := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-author-confirm", "author_confirmation", nil,
		map[string]any{
			"subject_ref": conceptRef,
			"decision":    "APPROVED",
		},
	)
	storyLocked := drivePromoteForProtocolChain(
		t, project, workspace, "e2e-story-promote", "",
		domain.DesignCheckpointStoryLocked,
		storyBundle,
		map[string]string{
			"story_review":        storyReview,
			"author_confirmation": authorConfirmation,
		},
	)
	if storyLocked.Result != "PROMOTED" {
		t.Fatalf("story locked=%+v", storyLocked)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("READY exists at story_locked before Canon: %v", err)
	}

	charactersRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-characters", "characters",
		[]string{conceptRef}, protocolFoundationPayload("characters.json"),
	)
	worldRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-world", "world",
		[]string{conceptRef}, protocolFoundationPayload("world.json"),
	)
	endingRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-ending", "ending_contract",
		[]string{conceptRef}, protocolFoundationPayload("ending_contract.json"),
	)
	foundationRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-foundation", "foundation",
		[]string{conceptRef, charactersRef, worldRef},
		protocolFoundationPayload("foundation.json"),
	)
	bookPlanRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-book-plan", "book_plan",
		[]string{conceptRef, charactersRef, worldRef, endingRef},
		protocolFoundationPayload("book_plan.json"),
	)
	styleRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-style", "style_profile",
		[]string{briefRef}, protocolFoundationPayload("style_profile.json"),
	)
	platformRef := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-platform", "platform_profile",
		[]string{briefRef}, protocolFoundationPayload("platform_profile.json"),
	)

	foundationBundle := driveImportBundleForProtocolChain(
		t, project, workspace, "e2e-foundation-bundle",
		map[string]string{
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
		},
	)
	readinessReview := driveImportArtifactForProtocolChain(
		t, project, workspace, "e2e-readiness-review",
		"foundation_readiness_review", nil,
		map[string]any{
			"review_type":    "foundation_readiness_review",
			"subject_ref":    foundationBundle,
			"policy_version": 1,
			"verdict":        "PASS",
			"findings":       []any{},
			"created_from_context_refs": []any{
				conceptRef,
				foundationBundle,
			},
		},
	)
	readyResult := drivePromoteForProtocolChain(
		t, project, workspace, "e2e-foundation-promote",
		storyLocked.NewDesignRoot,
		domain.DesignCheckpointFoundationReady,
		foundationBundle,
		map[string]string{
			"foundation_readiness_review": readinessReview,
		},
	)
	if readyResult.Result != "PROMOTED" {
		t.Fatalf("foundation ready=%+v", readyResult)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("Foundation READY was published before internal settlement: %v", err)
	}

	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}

	ready := waitProtocolReady(t, workspace, func(r protocolChainReady) bool {
		return r.TaskKind == "chapter" && r.Target == "chapter:1"
	})
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonRoot == "" ||
		status.FoundationDesignRoot != readyResult.NewDesignRoot ||
		status.DesignHead != readyResult.NewDesignRoot ||
		status.DesignCheckpoint != domain.DesignCheckpointFoundationReady {
		t.Fatalf("status=%+v", status)
	}
	if ready.BaseCanonRoot != status.CanonRoot {
		t.Fatalf("READY=%+v status=%+v", ready, status)
	}
	verification, err := project.Verify()
	if err != nil || !verification.OK {
		t.Fatalf("verification=%+v err=%v", verification, err)
	}
}

func writeProtocolDesignSubmission(
	t *testing.T,
	p *core.Project,
	workspace string,
	submissionID string,
	operation string,
	files map[string]any,
) {
	t.Helper()
	status, err := p.Status()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(files))
	for name, value := range files {
		names = append(names, name)
		writeProtocolJSON(t, filepath.Join(dir, name), value)
	}
	sort.Strings(names)
	writeProtocolJSON(t, filepath.Join(dir, "manifest.json"), protocol.DesignManifest{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       status.ProjectID,
		SubmissionID:    submissionID,
		ProtocolVersion: protocol.CurrentVersion,
		Operation:       operation,
		Files:           names,
	})
}

func settleProtocolDesignSubmission(
	t *testing.T,
	p *core.Project,
	submissionID string,
) domain.CoreDesignSubmissionResult {
	t.Helper()
	record, err := p.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != "PENDING" && record.State != "READY_TO_VALIDATE" {
		t.Fatalf("first design scan=%+v", record)
	}
	if record.State != "READY_TO_VALIDATE" {
		time.Sleep(300 * time.Millisecond)
		record, err = p.ScanDesignSubmission(submissionID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if record.State != "READY_TO_VALIDATE" {
		t.Fatalf("design record=%+v", record)
	}
	result, err := p.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func driveImportArtifactForProtocolChain(
	t *testing.T,
	p *core.Project,
	workspace, submissionID, artifactType string,
	inputs []string,
	payload any,
) string {
	t.Helper()
	if inputs == nil {
		inputs = []string{}
	}
	writeProtocolDesignSubmission(
		t, p, workspace, submissionID, "import",
		map[string]any{
			"artifact.json": map[string]any{
				"kind": "artifact",
				"artifact": map[string]any{
					"schema_version": 1,
					"artifact_type":  artifactType,
					"inputs":         inputs,
					"sources":        []string{},
					"payload":        payload,
				},
			},
		},
	)
	result := settleProtocolDesignSubmission(t, p, submissionID)
	if result.Result != "IMPORTED" {
		t.Fatalf("design artifact import=%+v", result)
	}
	ref := result.Refs["artifact.json"]
	if ref == "" {
		t.Fatalf("design artifact import returned no ref: %+v", result)
	}
	return ref
}

func driveImportBundleForProtocolChain(
	t *testing.T,
	p *core.Project,
	workspace, submissionID string,
	selections map[string]string,
) string {
	t.Helper()
	writeProtocolDesignSubmission(
		t, p, workspace, submissionID, "import",
		map[string]any{
			"bundle.json": map[string]any{
				"kind": "bundle",
				"bundle": map[string]any{
					"schema_version": 1,
					"selections":     selections,
				},
			},
		},
	)
	result := settleProtocolDesignSubmission(t, p, submissionID)
	if result.Result != "IMPORTED" {
		t.Fatalf("design bundle import=%+v", result)
	}
	ref := result.Refs["bundle.json"]
	if ref == "" {
		t.Fatalf("design bundle import returned no ref: %+v", result)
	}
	return ref
}

func drivePromoteForProtocolChain(
	t *testing.T,
	p *core.Project,
	workspace, submissionID, expectedRoot, checkpoint, bundleRef string,
	evidence map[string]string,
) domain.CoreDesignSubmissionResult {
	t.Helper()
	status, err := p.Status()
	if err != nil {
		t.Fatal(err)
	}
	writeProtocolDesignSubmission(
		t, p, workspace, submissionID, "promote",
		map[string]any{
			"promote.json": protocol.DesignPromoteRequest{
				SchemaVersion:      protocol.MachineSchemaVersion,
				ProjectID:          status.ProjectID,
				SubmissionID:       submissionID,
				ProtocolVersion:    protocol.CurrentVersion,
				ExpectedDesignRoot: expectedRoot,
				Checkpoint:         checkpoint,
				BundleRef:          bundleRef,
				Evidence:           evidence,
			},
		},
	)
	return settleProtocolDesignSubmission(t, p, submissionID)
}

func protocolCreativeBriefPayload() map[string]any {
	return map[string]any{
		"purpose":              "验证长篇语义设计链",
		"target_genre":         "都市男频",
		"platform_constraints": []any{},
		"preferences":          []any{"故事主线优先"},
		"exclusions":           []any{"不把金手指当故事本身"},
		"author_principles":    []any{"先定故事再定手段"},
	}
}

func protocolStoryDecisionsPayload() map[string]any {
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

func protocolStoryConceptPayload() map[string]any {
	return map[string]any{
		"story":            "主角为解决迫近的个人危机被迫进入新的竞争环境，并逐步改变自己与核心关系。",
		"protagonist_goal": "解决眼前危机并获得持续生存空间",
		"central_conflict": "个人目标与竞争环境的规则持续冲突",
		"story_engine":     "每次推进目标都会制造新的选择、代价和关系变化",
		"change_path":      "主角从被动求生变成能主动选择并承担后果",
		"ending_direction": "核心危机和长期关系获得明确收束",
	}
}

func protocolFoundationPayload(name string) any {
	switch name {
	case "foundation.json":
		return map[string]any{
			"title": "语义设计 E2E",
			"protagonist": map[string]any{
				"entity_type": "character",
				"local_ref":   "same",
			},
			"opening_location": map[string]any{
				"entity_type": "location",
				"local_ref":   "same",
			},
		}
	case "characters.json":
		return map[string]any{
			"characters": []any{
				map[string]any{"local_id": "same", "name": "主角"},
			},
		}
	case "world.json":
		return map[string]any{
			"entities": []any{
				map[string]any{
					"entity_type": "location",
					"local_id":    "same",
					"name":        "起点",
				},
			},
		}
	case "book_plan.json":
		return map[string]any{
			"direction": "完成主线",
			"whole_book_skeleton": map[string]any{
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
			},
		}
	case "ending_contract.json":
		return map[string]any{"main_resolution": "主线得到明确收束"}
	case "style_profile.json":
		return map[string]any{"language": "zh-CN"}
	case "platform_profile.json":
		return map[string]any{"platform": "fanqie"}
	default:
		panic("unknown foundation payload: " + name)
	}
}

func TestPublicFileProtocolFullChain(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := core.InitProject(core.InitOptions{ProjectID: "protocol-chain-test", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	forceProtocolChainLegacyMode(t, local)
	writeProtocolCapabilityAck(t, workspace)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 20*time.Millisecond) }()
	defer func() {
		cancel()
		if done == nil {
			return
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Serve: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Serve did not stop")
		}
	}()

	ready := waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.TaskKind == "foundation" })
	writeProtocolFoundation(t, workspace, ready)
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:1" })

	firstAttempt := ready.AttemptID
	writeProtocolChapter(t, workspace, ready, "rewrite")
	waitProtocolResult(t, filepath.Join(workspace, "exchange", "result", firstAttempt+".json"), "REWRITE")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:1" && r.AttemptID != firstAttempt })

	blockedAttempt := ready.AttemptID
	writeProtocolChapter(t, workspace, ready, "block")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.AttemptID == blockedAttempt && r.Status == "blocked" })
	status := readProtocolStatus(t, workspace)
	if status.BlockID == "" {
		t.Fatal("BLOCKED did not publish block_id in STATUS")
	}
	writeProtocolControl(t, workspace, "ctrl-block", "block_resolution", "", "选择A")
	waitProtocolResult(t, filepath.Join(workspace, "exchange", "control", "result", "ctrl-block.json"), "ACCEPTED")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool {
		return r.Target == "chapter:1" && r.AttemptID != blockedAttempt && r.Status == "ready"
	})
	writeProtocolChapter(t, workspace, ready, "normal")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:2" })

	writeProtocolControl(t, workspace, "ctrl-future", "author_directive", "future_plan", "第三章以后减少追逐，强化人物互疑")
	waitProtocolResult(t, filepath.Join(workspace, "exchange", "control", "result", "ctrl-future.json"), "ACCEPTED")
	writeProtocolChapter(t, workspace, ready, "normal")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:3" })
	constraints := readProtocolText(t, filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID, "constraints.json"))
	if !strings.Contains(constraints, "第三章以后减少追逐，强化人物互疑") {
		t.Fatalf("future_plan did not reach future task: %s", constraints)
	}

	writeProtocolControl(t, workspace, "ctrl-revision", "author_directive", "historical_revision", "重写第一章但保持后续主线")
	waitProtocolResult(t, filepath.Join(workspace, "exchange", "control", "result", "ctrl-revision.json"), "ACCEPTED")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.TaskKind == "revision" && r.Target == "chapter:1" })
	if !readProtocolStatus(t, workspace).RevisionReplay {
		t.Fatal("historical revision did not expose revision_replay in STATUS")
	}
	writeProtocolChapter(t, workspace, ready, "normal")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.TaskKind == "revision" && r.Target == "chapter:2" })
	writeProtocolChapter(t, workspace, ready, "normal")
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.TaskKind == "chapter" && r.Target == "chapter:3" })
	if readProtocolStatus(t, workspace).RevisionReplay {
		t.Fatal("revision replay did not clear after catching up")
	}

	writeProtocolChapter(t, workspace, ready, "ending")
	_ = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:4" })

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
		done = nil
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop before verify")
	}

	verification, err := project.Verify()
	if err != nil || !verification.OK {
		t.Fatalf("Verify=%+v err=%v", verification, err)
	}
	exportPath := filepath.Join(t.TempDir(), "book.md")
	exported, err := project.ExportBook(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Chapters != 3 || exported.CanonRoot == "" {
		t.Fatalf("export=%+v", exported)
	}
	book := readProtocolText(t, exportPath)
	for _, marker := range []string{"第1章正文", "第2章正文", "第3章正文"} {
		if !strings.Contains(book, marker) {
			t.Fatalf("export missing %q: %s", marker, book)
		}
	}
}

func TestPublicFileProtocolResumesAfterCoreRestart(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := core.InitProject(core.InitOptions{ProjectID: "protocol-restart-test", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	forceProtocolChainLegacyMode(t, local)
	writeProtocolCapabilityAck(t, workspace)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 20*time.Millisecond) }()
	ready := waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.TaskKind == "foundation" })
	writeProtocolFoundation(t, workspace, ready)
	ready = waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:1" })

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first Serve did not stop")
	}

	project, err = core.OpenProject(local)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	done = make(chan error, 1)
	go func() { done <- project.Serve(ctx, 20*time.Millisecond) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("second Serve: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("second Serve did not stop")
		}
	}()

	resumed := waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:1" })
	if resumed.AttemptID != ready.AttemptID || resumed.TaskID != ready.TaskID {
		t.Fatalf("restart changed active attempt: before=%+v after=%+v", ready, resumed)
	}
	writeProtocolChapter(t, workspace, resumed, "normal")
	next := waitProtocolReady(t, workspace, func(r protocolChainReady) bool { return r.Target == "chapter:2" })
	if next.AttemptID == resumed.AttemptID || readProtocolStatus(t, workspace).CanonRoot == "" {
		t.Fatalf("restart resume did not advance authority: resumed=%+v next=%+v status=%+v", resumed, next, readProtocolStatus(t, workspace))
	}
}

func forceProtocolChainLegacyMode(t *testing.T, local string) {
	t.Helper()
	path := filepath.Join(local, "meta", "core", "project.json")
	var state map[string]any
	readProtocolJSON(t, path, &state)
	state["design_mode"] = "legacy"
	writeProtocolJSON(t, path, state)
}

func writeProtocolCapabilityAck(t *testing.T, workspace string) {
	t.Helper()
	var challenge struct {
		ProjectID       string `json:"project_id"`
		ProtocolVersion string `json:"protocol_version"`
		Nonce           string `json:"nonce"`
		MarkdownProbe   string `json:"markdown_probe"`
	}
	readProtocolJSON(t, filepath.Join(workspace, "setup", "capability-challenge.json"), &challenge)
	writeProtocolJSON(t, filepath.Join(workspace, "setup", "capability-ack.json"), map[string]any{
		"project_id": challenge.ProjectID, "protocol_version": challenge.ProtocolVersion, "nonce": challenge.Nonce,
		"capabilities": map[string]bool{"read": true, "write_utf8_json": true, "write_utf8_md": true},
	})
	if err := os.WriteFile(filepath.Join(workspace, "setup", "capability-write-test.md"), []byte(challenge.MarkdownProbe), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeProtocolFoundation(t *testing.T, workspace string, ready protocolChainReady) {
	t.Helper()
	artifacts := map[string]any{
		"foundation.json":       map[string]any{"title": "协议全链", "protagonist": map[string]any{"entity_type": "character", "local_ref": "p"}, "opening_location": map[string]any{"entity_type": "location", "local_ref": "start"}},
		"characters.json":       map[string]any{"characters": []any{map[string]any{"local_id": "p", "name": "主角"}}},
		"world.json":            map[string]any{"entities": []any{map[string]any{"entity_type": "location", "local_id": "start", "name": "起点"}}},
		"book_plan.json":        map[string]any{"direction": "完成主线"},
		"ending_contract.json":  map[string]any{"main_resolution": "主线明确收束"},
		"style_profile.json":    map[string]any{"language": "zh-CN"},
		"platform_profile.json": map[string]any{"platform": "fanqie"},
	}
	base := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	files := make([]string, 0, len(artifacts))
	for name, value := range artifacts {
		writeProtocolJSON(t, filepath.Join(base, name), value)
		files = append(files, name)
	}
	writeProtocolManifest(t, filepath.Join(base, "manifest.json"), ready, files)
}

func writeProtocolChapter(t *testing.T, workspace string, ready protocolChainReady, mode string) {
	t.Helper()
	chapter := 0
	if _, err := fmt.Sscanf(ready.Target, "chapter:%d", &chapter); err != nil || chapter <= 0 {
		t.Fatalf("target=%q", ready.Target)
	}
	var contextDoc map[string]any
	readProtocolJSON(t, filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID, "context.json"), &contextDoc)
	foundationRef := contextDoc["foundation_reference"].(map[string]any)
	characters := foundationRef["characters"].(map[string]any)["characters"].([]any)
	world := foundationRef["world"].(map[string]any)["entities"].([]any)
	characterID := characters[0].(map[string]any)["canon_id"].(string)
	locationID := world[0].(map[string]any)["canon_id"].(string)

	contract := map[string]any{"chapter": chapter, "declared_pov": characterID}
	plan := map[string]any{
		"chapter": chapter, "base_canon_root": ready.BaseCanonRoot,
		"objective": "推进当前章节主线", "reader_payoff": "形成明确剧情推进",
		"beats": []any{
			map[string]any{"id": "beat-1", "intent": "建立当前压力"},
			map[string]any{"id": "beat-2", "intent": "行动并形成结果"},
		},
		"ending_hook_intent": "推出下一章问题",
	}
	review := map[string]any{"ok": true}
	qualityDimensions := map[string]any{}
	for _, name := range []string{"story_progress", "reader_payoff", "pacing", "character_consistency", "world_consistency", "continuity", "ending_hook"} {
		qualityDimensions[name] = map[string]any{"status": "pass", "note": "protocol E2E quality check"}
	}
	chapterReview := map[string]any{
		"subject_ref": "chapter.md", "plan_ref": "chapter_plan.json", "verdict": "pass",
		"dimensions": qualityDimensions, "issues": []any{},
	}
	changes := []any{}
	if mode == "rewrite" {
		contract["chapter"] = chapter + 100
	}
	if mode == "block" {
		contract["hard_constraints"] = []any{map[string]any{"id": "hc-choice"}}
		review = map[string]any{"ok": false, "author_decision_required": map[string]any{
			"constraint_refs": []string{"hc-choice"}, "conflict": "必须由作者二选一", "options": []string{"选择A", "选择B"},
		}}
	}
	if mode == "ending" {
		changes = []any{map[string]any{"kind": "ending_resolution", "event_ref": fmt.Sprintf("e%d", chapter)}}
	}
	anchor := fmt.Sprintf("第%d章锚点", chapter)
	base := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "chapter.md"), []byte(fmt.Sprintf("第%d章正文。%s。主角继续行动。", chapter, anchor)), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProtocolJSON(t, filepath.Join(base, "chapter_contract.json"), contract)
	writeProtocolJSON(t, filepath.Join(base, "chapter_plan.json"), plan)
	writeProtocolJSON(t, filepath.Join(base, "chapter_review.json"), chapterReview)
	writeProtocolJSON(t, filepath.Join(base, "events.json"), map[string]any{"events": []any{map[string]any{
		"local_id": fmt.Sprintf("e%d", chapter), "kind": "onscreen", "evidence_anchor": anchor, "observers": []string{characterID}, "actors": []string{characterID},
	}}})
	writeProtocolJSON(t, filepath.Join(base, "state_delta.json"), map[string]any{"changes": changes})
	writeProtocolJSON(t, filepath.Join(base, "self_review.json"), review)
	writeProtocolManifest(t, filepath.Join(base, "manifest.json"), ready, []string{"chapter.md", "chapter_contract.json", "chapter_plan.json", "chapter_review.json", "events.json", "state_delta.json", "self_review.json"})
	_ = locationID
}

func writeProtocolControl(t *testing.T, workspace, messageID, kind, scope, value string) {
	t.Helper()
	status := readProtocolStatus(t, workspace)
	control := map[string]any{
		"schema_version": 1, "project_id": status.ProjectID, "message_id": messageID, "kind": kind, "base_canon_root": status.CanonRoot,
	}
	if kind == "block_resolution" {
		control["block_id"] = status.BlockID
		control["choice"] = value
	} else {
		control["directive_scope"] = scope
		control["instruction"] = value
		if scope == "historical_revision" {
			control["chapter"] = 1
		}
	}
	base := filepath.Join(workspace, "exchange", "control", "inbox", messageID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProtocolJSON(t, filepath.Join(base, "control.json"), control)
	writeProtocolJSON(t, filepath.Join(base, "manifest.json"), map[string]any{
		"schema_version": 1, "project_id": status.ProjectID, "message_id": messageID,
		"base_canon_root": status.CanonRoot, "protocol_version": status.ProtocolVersion, "files": []string{"control.json"},
	})
}

func writeProtocolManifest(t *testing.T, path string, ready protocolChainReady, files []string) {
	t.Helper()
	writeProtocolJSON(t, path, map[string]any{
		"schema_version": 1, "project_id": ready.ProjectID, "task_id": ready.TaskID, "attempt_id": ready.AttemptID,
		"base_canon_root": ready.BaseCanonRoot, "protocol_version": ready.ProtocolVersion,
		"task_digest": ready.TaskDigest, "completion_nonce": ready.CompletionNonce, "files": files,
	})
}

func waitProtocolReady(t *testing.T, workspace string, accept func(protocolChainReady) bool) protocolChainReady {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	path := filepath.Join(workspace, "exchange", "READY.json")
	for time.Now().Before(deadline) {
		var ready protocolChainReady
		if readProtocolJSONIfExists(path, &ready) == nil && accept(ready) {
			var status protocolChainStatus
			statusPath := filepath.Join(workspace, "exchange", "STATUS.json")
			if readProtocolJSONIfExists(statusPath, &status) == nil && status.ActiveAttemptID == ready.AttemptID && status.ActiveTarget == ready.Target && status.BlockID == ready.BlockID {
				return ready
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("READY did not reach expected state: %s", readProtocolTextIfExists(path))
	return protocolChainReady{}
}

func waitProtocolResult(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var result struct {
			Result string `json:"result"`
		}
		if readProtocolJSONIfExists(path, &result) == nil && result.Result == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("result did not become %s: %s", want, readProtocolTextIfExists(path))
}

func readProtocolStatus(t *testing.T, workspace string) protocolChainStatus {
	t.Helper()
	var status protocolChainStatus
	readProtocolJSON(t, filepath.Join(workspace, "exchange", "STATUS.json"), &status)
	return status
}

func readProtocolJSON(t *testing.T, path string, dst any) {
	t.Helper()
	if err := readProtocolJSONIfExists(path, dst); err != nil {
		t.Fatal(err)
	}
}

func readProtocolJSONIfExists(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func writeProtocolJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readProtocolText(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func readProtocolTextIfExists(path string) string {
	raw, _ := os.ReadFile(path)
	return string(raw)
}
