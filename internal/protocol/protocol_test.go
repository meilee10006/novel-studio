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

func TestDecodeJSONRejectsDuplicateObjectKeys(t *testing.T) {
	for _, raw := range []string{
		`{"project_id":"book-a","project_id":"book-b"}`,
		`{"outer":{"id":"first","id":"second"}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var value any
			if err := DecodeJSON([]byte(raw), &value); err == nil {
				t.Fatalf("duplicate JSON object key unexpectedly accepted: %s", raw)
			}
		})
	}
}

func TestDecodeJSONRejectsLossyIntegerLiteral(t *testing.T) {
	for _, raw := range []string{
		`{"n":9007199254740993}`,
		`{"n":9007199254740993.0}`,
		`{"n":9.007199254740993e15}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var value map[string]any
			if err := DecodeJSON([]byte(raw), &value); err == nil {
				t.Fatalf("lossy integer unexpectedly accepted as %#v", value["n"])
			}
		})
	}
}

func TestDecodeJSONKeepsSupportedNumbersCompatible(t *testing.T) {
	for _, raw := range []string{
		`{"n":0.1}`,
		`{"n":9007199254740992}`,
		`{"n":1e3}`,
	} {
		t.Run(raw, func(t *testing.T) {
			var value map[string]any
			if err := DecodeJSON([]byte(raw), &value); err != nil {
				t.Fatalf("supported number rejected: %v", err)
			}
		})
	}
}

func TestChatGPTProtocolDescribesSubmissionAndControlSchemas(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"manifest.json", "schema_version", "project_id", "task_id", "attempt_id", "JSON 对象中的键不得重复", "整数值必须能被 Core 精确保留",
		"base_canon_root", "protocol_version", "task_digest", "completion_nonce", "files",
		"foundation.json", "title", "protagonist 必须引用 character", "opening_location 必须引用 location", "characters.json", "characters 必须是数组", "world.json", "world.entities 一旦出现必须是数组", "每个 Foundation entity 必须有非空字符串 name", "local_id", "entity_type", "local_ref 一旦出现必须是非空字符串", "Foundation 首次定义不得自行提供 canon_id",
		"book_plan.json", "direction", "ending_contract.json", "main_resolution",
		"style_profile.json", "language", "platform_profile.json", "platform",
		"chapter.md", "chapter_contract.json", "chapter", "declared_pov", "events.json",
		"evidence_anchor", "kind 只支持 onscreen / offscreen", "events.json 不得自行提供 canon_id", "event_ref 与 event_canon_id 不得同时提供", "event_ref 与 event_canon_id 字段键不得同时出现", "event_canon_id 一旦出现必须是非空字符串", "event_canon_id 必须引用已验收 story event", "当前 attempt 新事件只能使用 event_ref", "actors/observers 一旦出现必须是字符串数组", "actors/observers 数组内不得重复人物引用", "kind=offscreen", "离屏事件", "hard_constraints[].id", "state_delta.json", "state_delta.json.changes 必须是数组", "event_ref", "event_ref 一旦出现必须是非空字符串", "self_review.json", "self_review.ok 必须是 boolean", "ok:false 必须同时提供 author_decision_required", "ok:true 不得同时提供 author_decision_required",
		"foundation_reference", "block_resolution", "author_directive", "future_plan",
		"historical_revision", "block_id", "choice", "instruction",
		"active_attempt_id", "active_target", "STATUS.block_id == READY.block_id", "export_ready", "export_problems", "不一致",
		"hard_constraints", "hard_constraints 一旦出现必须是数组", "唯一非空 id", "author_decision_required", "constraint_refs", "constraint_refs 必须是非空字符串数组", "offscreen constraint_refs 的重复引用会被兼容接受", "conflict", "options",
		"character_add", "location_add", "resource_add", "description 一旦出现必须是字符串", "Core-owned ID 字段必须完全省略", "即使值为 null", "同一提交", "id_mappings", "首次创建不得自行提供永久 ID 字段",
		"knowledge_add", "character_id", "fact_id", "statement", "knowledge statement 一旦出现必须是字符串", "source", "observed", "transmitted", "from_character_id 必须出现在 event actors", "transmitted 不得改写来源 fact 的 statement", "同一角色已有非空 statement 时不得改写", "同一 fact_id 跨角色共享同一 statement 语义",
		"resource_id", "delta", "location_id", "start_tick", "end_tick", "location.start_tick 不得早于该角色前态 end_tick",
		"relationship_id", "两个不同 canonical characters", "tags", "tags 一旦出现必须是唯一非空字符串数组", "change kind 必须来自支持列表", "foreshadow_id", "foreshadow.description", "foreshadow.description 一旦出现必须是字符串", "首次创建伏笔使用 local_id", "永久 foreshadow ID", "description 创建后不可改写", "payoff_ready", "paid_off", "closed",
		"promise_id", "reader_promise.statement", "reader_promise.statement 一旦出现必须是字符串", "首次创建读者承诺使用 local_id", "永久 reader promise ID", "statement 创建后不可改写", "advanced", "fulfilled", "deferred", "deadline_chapter", "retired", "fulfilled / retired 为终态",
		"conflict_id", "首次创建冲突使用 local_id", "永久 conflict ID", "participants", "participants 必须是唯一非空字符串数组", "escalation_condition", "close_condition", "定义性字段创建后不可改写", "open → escalated → resolved",
		"ending_resolution", "travel_constraints", "min_ticks", "planning_patch.json", "next_arc", "rolling_planning_due", "提前规划阶段仍可省略 planning_patch.json",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing %q", want)
		}
	}
}

func TestChatGPTProtocolDescribesSemanticDesignLifecycle(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"design_mode",
		"exchange/design/inbox",
		"story_locked",
		"foundation_ready",
		"author_confirmation",
		"foundation_readiness_review",
		"expected_design_root",
		"没有 READY",
		"foundation_design_root",
		"legacy",
		"required",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing semantic design guidance %q", want)
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

func TestChatGPTProtocolDescribesStructuredRewriteFeedback(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"rewrite_feedback",
		"code",
		"entity_ref",
		"expected",
		"observed",
		"evidence_refs",
		"allowed_scope",
		"violations",
		"一一对应",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing structured REWRITE guidance %q", want)
		}
	}
}

func TestChatGPTProtocolDescribesCompleteBackwardCompatibleChapterContract(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"start_state",
		"purpose",
		"main_conflict",
		"reader_question",
		"obligation_refs",
		"immutable_refs",
		"foreshadow_operations",
		"allowed_states",
		"expected_changes",
		"ending_hook",
		"target_length",
		"min_chars",
		"max_chars",
		"planning_obligations",
		"历史最小 1.0 chapter_contract 仍可读取",
		"新提交应使用完整 chapter_contract",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing complete chapter contract guidance %q", want)
		}
	}
}

func TestChatGPTProtocolDescribesChapterQualityArtifactChain(t *testing.T) {
	text := RenderChatGPTProtocol("book-1")
	for _, want := range []string{
		"task.json.required_artifacts 是当前 attempt 的唯一文件合同",
		"chapter_plan.json",
		"先写 chapter_plan.json，再写 chapter.md",
		"chapter_review.json",
		"subject_ref",
		"plan_ref",
		"story_progress",
		"reader_payoff",
		"character_consistency",
		"world_consistency",
		"verdict=revise",
		"arc_rehearsal.json",
		"至少两个候选",
		"selected_scenario_id",
		"只有 planning_patch.json 改变权威 planning",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated ChatGPT protocol missing chapter-quality guidance %q", want)
		}
	}
}
