package core

import (
	"fmt"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func TestKnowledgeFactStatementIsConsistentAcrossCharacters(t *testing.T) {
	project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	statement := "钥匙在钟楼地下室"
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": firstID, "fact_id": "fact-key", "statement": statement,
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	first["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":[%q]}]}`, firstID))
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", got)
	}

	ready = readReady(t, workspace)
	bad := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": secondID, "fact_id": "fact-key", "statement": "钥匙在阁楼",
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	bad["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":[%q]}]}`, secondID))
	got := submitAndSettleChapter(t, project, workspace, ready, bad)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("conflicting statement=%+v", got)
	}

	ready = readReady(t, workspace)
	good := longformChapterArtifacts(2, []map[string]any{{
		"kind": "knowledge_add", "character_id": secondID, "fact_id": "fact-key",
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	good["events.json"] = []byte(fmt.Sprintf(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":[%q]}]}`, secondID))
	if got := submitAndSettleChapter(t, project, workspace, ready, good); got.Result != "ACCEPTED" {
		t.Fatalf("inherited statement=%+v", got)
	}

	raw, err := project.store.ReadCoreCanonStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	var canon domain.CoreCanonState
	if err := protocol.DecodeJSON(raw, &canon); err != nil {
		t.Fatal(err)
	}
	if got := canon.Longform.Knowledge[secondID]["fact-key"].Statement; got != statement {
		t.Fatalf("statement=%q want %q", got, statement)
	}
}
