package core

import (
	"encoding/json"
	"testing"
)

func TestExtendedChapterContractStructure(t *testing.T) {
	tests := []struct {
		name     string
		contract string
		want     string
	}{
		{"purpose type", `{"chapter":1,"declared_pov":"character-000001","purpose":7}`, "purpose"},
		{"blank reader question", `{"chapter":1,"declared_pov":"character-000001","reader_question":"  "}`, "reader_question"},
		{"duplicate obligations", `{"chapter":1,"declared_pov":"character-000001","obligation_refs":["promise-1","promise-1"]}`, "obligation_refs"},
		{"bad target length", `{"chapter":1,"declared_pov":"character-000001","target_length":{"min_chars":20,"max_chars":10}}`, "target_length"},
		{"unknown expected change", `{"chapter":1,"declared_pov":"character-000001","expected_changes":["teleport"]}`, "expected_changes"},
		{"bad foreshadow operation", `{"chapter":1,"declared_pov":"character-000001","foreshadow_operations":[{"foreshadow_id":"f"}]}`, "foreshadow_operations"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			artifacts["chapter_contract.json"] = []byte(tc.contract)
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.want) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func TestExtendedChapterContractKeepsLegacyMinimumCompatible(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
	if got.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestExtendedChapterContractChecksMechanicalSemantics(t *testing.T) {
	t.Run("length outside range", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","target_length":{"min_chars":1,"max_chars":2}}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "target_length") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("expected change missing", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","expected_changes":["relationship"]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "expected_changes") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	for _, field := range []string{"obligation_refs", "immutable_refs"} {
		field := field
		t.Run("unknown "+field, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			contract, _ := json.Marshal(map[string]any{
				"chapter": 1, "declared_pov": "character-000001", field: []string{"unknown-canon-id"},
			})
			artifacts["chapter_contract.json"] = contract
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, field) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}

	t.Run("unknown foreshadow operation", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","foreshadow_operations":[{"foreshadow_id":"foreshadow-999999","allowed_states":["reinforced"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "foreshadow_operations") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("foreshadow transition outside allowed states", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		first := longformChapterArtifacts(1, []map[string]any{{
			"kind": "foreshadow", "local_id": "umbrella", "description": "红伞线索", "state": "seeded", "event_ref": "e1",
		}})
		created := submitAndSettleChapter(t, project, workspace, ready, first)
		if created.Result != "ACCEPTED" {
			t.Fatalf("create=%+v", created)
		}
		foreshadowID := mappingID(created.IDMappings, "foreshadow", "umbrella")
		if foreshadowID == "" {
			t.Fatalf("mappings=%+v", created.IDMappings)
		}
		next := readReady(t, workspace)
		artifacts := longformChapterArtifacts(2, []map[string]any{{
			"kind": "foreshadow", "foreshadow_id": foreshadowID, "state": "reinforced", "event_ref": "e1",
		}})
		contract, _ := json.Marshal(map[string]any{
			"chapter": 2, "declared_pov": "character-000001",
			"foreshadow_operations": []map[string]any{{"foreshadow_id": foreshadowID, "allowed_states": []string{"payoff_ready"}}},
		})
		artifacts["chapter_contract.json"] = contract
		got := submitAndSettleChapter(t, project, workspace, next, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "allowed_states") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("planning obligation not issued", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","planning_obligations":["rolling_planning_due"]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "planning_obligations") {
			t.Fatalf("settlement=%+v", got)
		}
	})
}

func TestExtendedChapterContractAcceptsProvableSemantics(t *testing.T) {
	t.Run("length expected change and known refs", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, []map[string]any{{
			"kind": "character_add", "local_id": "ally", "name": "同伴", "event_ref": "e1",
		}})
		contract, _ := json.Marshal(map[string]any{
			"chapter": 1, "declared_pov": "character-000001",
			"target_length":    map[string]any{"min_chars": 1, "max_chars": 100},
			"expected_changes": []string{"character_add"},
			"obligation_refs":  []string{"character-000001"},
			"immutable_refs":   []string{"character-000001"},
		})
		artifacts["chapter_contract.json"] = contract
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("allowed foreshadow transition", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		created := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{{
			"kind": "foreshadow", "local_id": "umbrella", "description": "红伞线索", "state": "seeded", "event_ref": "e1",
		}}))
		if created.Result != "ACCEPTED" {
			t.Fatalf("create=%+v", created)
		}
		foreshadowID := mappingID(created.IDMappings, "foreshadow", "umbrella")
		next := readReady(t, workspace)
		artifacts := longformChapterArtifacts(2, []map[string]any{{
			"kind": "foreshadow", "foreshadow_id": foreshadowID, "state": "reinforced", "event_ref": "e1",
		}})
		contract, _ := json.Marshal(map[string]any{
			"chapter": 2, "declared_pov": "character-000001",
			"foreshadow_operations": []map[string]any{{"foreshadow_id": foreshadowID, "allowed_states": []string{"reinforced", "payoff_ready"}}},
		})
		artifacts["chapter_contract.json"] = contract
		got := submitAndSettleChapter(t, project, workspace, next, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("issued rolling planning obligation", func(t *testing.T) {
		project, workspace, ready := planningChapterReadyProject(t, 2, 1)
		artifacts := validChapterArtifacts(1)
		var contract map[string]any
		if err := json.Unmarshal(artifacts["chapter_contract.json"], &contract); err != nil {
			t.Fatal(err)
		}
		contract["planning_obligations"] = []string{"rolling_planning_due"}
		artifacts["chapter_contract.json"], _ = json.Marshal(contract)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})
}

func TestExtendedChapterContractStartStateMustMatchAttemptBase(t *testing.T) {
	t.Run("mismatch rewrites", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","start_state":{"canon_root":"wrong-root"}}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "start_state.canon_root") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("matching base accepts", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		contract, _ := json.Marshal(map[string]any{
			"chapter": 1, "declared_pov": "character-000001",
			"start_state": map[string]any{"canon_root": ready.BaseCanonRoot},
		})
		artifacts["chapter_contract.json"] = contract
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
