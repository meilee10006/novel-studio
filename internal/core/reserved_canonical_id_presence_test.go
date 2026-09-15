package core

import "testing"

func TestReservedCanonicalIDFieldsAreRejectedByPresence(t *testing.T) {
	t.Run("chapter objects", func(t *testing.T) {
		tests := []struct {
			name   string
			change map[string]any
		}{
			{"character canon number", map[string]any{"kind": "character_add", "local_id": "ally", "name": "新同伴", "canon_id": 123, "event_ref": "e1"}},
			{"foreshadow id object", map[string]any{"kind": "foreshadow", "local_id": "umbrella", "description": "红伞线索", "foreshadow_id": map[string]any{"bad": true}, "state": "seeded", "event_ref": "e1"}},
			{"promise id boolean", map[string]any{"kind": "reader_promise", "local_id": "promise", "statement": "揭晓红伞主人", "promise_id": true, "state": "advanced"}},
			{"conflict id array", map[string]any{"kind": "conflict", "local_id": "rivalry", "conflict_id": []any{"bad"}, "description": "争夺钥匙", "participants": []string{"character-000001"}, "state": "open", "escalation_condition": "公开化", "close_condition": "归属明确", "event_ref": "e1"}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				project, _, workspace, ready := acceptedFoundationProject(t)
				got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{tc.change}))
				if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
					t.Fatalf("settlement=%+v", got)
				}
			})
		}
	})

	t.Run("story event canon null", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","canon_id":null,"kind":"onscreen","evidence_anchor":"主角看见门口的灯"}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("foundation reference canon null", func(t *testing.T) {
		project, _, workspace := newCapabilityPassedProject(t)
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		ready := readReady(t, workspace)
		artifacts := validFoundationArtifacts()
		artifacts["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same","canon_id":null},"opening_location":{"entity_type":"location","local_ref":"same"}}`)
		got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("foundation canon number", func(t *testing.T) {
		project, _, workspace := newCapabilityPassedProject(t)
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		ready := readReady(t, workspace)
		artifacts := validFoundationArtifacts()
		artifacts["characters.json"] = []byte(`{"characters":[{"local_id":"same","name":"主角","canon_id":123}]}`)
		got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
