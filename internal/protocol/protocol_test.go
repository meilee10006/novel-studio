package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeTextRejectsTraversalAbsoluteSymlinkAndOversize(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"../escape.json", filepath.Join(outside, "secret.json"), "escape/secret.json"} {
		if _, err := ReadUTF8(root, rel, 1024); err == nil {
			t.Fatalf("ReadUTF8(%q) unexpectedly succeeded", rel)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "large.md"), []byte(strings.Repeat("x", 33)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUTF8(root, "large.md", 32); err == nil {
		t.Fatal("oversize input unexpectedly accepted")
	}
}

func TestAtomicWriteRejectsSymlinkParentAndNonProtocolExtension(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := WriteUTF8Atomic(root, "escape/pwned.json", []byte("{}"), 0o644); err == nil {
		t.Fatal("write through symlink unexpectedly accepted")
	}
	if err := WriteUTF8Atomic(root, "script.sh", []byte("echo no"), 0o644); err == nil {
		t.Fatal("non-protocol extension unexpectedly accepted")
	}
}

func TestChatGPTProtocolCarriesCurrentVersion(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	if !strings.Contains(text, CurrentVersion) || !strings.Contains(text, "book-1") {
		t.Fatalf("protocol text missing version/project: %q", text)
	}
}

func TestProtocolTextRejectsInvalidUTF8AndExcessiveJSONShape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.json"), []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadUTF8(root, "bad.json", 1024); err == nil {
		t.Fatal("invalid UTF-8 unexpectedly accepted")
	}

	deep := strings.Repeat("[", MaxJSONDepth+1) + "0" + strings.Repeat("]", MaxJSONDepth+1)
	var value any
	if err := DecodeJSON([]byte(deep), &value); err == nil {
		t.Fatal("excessive JSON depth unexpectedly accepted")
	}

	wide := "[" + strings.Repeat("0,", MaxJSONArrayLength) + "0]"
	if err := DecodeJSON([]byte(wide), &value); err == nil {
		t.Fatal("excessive JSON array unexpectedly accepted")
	}
}

func TestChatGPTProtocolDescribesSubmissionAndControlSchemas(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"manifest.json", "schema_version", "project_id", "task_id", "attempt_id",
		"base_canon_root", "protocol_version", "task_digest", "completion_nonce", "files",
		"foundation.json", "title", "characters.json", "characters 必须是数组", "world.json", "world.entities 一旦出现必须是数组", "local_id", "entity_type", "local_ref 一旦出现必须是非空字符串",
		"book_plan.json", "direction", "ending_contract.json", "main_resolution",
		"style_profile.json", "language", "platform_profile.json", "platform",
		"chapter.md", "chapter_contract.json", "chapter", "declared_pov", "events.json",
		"evidence_anchor", "kind 只支持 onscreen / offscreen", "actors/observers 一旦出现必须是字符串数组", "kind=offscreen", "离屏事件", "hard_constraints[].id", "state_delta.json", "state_delta.json.changes 必须是数组", "event_ref", "self_review.json",
		"foundation_reference", "block_resolution", "author_directive", "future_plan",
		"historical_revision", "block_id", "choice", "instruction",
		"active_attempt_id", "active_target", "STATUS.block_id == READY.block_id", "export_ready", "export_problems", "不一致",
		"hard_constraints", "hard_constraints 一旦出现必须是数组", "唯一非空 id", "author_decision_required", "constraint_refs", "constraint_refs 必须是非空字符串数组", "conflict", "options",
		"character_add", "location_add", "resource_add", "同一提交", "id_mappings",
		"knowledge_add", "character_id", "fact_id", "statement", "source", "observed",
		"resource_id", "delta", "location_id", "start_tick", "end_tick",
		"relationship_id", "tags", "tags 一旦出现必须是唯一非空字符串数组", "change kind 必须来自支持列表", "foreshadow_id", "foreshadow.description", "首次创建伏笔使用 local_id", "永久 foreshadow ID", "payoff_ready", "paid_off", "closed",
		"promise_id", "reader_promise.statement", "首次创建读者承诺使用 local_id", "永久 reader promise ID", "advanced", "fulfilled", "deferred", "deadline_chapter", "retired",
		"conflict_id", "首次创建冲突使用 local_id", "永久 conflict ID", "participants", "participants 必须是唯一非空字符串数组", "escalation_condition", "close_condition", "open → escalated → resolved",
		"ending_resolution", "travel_constraints", "min_ticks", "planning_patch.json", "next_arc",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing %q", want)
		}
	}
}

func TestChatGPTProtocolJSONExamplesAreValid(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	parts := strings.Split(text, "~~~json\n")
	if len(parts) < 2 {
		t.Fatal("generated ChatGPT protocol has no JSON examples")
	}
	for i, part := range parts[1:] {
		block, _, ok := strings.Cut(part, "\n~~~")
		if !ok {
			t.Fatalf("JSON example %d is missing closing fence", i+1)
		}
		if !json.Valid([]byte(block)) {
			t.Errorf("JSON example %d is not valid JSON: %s", i+1, block)
		}
	}
}
