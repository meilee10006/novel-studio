package core

import "testing"

func TestFoundationCannotPredeclareCanonicalIDs(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string][]byte)
	}{
		{"character definition", func(a map[string][]byte) {
			a["characters.json"] = []byte(`{"characters":[{"local_id":"same","name":"主角","canon_id":"character-999999"}]}`)
		}},
		{"world definition", func(a map[string][]byte) {
			a["world.json"] = []byte(`{"entities":[{"entity_type":"location","local_id":"same","name":"起点","canon_id":"location-999999"}]}`)
		}},
		{"protagonist reference", func(a map[string][]byte) {
			a["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same","canon_id":"character-999999"},"opening_location":{"entity_type":"location","local_ref":"same"}}`)
		}},
		{"opening location reference", func(a map[string][]byte) {
			a["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":"same","canon_id":"location-999999"}}`)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace := newCapabilityPassedProject(t)
			if err := project.Reconcile(); err != nil {
				t.Fatal(err)
			}
			ready := readReady(t, workspace)
			artifacts := validFoundationArtifacts()
			tc.edit(artifacts)
			got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
			if err != nil {
				t.Fatal(err)
			}
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "canonical") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
