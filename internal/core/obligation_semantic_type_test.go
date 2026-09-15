package core

import "testing"

func TestForeshadowTransitionDescriptionMustBeStringWhenPresent(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "foreshadow", "local_id": "umbrella", "description": "红伞与旧案有关", "state": "seeded", "event_ref": "e1",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create=%+v", settled)
	}
	id := mappingID(settled.IDMappings, "foreshadow", "umbrella")
	next := readReady(t, workspace)
	change := longformChapterArtifacts(2, []map[string]any{{
		"kind": "foreshadow", "foreshadow_id": id, "description": 123, "state": "reinforced", "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, next, change)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "description") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestReaderPromiseTransitionStatementMustBeStringWhenPresent(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "reader_promise", "local_id": "umbrella", "statement": "揭晓红伞主人", "state": "advanced",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create=%+v", settled)
	}
	id := mappingID(settled.IDMappings, "reader_promise", "umbrella")
	next := readReady(t, workspace)
	change := longformChapterArtifacts(2, []map[string]any{{
		"kind": "reader_promise", "promise_id": id, "statement": map[string]any{"bad": true}, "state": "deferred", "deadline_chapter": 5,
	}})
	got := submitAndSettleChapter(t, project, workspace, next, change)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("settlement=%+v", got)
	}
}
