package core

import (
	"encoding/json"
	"testing"
)

func TestForeshadowDescriptionIsStableAcrossTransitions(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "foreshadow", "local_id": "umbrella",
		"description": "红色纸伞与十年前旧案直接相关", "state": "seeded", "event_ref": "e1",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create=%+v", settled)
	}
	foreshadowID := mappingID(settled.IDMappings, "foreshadow", "umbrella")
	if foreshadowID == "" {
		t.Fatalf("mapping missing: %+v", settled.IDMappings)
	}

	badReady := readReady(t, workspace)
	bad := longformChapterArtifacts(2, []map[string]any{{
		"kind": "foreshadow", "foreshadow_id": foreshadowID,
		"description": "红色纸伞其实只是无关道具", "state": "reinforced", "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, badReady, bad)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "description") {
		t.Fatalf("changed description=%+v", got)
	}

	retry := readReady(t, workspace)
	good := longformChapterArtifacts(2, []map[string]any{{
		"kind": "foreshadow", "foreshadow_id": foreshadowID,
		"state": "reinforced", "event_ref": "e1",
	}})
	got = submitAndSettleChapter(t, project, workspace, retry, good)
	if got.Result != "ACCEPTED" {
		t.Fatalf("omitted description=%+v", got)
	}
	raw, err := project.store.ReadCoreCanonStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	var canon struct {
		Longform struct {
			Foreshadows map[string]struct {
				Description string `json:"description"`
			} `json:"foreshadows"`
		} `json:"longform"`
	}
	if err := json.Unmarshal(raw, &canon); err != nil {
		t.Fatal(err)
	}
	if got := canon.Longform.Foreshadows[foreshadowID].Description; got != "红色纸伞与十年前旧案直接相关" {
		t.Fatalf("description=%q", got)
	}
}
