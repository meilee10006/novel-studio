package core

import "testing"

func TestKnowledgeStatementMustBeStringWhenPresent(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "fact-key",
		"statement": 123, "source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":["character-000001"]}]}`)
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("settlement=%+v", got)
	}
}
