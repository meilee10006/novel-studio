package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndingResolutionRequiresAcceptedEvent(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "ending_resolution",
	}})
	settlement := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settlement.Result != "REWRITE" || !containsViolation(settlement.Violations, "ending") {
		t.Fatalf("settlement=%+v", settlement)
	}
}

func TestChapterRejectsUnknownCanonicalCharacterReferences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events map[string]any
		change map[string]any
	}{
		{
			name: "event observer",
			events: map[string]any{"events": []any{map[string]any{
				"local_id": "e1", "evidence_anchor": "主角看见门口的灯", "observers": []string{"character-999999"},
			}}},
		},
		{
			name: "knowledge character",
			change: map[string]any{
				"kind": "knowledge_add", "character_id": "character-999999", "fact_id": "secret-a",
				"source": map[string]any{"kind": "observed", "event_ref": "e1"},
			},
		},
		{
			name: "location character",
			change: map[string]any{
				"kind": "location", "character_id": "character-999999", "location_id": "location-a", "start_tick": 0, "end_tick": 1, "event_ref": "e1",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			changes := []map[string]any{}
			if tc.change != nil {
				changes = append(changes, tc.change)
			}
			artifacts := longformChapterArtifacts(1, changes)
			if tc.events != nil {
				raw, _ := json.Marshal(tc.events)
				artifacts["events.json"] = raw
			}
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "character") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func TestLocationChangeRejectsUnknownCanonicalLocation(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "location", "character_id": "character-000001", "location_id": "location-999999", "start_tick": 0, "end_tick": 1, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "location") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestLocationChangeRequiresCharacterID(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "location", "location_id": "location-000002", "start_tick": 0, "end_tick": 1, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "character_id") {
		t.Fatalf("settlement=%+v", got)
	}
}

func TestKnowledgeWithoutSourceIsRewrite(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "secret-a",
	}})
	settlement := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settlement.Result != "REWRITE" || !containsViolation(settlement.Violations, "knowledge") {
		t.Fatalf("settlement=%+v", settlement)
	}
}

func TestKnowledgeStatementPersistsAndAppearsInNextContext(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	statement := "主角知道门口的灯后藏着钥匙"
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "knowledge_add", "character_id": "character-000001", "fact_id": "secret-a", "statement": statement,
		"source": map[string]any{"kind": "observed", "event_ref": "e1"},
	}})
	settlement := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settlement.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", settlement)
	}
	state := readCanonState(t, project)
	fact, ok := state.Longform.Knowledge["character-000001"]["secret-a"].(map[string]any)
	if !ok || fact["statement"] != statement {
		t.Fatalf("knowledge fact lost statement: %+v", state.Longform.Knowledge["character-000001"]["secret-a"])
	}
	next := readReady(t, workspace)
	contextPath := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "canon_excerpt.json")
	raw, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), statement) || !strings.Contains(string(raw), "secret-a") {
		t.Fatalf("next context lost readable knowledge fact: %s", raw)
	}
}

func TestResourceCannotGoNegative(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	artifacts := longformChapterArtifacts(1, []map[string]any{{
		"kind": "resource", "resource_id": "cash", "delta": -1, "event_ref": "e1",
	}})
	settlement := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settlement.Result != "REWRITE" || !containsViolation(settlement.Violations, "resource") {
		t.Fatalf("settlement=%+v", settlement)
	}
}

func TestRelationshipRejectsUnknownCanonicalCharacter(t *testing.T) {
	project, workspace, firstID, secondID, _ := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	got := submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{{
		"kind": "relationship", "relationship_id": firstID + "|character-999999", "tags": []string{"distrust"}, "event_ref": "e1",
	}}))
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "relationship") {
		t.Fatalf("unknown relationship endpoint settlement=%+v", got)
	}

	ready = readReady(t, workspace)
	got = submitAndSettleChapter(t, project, workspace, ready, longformChapterArtifacts(1, []map[string]any{{
		"kind": "relationship", "relationship_id": firstID + "|" + secondID, "tags": []string{"distrust"}, "event_ref": "e1",
	}}))
	if got.Result != "ACCEPTED" {
		t.Fatalf("canonical relationship settlement=%+v", got)
	}
}

func projectWithTwoCharacters(t *testing.T) (*Project, string, string, string, string) {
	t.Helper()
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	artifacts["characters.json"] = []byte(`{"characters":[{"local_id":"first","name":"甲"},{"local_id":"second","name":"乙"}]}`)
	artifacts["foundation.json"] = []byte(`{"title":"测试书","protagonist":{"entity_type":"character","local_ref":"first"},"opening_location":{"entity_type":"location","local_ref":"same"}}`)
	result, err := project.SettleFoundation(FoundationSubmission{Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts})
	if err != nil || result.Result != "ACCEPTED" {
		t.Fatalf("foundation=%+v err=%v", result, err)
	}
	firstID := mappingID(result.IDMappings, "character", "first")
	secondID := mappingID(result.IDMappings, "character", "second")
	locationID := mappingID(result.IDMappings, "location", "same")
	if firstID == "" || secondID == "" || locationID == "" {
		t.Fatalf("mappings=%+v", result.IDMappings)
	}
	return project, workspace, firstID, secondID, locationID
}

