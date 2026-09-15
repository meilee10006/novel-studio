package core

import "testing"

func TestStoryEventParticipantFieldsMustBeStringArrays(t *testing.T) {
	tests := []struct {
		name       string
		eventJSON  string
		wantResult string
		wantText   string
	}{
		{"actors string", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":"character-000001"}]}`, "REWRITE", "actors"},
		{"observers number", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":1}]}`, "REWRITE", "observers"},
		{"actors mixed", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":["character-000001",1]}]}`, "REWRITE", "actors"},
		{"actors duplicate", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":["character-000001","character-000001"]}]}`, "REWRITE", "duplicate"},
		{"observers duplicate", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":["character-000001","character-000001"]}]}`, "REWRITE", "duplicate"},
		{"valid arrays", `{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","actors":["character-000001"],"observers":["character-000001"]}]}`, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			artifacts["events.json"] = []byte(tc.eventJSON)
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != tc.wantResult {
				t.Fatalf("settlement=%+v", got)
			}
			if tc.wantText != "" && !containsViolation(got.Violations, tc.wantText) {
				t.Fatalf("violations=%v want substring %q", got.Violations, tc.wantText)
			}
		})
	}
}
