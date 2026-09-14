package core

import (
	"fmt"
	"testing"
)

func TestKnowledgeTransmissionPreservesFactStatement(t *testing.T) {
	project, workspace, sourceID, recipientID, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": sourceID, "fact_id": "fact-key",
		"statement": "钥匙在钟楼地下室", "source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	first["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":1,"declared_pov":%q}`, sourceID))
	first["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":[%q]}]}`, sourceID))
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("seed knowledge=%+v", got)
	}

	next := readReady(t, workspace)
	bad := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": recipientID, "fact_id": "fact-key",
		"statement": "钥匙在阁楼", "source": map[string]any{"kind": "transmitted", "from_character_id": sourceID, "event_ref": "e1"},
	}})
	bad["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":2,"declared_pov":%q}`, recipientID))
	bad["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":[%q],"observers":[%q]}]}`, sourceID, recipientID))
	got := submitAndSettleChapter(t, project, workspace, next, bad)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("conflicting transmitted statement=%+v", got)
	}

	retry := readReady(t, workspace)
	good := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": recipientID, "fact_id": "fact-key",
		"source": map[string]any{"kind": "transmitted", "from_character_id": sourceID, "event_ref": "e1"},
	}})
	good["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":2,"declared_pov":%q}`, recipientID))
	good["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":[%q],"observers":[%q]}]}`, sourceID, recipientID))
	if got := submitAndSettleChapter(t, project, workspace, retry, good); got.Result != "ACCEPTED" {
		t.Fatalf("inherited transmitted statement=%+v", got)
	}
	state := readCanonState(t, project)
	rawFact, ok := state.Longform.Knowledge[recipientID]["fact-key"].(map[string]any)
	if !ok {
		t.Fatalf("recipient fact=%#v", state.Longform.Knowledge[recipientID]["fact-key"])
	}
	statement, _ := rawFact["statement"].(string)
	if statement != "钥匙在钟楼地下室" {
		t.Fatalf("recipient statement=%q", statement)
	}
}
