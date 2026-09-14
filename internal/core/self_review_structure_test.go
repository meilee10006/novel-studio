package core

import "testing"

func TestSelfReviewOKMustBeBooleanAndConsistentWithOutcome(t *testing.T) {
	t.Run("missing ok rewrites", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["self_review.json"] = []byte(`{}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "self_review.ok") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("non boolean ok rewrites", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["self_review.json"] = []byte(`{"ok":"yes"}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "self_review.ok") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("false without decision rewrites", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		artifacts := longformChapterArtifacts(1, nil)
		artifacts["self_review.json"] = []byte(`{"ok":false}`)
		got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "author_decision_required") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("true accepts", func(t *testing.T) {
		project, _, workspace, ready := acceptedFoundationProject(t)
		got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, nil))
		if got.Result != "ACCEPTED" {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
