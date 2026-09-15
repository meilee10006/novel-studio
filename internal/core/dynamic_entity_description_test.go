package core

import "testing"

func TestDynamicEntityDescriptionMustBeStringWhenPresent(t *testing.T) {
	tests := []struct {
		name   string
		change map[string]any
	}{
		{"character", map[string]any{"kind": "character_add", "local_id": "ally", "name": "新同伴", "description": 123, "event_ref": "e1"}},
		{"location", map[string]any{"kind": "location_add", "local_id": "station", "name": "旧车站", "description": map[string]any{"text": "非法对象"}, "event_ref": "e1"}},
		{"resource", map[string]any{"kind": "resource_add", "local_id": "cash", "name": "现金", "description": []any{"非法数组"}, "event_ref": "e1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			artifacts := longformChapterArtifacts(1, []map[string]any{tc.change})
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "description") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
