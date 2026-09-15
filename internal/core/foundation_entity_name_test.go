package core

import "testing"

func TestFoundationEntitiesRequireReadableNames(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string][]byte)
	}{
		{"character missing name", func(a map[string][]byte) {
			a["characters.json"] = []byte(`{"characters":[{"local_id":"same"}]}`)
		}},
		{"character name wrong type", func(a map[string][]byte) {
			a["characters.json"] = []byte(`{"characters":[{"local_id":"same","name":123}]}`)
		}},
		{"world entity missing name", func(a map[string][]byte) {
			a["world.json"] = []byte(`{"entities":[{"entity_type":"location","local_id":"same"}]}`)
		}},
		{"world entity name wrong type", func(a map[string][]byte) {
			a["world.json"] = []byte(`{"entities":[{"entity_type":"location","local_id":"same","name":{"bad":true}}]}`)
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
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "name") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
