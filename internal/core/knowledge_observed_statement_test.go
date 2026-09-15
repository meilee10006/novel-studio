package core

import (
	"encoding/json"
	"testing"
)

func TestNewObservedKnowledgeRequiresReadableStatement(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "fact-key",
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestObservedKnowledgePreservesExistingFactStatement(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "fact-key",
		"statement": "钥匙在钟楼地下室", "source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("seed knowledge=%+v", got)
	}

	next := readReady(t, workspace)
	bad := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "fact-key",
		"statement": "钥匙在阁楼", "source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	if got := submitAndSettleChapter(t, project, workspace, next, bad); got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("conflicting observed statement=%+v", got)
	}

	retry := readReady(t, workspace)
	good := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "fact-key",
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	if got := submitAndSettleChapter(t, project, workspace, retry, good); got.Result != "ACCEPTED" {
		t.Fatalf("repeat observed knowledge=%+v", got)
	}
	state := readCanonState(t, project)
	raw := state.Longform.Knowledge["character-000001"]["fact-key"]
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var fact struct {
		Statement string `json:"statement"`
	}
	if err := json.Unmarshal(encoded, &fact); err != nil {
		t.Fatal(err)
	}
	if fact.Statement != "钥匙在钟楼地下室" {
		t.Fatalf("statement=%q", fact.Statement)
	}
}
