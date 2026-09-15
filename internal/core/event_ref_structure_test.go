package core

import "testing"

func TestEventRefMustBeNonEmptyStringWhenPresent(t *testing.T) {
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
	artifacts := longformChapterArtifacts(2, []map[string]any{{
		"kind": "resource_add", "local_id": "cash", "name": "现金", "event_ref": 123, "event_canon_id": eventID,
	}})
	got := submitAndSettleChapter(t, project, workspace, next, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "event_ref") {
		t.Fatalf("settlement=%+v", got)
	}
}
