package core

import "testing"

func TestFoundationEntityCollectionsMustBeArrays(t *testing.T) {
	t.Run("characters not array", func(t *testing.T) {
		project, _, workspace := newCapabilityPassedProject(t)
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		ready := readReady(t, workspace)
		artifacts := validFoundationArtifacts()
		artifacts["characters.json"] = []byte(`{"characters":"hero"}`)
		got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "characters must be an array") {
			t.Fatalf("settlement=%+v", got)
		}
	})

	t.Run("world entities not array", func(t *testing.T) {
		project, _, workspace := newCapabilityPassedProject(t)
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		ready := readReady(t, workspace)
		artifacts := validFoundationArtifacts()
		artifacts["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"same"},"opening_location":{"entity_type":"location","local_ref":"same"}}`)
		artifacts["world.json"] = []byte(`{"entities":"location"}`)
		got, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
		if err != nil {
			t.Fatal(err)
		}
		if got.Result != "REWRITE" || !containsViolation(got.Violations, "world.entities must be an array") {
			t.Fatalf("settlement=%+v", got)
		}
	})
}
