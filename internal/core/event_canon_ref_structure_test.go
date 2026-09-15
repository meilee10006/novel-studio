package core

import "testing"

func TestEventCanonReferenceStructureIsStrict(t *testing.T) {
	t.Run("event ref cannot coexist with canon field even null", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, []map[string]any{{
			"kind": "resource_add", "local_id": "cash", "name": "现金", "event_ref": "e1", "event_canon_id": nil,
		}})
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("canon event id must be string when present", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, []map[string]any{{
			"kind": "reader_promise", "local_id": "promise", "statement": "揭晓红伞主人", "state": "advanced", "event_canon_id": 123,
		}})
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "event_canon_id") {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
