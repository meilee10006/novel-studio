package core

import "testing"

func TestStateDeltaChangesMustBeExplicitArray(t *testing.T) {
	tests := []struct {
		name       string
		delta      string
		wantResult string
		wantText   string
	}{
		{"missing changes", `{}`, "REWRITE", "state_delta.json.changes"},
		{"not array", `{"changes":"none"}`, "REWRITE", "state_delta.json.changes"},
		{"empty array", `{"changes":[]}`, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			artifacts["state_delta.json"] = []byte(tc.delta)
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
