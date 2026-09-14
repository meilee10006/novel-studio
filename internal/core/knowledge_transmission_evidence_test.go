package core

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestKnowledgeTransmissionRequiresSourceActorAndRecipientObserver(t *testing.T) {
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
	second := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": recipientID, "fact_id": "fact-key",
		"statement": "钥匙在钟楼地下室",
		"source":    map[string]any{"kind": "transmitted", "from_character_id": sourceID, "event_ref": "e1"},
	}})
	second["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":2,"declared_pov":%q}`, recipientID))
	// Recipient observes the event, but the claimed source character is absent from actors.
	second["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":[%q]}]}`, recipientID))
	got := submitAndSettleChapter(t, project, workspace, next, second)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "transmission source") {
		t.Fatalf("missing source actor settlement=%+v", got)
	}

	// Sanity-check the same transmission with an explicit source actor.
	retry := readReady(t, workspace)
	third := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": recipientID, "fact_id": "fact-key",
		"statement": "钥匙在钟楼地下室",
		"source":    map[string]any{"kind": "transmitted", "from_character_id": sourceID, "event_ref": "e1"},
	}})
	third["chapter_contract.json"] = []byte(fmt.Sprintf(`{"chapter":2,"declared_pov":%q}`, recipientID))
	events, _ := json.Marshal(map[string]any{"events": []map[string]any{{
		"local_id": "e1", "kind": "onscreen", "evidence_anchor": "主角看见门口的灯",
		"actors": []string{sourceID}, "observers": []string{recipientID},
	}}})
	third["events.json"] = events
	if got := submitAndSettleChapter(t, project, workspace, retry, third); got.Result != "ACCEPTED" {
		t.Fatalf("evidenced transmission=%+v", got)
	}
}
