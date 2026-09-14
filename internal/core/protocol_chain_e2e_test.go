package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenhongyang/novel-studio/internal/core"
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

func TestPublicFileProtocolFullChain(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := core.InitProject(core.InitOptions{ProjectID: "protocol-chain-test", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
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
	review := map[string]any{"ok": true}
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
	writeProtocolJSON(t, filepath.Join(base, "events.json"), map[string]any{"events": []any{map[string]any{
		"local_id": fmt.Sprintf("e%d", chapter), "kind": "onscreen", "evidence_anchor": anchor, "observers": []string{characterID}, "actors": []string{characterID},
	}}})
	writeProtocolJSON(t, filepath.Join(base, "state_delta.json"), map[string]any{"changes": changes})
	writeProtocolJSON(t, filepath.Join(base, "self_review.json"), review)
	writeProtocolManifest(t, filepath.Join(base, "manifest.json"), ready, []string{"chapter.md", "chapter_contract.json", "events.json", "state_delta.json", "self_review.json"})
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
