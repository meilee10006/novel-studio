package core

import "testing"

func TestFoundationLocalReferenceMustBeStringWhenPresent(t *testing.T) {
	tests := []struct {
		name       string
		foundation string
		wantResult string
		wantText   string
	}{
		{"number", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":123},"opening_location":{"entity_type":"location","local_ref":"same"}}`, "REWRITE", "local_ref"},
		{"object", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":{}},"opening_location":{"entity_type":"location","local_ref":"same"}}`, "REWRITE", "local_ref"},
		{"valid", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":"same"}}`, "ACCEPTED", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace := newCapabilityPassedProject(t)
			if err := project.Reconcile(); err != nil {
				t.Fatal(err)
			}
			ready := readReady(t, workspace)
			artifacts := validFoundationArtifacts()
			artifacts["foundation.json"] = []byte(tc.foundation)
			got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
			if err != nil {
				t.Fatal(err)
			}
			if got.Result != tc.wantResult {
				t.Fatalf("settlement=%+v", got)
			}
			if tc.wantText != "" && !containsViolation(got.Violations, tc.wantText) {
				t.Fatalf("violations=%v want substring %q", got.Violations, tc.wantText)
			}
		})
	}
}
