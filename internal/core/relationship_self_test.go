package core

import "testing"

func TestRelationshipCannotReferenceSameCharacterTwice(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "relationship", "relationship_id": "character-000001|character-000001",
		"tags": []string{"自我信任"}, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "different") {
		t.Fatalf("settlement=%+v", got)
	}
}
