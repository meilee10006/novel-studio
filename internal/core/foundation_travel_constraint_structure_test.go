package core

import "testing"

func TestFoundationTravelConstraintEndpointsMustReferenceLocations(t *testing.T) {
	tests := []struct {
		name  string
		world string
	}{
		{
			name: "from character",
			world: `{
				"entities":[{"entity_type":"location","local_id":"same","name":"起点"}],
				"travel_constraints":[{
					"from":{"entity_type":"character","local_ref":"same"},
					"to":{"entity_type":"location","local_ref":"same"},
					"min_ticks":1
				}]
			}`,
		},
		{
			name: "to character",
			world: `{
				"entities":[{"entity_type":"location","local_id":"same","name":"起点"}],
				"travel_constraints":[{
					"from":{"entity_type":"location","local_ref":"same"},
					"to":{"entity_type":"character","local_ref":"same"},
					"min_ticks":1
				}]
			}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace := newCapabilityPassedProject(t)
			if err := project.Reconcile(); err != nil {
				t.Fatal(err)
			}
			ready := readReady(t, workspace)
			artifacts := validFoundationArtifacts()
			artifacts["world.json"] = []byte(tc.world)

			got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
			if err != nil {
				t.Fatal(err)
			}
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "travel constraint requires from/to canon ids") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}
