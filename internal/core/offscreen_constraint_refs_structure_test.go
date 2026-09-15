package core

import "testing"

func TestOffscreenConstraintRefsMustBeNonEmptyStringArray(t *testing.T) {
	tests := []struct {
		name       string
		refsJSON   string
		wantResult string
		wantText   string
	}{
		{"string", `"hc-a"`, "REWRITE", "constraint_refs"},
		{"mixed", `["hc-a",1]`, "REWRITE", "constraint_refs"},
		{"blank element", `["hc-a",""]`, "REWRITE", "constraint_refs"},
		{"duplicate valid ref", `["hc-a","hc-a"]`, "ACCEPTED", ""},
		{"valid", `["hc-a"]`, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, nil)
			artifacts["chapter_contract.json"] = []byte(`{"chapter":1,"declared_pov":"character-000001","hard_constraints":[{"id":"hc-a"}]}`)
			artifacts["events.json"] = []byte(`{"events":[{"local_id":"e1","kind":"offscreen","constraint_refs":` + tc.refsJSON + `}]}`)
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