func TestEvidenceBackedLongformChangesPersist(t *testing.T) {
	project, workspace, characterID, secondID, locationID := projectWithTwoCharacters(t)
	ready := readReady(t, workspace)
	changes := []map[string]any{
		{"kind": "knowledge_add", "character_id": characterID, "fact_id": "secret-a", "source": map[string]any{"kind": "observed", "event_ref": "e1"}},
		{"kind": "resource", "resource_id": "cash", "delta": 3, "event_ref": "e1"},
		{"kind": "location", "character_id": characterID, "location_id": locationID, "start_tick": 0, "end_tick": 10, "event_ref": "e1"},
		{"kind": "relationship", "relationship_id": characterID + "|" + secondID, "tags": []string{"distrust"}, "event_ref": "e1"},
		{"kind": "foreshadow", "foreshadow_id": "f-1", "state": "seeded", "event_ref": "e1"},
		{"kind": "reader_promise", "promise_id": "p-1", "state": "fulfilled", "event_ref": "e1"},
	}
	artifacts := longformChapterArtifacts(1, changes)
	settlement := submitAndSettleChapter(t, project, workspace, ready, artifacts)
	if settlement.Result != "ACCEPTED" {
		t.Fatalf("settlement=%+v", settlement)
	}
	state := readCanonState(t, project)
	if state.Longform.Resources["cash"] != 3 {
		t.Fatalf("resources=%+v", state.Longform.Resources)
	}
	if _, ok := state.Longform.Knowledge[characterID]["secret-a"]; !ok {
		t.Fatalf("knowledge=%+v", state.Longform.Knowledge)
	}
	if state.Longform.ReaderPromises["p-1"].State != "fulfilled" {
		t.Fatalf("promises=%+v", state.Longform.ReaderPromises)
	}
}

func TestOverlappingDifferentLocationsIsRewrite(t *testing.T) {
	project, workspace, characterID, fromID, toID := projectWithTravelConstraint(t, 0)
	ready := readReady(t, workspace)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": fromID, "start_tick": 0, "end_tick": 10, "event_ref": "e1",
	}})
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", got)
	}
	secondReady := readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": toID, "start_tick": 5, "end_tick": 15, "event_ref": "e1",
	}})
	got := submitAndSettleChapter(t, project, workspace, secondReady, second)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "location") {
		t.Fatalf("second=%+v", got)
	}
}
func TestReadableObligationSemanticsPersistAcrossTransitions(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	foreshadowDescription := "红色纸伞与十年前旧案直接相关"
	promiseStatement := "读者期待知道红色纸伞真正主人是谁"
	first := longformChapterArtifacts(1, []map[string]any{
		{"kind": "foreshadow", "foreshadow_id": "fs-umbrella", "state": "seeded", "description": foreshadowDescription, "event_ref": "e1"},
		{"kind": "reader_promise", "promise_id": "promise-umbrella", "state": "advanced", "statement": promiseStatement},
	})
	if got := submitAndSettleChapter(t, project, workspace, ready, first); got.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", got)
	}
	secondReady := readReady(t, workspace)
	second := longformChapterArtifacts(2, []map[string]any{
		{"kind": "foreshadow", "foreshadow_id": "fs-umbrella", "state": "reinforced", "event_ref": "e1"},
		{"kind": "reader_promise", "promise_id": "promise-umbrella", "state": "deferred", "deadline_chapter": 5},
	})
	if got := submitAndSettleChapter(t, project, workspace, secondReady, second); got.Result != "ACCEPTED" {
		t.Fatalf("second=%+v", got)
	}
	next := readReady(t, workspace)
	canonPath := filepath.Join(workspace, "exchange", "outbox", next.TaskID, next.AttemptID, "canon_excerpt.json")
	raw, err := os.ReadFile(canonPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{foreshadowDescription, promiseStatement, "reinforced", "deferred"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("next context lost obligation semantic %q: %s", want, raw)
		}
	}
}

func TestEvidenceRequiredForRelationshipForeshadowAndPromise(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change map[string]any
	}{
		{"relationship", map[string]any{"kind": "relationship", "relationship_id": "a:b", "tags": []string{"trust"}}},
		{"foreshadow", map[string]any{"kind": "foreshadow", "foreshadow_id": "f-1", "state": "seeded"}},
		{"promise", map[string]any{"kind": "reader_promise", "promise_id": "p-1", "state": "fulfilled"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			change := make(map[string]any, len(tc.change))
			for key, value := range tc.change {
				change[key] = value
			}
			var project *Project
			var workspace string
			if tc.name == "relationship" {
				var firstID, secondID string
				project, workspace, firstID, secondID, _ = projectWithTwoCharacters(t)
				change["relationship_id"] = firstID + "|" + secondID
			} else {
				project, _, workspace, _ = acceptedFoundationProject(t)
			}
			ready := readReady(t, workspace)
			artifacts := longformChapterArtifacts(1, []map[string]any{change})
			got := submitAndSettleChapter(t, project, workspace, ready, artifacts)
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "evidence") {
				t.Fatalf("settlement=%+v", got)
			}
		})
	}
}

