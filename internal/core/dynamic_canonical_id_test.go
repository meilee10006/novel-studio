package core

import "testing"

func TestNewChapterObjectsCannotPredeclareCanonicalIDs(t *testing.T) {
	tests := []struct {
		name   string
		change map[string]any
	}{
		{"character", map[string]any{"kind": "character_add", "local_id": "ally", "name": "新同伴", "canon_id": "character-999999", "event_ref": "e1"}},
		{"location", map[string]any{"kind": "location_add", "local_id": "station", "name": "旧车站", "canon_id": "location-999999", "event_ref": "e1"}},
		{"resource", map[string]any{"kind": "resource_add", "local_id": "cash", "name": "现金", "canon_id": "resource-999999", "event_ref": "e1"}},
		{"foreshadow", map[string]any{"kind": "foreshadow", "local_id": "umbrella", "description": "红伞线索", "foreshadow_id": "foreshadow-999999", "state": "seeded", "event_ref": "e1"}},
		{"reader promise", map[string]any{"kind": "reader_promise", "local_id": "promise", "statement": "揭晓红伞主人", "promise_id": "reader-promise-999999", "state": "advanced"}},
		{"conflict", map[string]any{"kind": "conflict", "local_id": "rivalry", "conflict_id": "conflict-999999", "description": "争夺钥匙", "participants": []string{"character-000001"}, "state": "open", "escalation_condition": "争夺公开化", "close_condition": "钥匙归属明确", "event_ref": "e1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, []map[string]any{tc.change})
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
