package core

import (
	"fmt"
	"testing"
)

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

func TestCurrentAttemptCanonicalIDsCannotBeGuessedInReferences(t *testing.T) {
	tests := []struct {
		name     string
		idPrefix string
		build    func(string) map[string][]byte
		wantText string
	}{
		{
			name: "character declared pov", idPrefix: "character", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				artifacts := longformChapterArtifacts(1, []map[string]any{{
					"kind": "character_add", "local_id": "ally", "name": "新同伴", "event_ref": "e1",
				}})
				artifacts["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":1,"declared_pov":%q}`, guessed))
				return artifacts
			},
		},
		{
			name: "location state change", idPrefix: "location", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				return longformChapterArtifacts(1, []map[string]any{
					{"kind": "location_add", "local_id": "station", "name": "旧车站", "event_ref": "e1"},
					{"kind": "location", "character_id": "character-000001", "location_id": guessed, "start_tick": 0, "end_tick": 1, "event_ref": "e1"},
				})
			},
		},
		{
			name: "resource state change", idPrefix: "resource", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				return longformChapterArtifacts(1, []map[string]any{
					{"kind": "resource_add", "local_id": "cash", "name": "现金", "event_ref": "e1"},
					{"kind": "resource", "resource_id": guessed, "delta": 1, "event_ref": "e1"},
				})
			},
		},
		{
			name: "foreshadow transition", idPrefix: "foreshadow", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				return longformChapterArtifacts(1, []map[string]any{
					{"kind": "foreshadow", "local_id": "umbrella", "description": "红伞线索", "state": "seeded", "event_ref": "e1"},
					{"kind": "foreshadow", "foreshadow_id": guessed, "state": "reinforced", "event_ref": "e1"},
				})
			},
		},
		{
			name: "reader promise transition", idPrefix: "reader-promise", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				return longformChapterArtifacts(1, []map[string]any{
					{"kind": "reader_promise", "local_id": "promise", "statement": "揭晓红伞主人", "state": "advanced"},
					{"kind": "reader_promise", "promise_id": guessed, "state": "deferred", "deadline_chapter": 3},
				})
			},
		},
		{
			name: "conflict transition", idPrefix: "conflict", wantText: "current attempt",
			build: func(guessed string) map[string][]byte {
				return longformChapterArtifacts(1, []map[string]any{
					{"kind": "conflict", "local_id": "rivalry", "description": "争夺钥匙", "participants": []string{"character-000001"}, "state": "open", "escalation_condition": "争夺公开化", "close_condition": "钥匙归属明确", "event_ref": "e1"},
					{"kind": "conflict", "conflict_id": guessed, "state": "escalated", "event_ref": "e1"},
				})
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			state, err := project.store.LoadCoreProductionState()
			if err != nil || state == nil {
				t.Fatalf("production state=%+v err=%v", state, err)
			}
			guessed := fmt.Sprintf("%s-%06d", tc.idPrefix, state.NextEntitySeq)
			got := submitAndSettleChapter(t, project, workspace, ready, tc.build(guessed))
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.wantText) {
				t.Fatalf("guessed current id settlement=%+v guessed=%s", got, guessed)
			}
		})
	}
}
