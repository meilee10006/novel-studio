package core

import (
	"fmt"
	"sort"
)

func canonicalizeChapterConflicts(value any, seq int, violations *[]string) ([]IDMapping, map[string]string, int) {
	root, _ := value.(map[string]any)
	changes, _ := root["changes"].([]any)
	defs := map[string]map[string]any{}
	for _, raw := range changes {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "conflict" {
			continue
		}
		localID := cleanString(change["local_id"])
		if localID == "" {
			continue
		}
		if _, exists := change["conflict_id"]; exists {
			*violations = append(*violations, "new conflict must not predeclare canonical id")
			continue
		}
		participants, participantProblems := stringArray(change["participants"], "conflict participants")
		if len(participantProblems) > 0 {
			*violations = append(*violations, participantProblems...)
			continue
		}
		if cleanString(change["description"]) == "" || len(participants) == 0 || cleanString(change["escalation_condition"]) == "" || cleanString(change["close_condition"]) == "" {
			*violations = append(*violations, "new conflict requires local_id, description, participants, escalation_condition, and close_condition")
			continue
		}
		if _, exists := defs[localID]; exists {
			*violations = append(*violations, "duplicate conflict local_id: "+localID)
			continue
		}
		defs[localID] = change
	}
	localIDs := make([]string, 0, len(defs))
	for localID := range defs {
		localIDs = append(localIDs, localID)
	}
	sort.Strings(localIDs)
	mappings := make([]IDMapping, 0, len(localIDs))
	lookup := make(map[string]string, len(localIDs))
	for _, localID := range localIDs {
		canonID := fmt.Sprintf("conflict-%06d", seq)
		seq++
		lookup[localID] = canonID
		change := defs[localID]
		change["conflict_id"] = canonID
		delete(change, "local_id")
		mappings = append(mappings, IDMapping{EntityType: "conflict", LocalID: localID, CanonID: canonID})
	}
	return mappings, lookup, seq
}
func rewriteChapterLocalConflictRefs(value any, lookup map[string]string) {
	if len(lookup) == 0 {
		return
	}
	root, _ := value.(map[string]any)
	for _, raw := range anySlice(root["changes"]) {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "conflict" {
			continue
		}
		if local := cleanString(change["conflict_id"]); lookup[local] != "" {
			change["conflict_id"] = lookup[local]
		}
	}
}
func collectSubmittedChapterObjectRefs(values map[string]any) map[string][]string {
	refs := map[string][]string{}
	add := func(entityType string, value any) {
		if id := cleanString(value); id != "" {
			refs[entityType] = append(refs[entityType], id)
		}
	}
	if contract, ok := values["chapter_contract.json"].(map[string]any); ok {
		add("character", contract["declared_pov"])
	}
	if eventsRoot, ok := values["events.json"].(map[string]any); ok {
		for _, raw := range anySlice(eventsRoot["events"]) {
			item, _ := raw.(map[string]any)
			for _, field := range []string{"actors", "observers"} {
				for _, value := range anySlice(item[field]) {
					add("character", value)
				}
			}
		}
	}
	root, _ := values["state_delta.json"].(map[string]any)
	for _, raw := range anySlice(root["changes"]) {
		change, _ := raw.(map[string]any)
		switch cleanString(change["kind"]) {
		case "knowledge_add":
			add("character", change["character_id"])
			if source, ok := change["source"].(map[string]any); ok {
				add("character", source["from_character_id"])
			}
		case "location":
			add("character", change["character_id"])
			add("location", change["location_id"])
		case "resource":
			add("resource", change["resource_id"])
		case "relationship":
			if left, right, ok := relationshipCharacterIDs(cleanString(change["relationship_id"])); ok {
				add("character", left)
				add("character", right)
			}
		case "foreshadow":
			add("foreshadow", change["foreshadow_id"])
		case "reader_promise":
			add("reader_promise", change["promise_id"])
		case "conflict":
			add("conflict", change["conflict_id"])
			for _, participant := range anySlice(change["participants"]) {
				add("character", participant)
			}
		}
	}
	return refs
}
func rejectCurrentAttemptCanonicalObjectRefs(refs map[string][]string, mappings []IDMapping, violations *[]string) {
	current := map[string]map[string]bool{}
	for _, mapping := range mappings {
		if mapping.CanonID == "" || mapping.EntityType == "story_event" {
			continue
		}
		if current[mapping.EntityType] == nil {
			current[mapping.EntityType] = map[string]bool{}
		}
		current[mapping.EntityType][mapping.CanonID] = true
	}
	for entityType, ids := range refs {
		for _, id := range ids {
			if current[entityType][id] {
				*violations = append(*violations, "current attempt canonical "+entityType+" id must be referenced by local id: "+id)
			}
		}
	}
}