func longformChapterArtifacts(chapter int, changes []map[string]any) map[string][]byte {
	body := "章节正文。主角看见门口的灯。"
	contract, _ := json.Marshal(map[string]any{"chapter": chapter, "declared_pov": "character-000001"})
	events, _ := json.Marshal(map[string]any{"events": []map[string]any{{
		"local_id": "e1", "evidence_anchor": "主角看见门口的灯", "observers": []string{"character-000001"},
	}}})
	delta, _ := json.Marshal(map[string]any{"changes": changes})
	return map[string][]byte{
		"chapter.md": []byte(body), "chapter_contract.json": contract,
		"events.json": events, "self_review.json": []byte(`{"ok":true}`), "state_delta.json": delta,
	}
}

func submitAndSettleChapter(t *testing.T, project *Project, workspace string, ready readyView, artifacts map[string][]byte) ChapterSettlement {
	t.Helper()
	writeSubmission(t, workspace, ready, artifacts, false)
	project.submissionQuietPeriod = 0
	_, _ = project.ScanActiveSubmission()
	_, _ = project.ScanActiveSubmission()
	settlement, err := project.SettleActiveSnapshot()
	if err != nil {
		t.Fatalf("SettleActiveSnapshot: %v", err)
	}
	return settlement
}
func containsViolation(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func readCanonState(t *testing.T, project *Project) CoreCanonStateView {
	t.Helper()
	raw, err := project.store.ReadCoreCanonStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	var out CoreCanonStateView
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

type CoreCanonStateView struct {
	Longform struct {
		Knowledge      map[string]map[string]any `json:"knowledge"`
		Resources      map[string]int64          `json:"resources"`
		ReaderPromises map[string]struct {
			State string `json:"state"`
		} `json:"reader_promises"`
	} `json:"longform"`
}

func TestModeledTravelConstraintRejectsTooFastMove(t *testing.T) {
	project, workspace, characterID, fromID, toID := projectWithTravelConstraint(t, 10)
	first := readReady(t, workspace)
	got := submitAndSettleChapter(t, project, workspace, first, longformChapterArtifacts(1, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": fromID,
		"start_tick": 0, "end_tick": 10, "event_ref": "e1",
	}}))
	if got.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", got)
	}
	second := readReady(t, workspace)
	got = submitAndSettleChapter(t, project, workspace, second, longformChapterArtifacts(2, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": toID,
		"start_tick": 15, "end_tick": 25, "event_ref": "e1",
	}}))
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "travel") {
		t.Fatalf("second=%+v", got)
	}
}
func TestUnmodeledTravelDoesNotInventDistance(t *testing.T) {
	project, workspace, characterID, fromID, toID := projectWithTravelConstraint(t, 0)
	first := readReady(t, workspace)
	got := submitAndSettleChapter(t, project, workspace, first, longformChapterArtifacts(1, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": fromID,
		"start_tick": 0, "end_tick": 10, "event_ref": "e1",
	}}))
	if got.Result != "ACCEPTED" {
		t.Fatalf("first=%+v", got)
	}
	second := readReady(t, workspace)
	got = submitAndSettleChapter(t, project, workspace, second, longformChapterArtifacts(2, []map[string]any{{
		"kind": "location", "character_id": characterID, "location_id": toID,
		"start_tick": 10, "end_tick": 20, "event_ref": "e1",
	}}))
	if got.Result != "ACCEPTED" {
		t.Fatalf("unmodeled travel should be accepted: %+v", got)
	}
}
func projectWithTravelConstraint(t *testing.T, minTicks int64) (*Project, string, string, string, string) {
	t.Helper()
	project, _, workspace := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := readReady(t, workspace)
	artifacts := validFoundationArtifacts()
	world := map[string]any{
		"entities": []map[string]any{
			{"entity_type": "location", "local_id": "same", "name": "起点"},
			{"entity_type": "location", "local_id": "from", "name": "甲地"},
			{"entity_type": "location", "local_id": "to", "name": "乙地"},
		},
	}
	if minTicks > 0 {
		world["travel_constraints"] = []map[string]any{{
			"from":      map[string]any{"entity_type": "location", "local_ref": "from"},
			"to":        map[string]any{"entity_type": "location", "local_ref": "to"},
			"min_ticks": minTicks,
		}}
	}
	raw, err := json.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	artifacts["world.json"] = raw
	result, err := project.SettleFoundation(FoundationSubmission{
		Manifest: manifestForReady(ready, artifacts), Artifacts: artifacts,
	})
	if err != nil || result.Result != "ACCEPTED" {
		t.Fatalf("foundation=%+v err=%v", result, err)
	}
	characterID := mappingID(result.IDMappings, "character", "same")
	fromID := mappingID(result.IDMappings, "location", "from")
	toID := mappingID(result.IDMappings, "location", "to")
	if characterID == "" || fromID == "" || toID == "" {
		t.Fatalf("mappings=%+v", result.IDMappings)
	}
	return project, workspace, characterID, fromID, toID
}
