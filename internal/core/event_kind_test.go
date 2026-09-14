package core

import "testing"

func TestStoryEventKindMustBeExplicitAndSupported(t *testing.T) {
	t.Run("missing kind", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","evidence_anchor":"主角看见门口的灯","observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "kind") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("unsupported kind", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"telepathy","evidence_anchor":"主角看见门口的灯","observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "kind") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("onscreen", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
