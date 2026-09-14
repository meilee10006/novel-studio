package core

import "testing"

func TestChapterHardConstraintIDsAreStructuredAndUnique(t *testing.T) {
	tests := []struct {
		name       string
		contract   string
		wantResult string
		wantText   string
	}{
		{"not array", `{"chapter":1,"declared_pov":"character-000001","hard_constraints":"hc-a"}`, "REWRITE", "hard_constraints"},
		{"missing id", `{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{}]}`, "REWRITE", "id"},
		{"duplicate id", `{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-a"},{"id":"hc-a"}]}`, "REWRITE", "duplicate"},
		{"valid", `{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-a"},{"id":"hc-b"}]}`, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			artifacts["chapter_contract.json"] = []byte(tc.contract)
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
