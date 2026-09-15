package core

import "testing"

func TestConflictParticipantsMustBeUniqueNonEmptyStringArray(t *testing.T) {
	tests := []struct {
		name         string
		participants any
		wantResult   string
		wantText     string
	}{
		{"string", "character-000001", "REWRITE", "participants"},
		{"mixed", []any{"character-000001", 1}, "REWRITE", "participants"},
		{"blank", []any{"character-000001", ""}, "REWRITE", "participants"},
		{"duplicate", []any{"character-000001", "character-000001"}, "REWRITE", "participants"},
		{"valid", []any{"character-000001"}, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			change := map[string]any{
				"kind": "conflict", "local_id": "c1", "description": "争夺钥匙",
				"participants": tc.participants, "state": "open",
				"escalation_condition": "争夺公开化", "close_condition": "钥匙归属明确", "event_ref": "e1",
			}
			got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{change}))
			if got.Result != tc.wantResult {
				t.Fatalf("settlement=%+v", got)
			}
			if tc.wantText != "" && !containsViolation(got.Violations, tc.wantText) {
				t.Fatalf("violations=%v want substring %q", got.Violations, tc.wantText)
			}
		})
	}
}

func TestConflictTransitionParticipantsMustAlsoBeStructured(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "conflict", "local_id": "c1", "description": "争夺钥匙",
		"participants": []any{"character-000001"}, "state": "open",
		"escalation_condition": "争夺公开化", "close_condition": "钥匙归属明确", "event_ref": "e1",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create settlement=%+v", settled)
	}
	conflictID := mappingID(settled.IDMappings, "conflict", "c1")
	if conflictID == "" {
		t.Fatalf("missing conflict mapping: %+v", settled.IDMappings)
	}

	next := readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{{
		"kind": "conflict", "conflict_id": conflictID, "state": "escalated",
		"participants": []any{"character-000001", 1}, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, next, second)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "participants") {
		t.Fatalf("transition settlement=%+v", got)
	}
}

func TestConflictTransitionCannotExplicitlyClearParticipants(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "conflict", "local_id": "c1", "description": "争夺钥匙",
		"participants": []any{"character-000001"}, "state": "open",
		"escalation_condition": "争夺公开化", "close_condition": "钥匙归属明确", "event_ref": "e1",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create settlement=%+v", settled)
	}
	conflictID := mappingID(settled.IDMappings, "conflict", "c1")
	if conflictID == "" {
		t.Fatalf("missing conflict mapping: %+v", settled.IDMappings)
	}

	next := readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{{
		"kind": "conflict", "conflict_id": conflictID, "state": "escalated",
		"participants": []any{}, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, next, second)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "participants") {
		t.Fatalf("transition settlement=%+v", got)
	}
}
