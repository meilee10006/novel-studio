package core

import "testing"

func TestStoryEventCanonicalIDsAreCoreOwned(t *testing.T) {
	t.Run("event definition cannot predeclare canon id", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","canon_id":"story-event-999999","kind":"onscreen","evidence_anchor":"主角看见门口的灯","observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("current event ref cannot include predeclared canonical id", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, []map[string]any{{
			"kind": "resource_add", "local_id": "cash", "name": "现金", "event_ref": "e1", "event_canon_id": "story-event-999999",
		}})
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("historical canonical event remains valid evidence", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		first := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
		if first.Result != "ACCEPTED" {
			t.Fatalf("chapter1=%+v", first)
		}
		eventID := mappingID(first.IDMappings, "story_event", "e1")
		if eventID == "" {
			t.Fatalf("missing event mapping: %+v", first.IDMappings)
		}
		next := readReady(t, workspace)
		second := longformChapterArtifacts(2, []map[string]any{{
			"kind": "resource_add", "local_id": "cash", "name": "现金", "event_canon_id": eventID,
		}})
		got := submitAndSettleChapter(t, project, workspace, next, second)
		if got.Result != "ACCEPTED" {
			t.Fatalf("historical event evidence=%+v", got)
		}
	})
}
