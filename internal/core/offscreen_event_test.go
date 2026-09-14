package core

import (
	"testing"
)

func TestOffscreenEventRequiresKnownHardConstraint(t *testing.T) {
	t.Run("missing constraint refs", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"offscreen","actors":["character-000001"],"observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "offscreen") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("unknown constraint ref", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-allowed"}]}`)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"offscreen","constraint_refs":["hc-missing"],"actors":["character-000001"],"observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "constraint") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("known constraint ref", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-offscreen"}]}`)
		artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"offscreen","constraint_refs":["hc-offscreen"],"actors":["character-000001"],"observers":["character-000001"]}]}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
