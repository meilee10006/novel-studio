package core

import "testing"

func TestFoundationRequiresProtagonistAndOpeningLocationRefs(t *testing.T) {
	tests := []struct {
		name       string
		foundation string
		want       string
	}{
		{"missing protagonist", `{"title":"测试书","opening_location":{"entity_type":"location","local_ref":"same"}}`, "protagonist"},
		{"protagonist not object", `{"title":"测试书","protagonist":"same","opening_location":{"entity_type":"location","local_ref":"same"}}`, "protagonist"},
		{"protagonist wrong type", `{"title":"测试书","protagonist":{"entity_type":"location","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":"same"}}`, "protagonist"},
		{"protagonist missing local ref", `{"title":"测试书","protagonist":{"entity_type":"character"},"opening_location":{"entity_type":"location","local_ref":"same"}}`, "protagonist"},
		{"missing opening location", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"}}`, "opening_location"},
		{"opening location wrong type", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"character","local_ref":"same"}}`, "opening_location"},
		{"opening location missing local ref", `{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location"}}`, "opening_location"},
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
			if got.Result != "REWRITE" || !containsViolation(got.Violations, tc.want) {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
