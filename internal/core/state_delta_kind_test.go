package core

import "testing"

func TestStateDeltaChangeKindMustBePresentAndSupported(t *testing.T) {
	tests := []struct {
		name       string
		change     map[string]any
		wantResult string
		wantText   string
	}{
		{"missing", map[string]any{"event_ref": "e1"}, "REWRITE", "kind"},
		{"unknown", map[string]any{"kind": "teleport", "event_ref": "e1"}, "REWRITE", "unsupported"},
		{"supported", map[string]any{"kind": "ending_resolution", "event_ref": "e1"}, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, []map[string]any{tc.change})
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
