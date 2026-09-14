package core

import "testing"

func TestRelationshipTagsMustBeUniqueNonEmptyStringArrayWhenPresent(t *testing.T) {
	tests := []struct {
		name       string
		tags       any
		wantResult string
		wantText   string
	}{
		{"string", "trust", "REWRITE", "tags"},
		{"mixed", []any{"trust", 1}, "REWRITE", "tags"},
		{"blank", []any{"trust", ""}, "REWRITE", "tags"},
		{"duplicate", []any{"trust", "trust"}, "REWRITE", "tags"},
		{"valid", []any{"trust", "debts"}, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
			ready := readReady(t, workspace)
			change := map[string]any{
				"kind": "relationship", "relationship_id": firstID + "|" + secondID,
				"tags": tc.tags, "event_ref": "e1",
			}
			got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{change}))
			if got.Result != tc.wantResult {
				t.Fatalf("settlement=%+v", got)
			}
			if tc.wantText != "" && !containsViolation(got.Violations, tc.wantText) {
				t.Fatalf("violations=%v want substring %q", got.Violations, tc.wantText)
			}
		})
	}
}
