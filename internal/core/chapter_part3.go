package core

import (
	"fmt"
	"sort"
	"strings"
)

func canonicalizeChapterEntityAdds(value any, seq int, violations *[]string) ([]IDMapping, map[string]string, map[string]string, map[string]string, int) {
	root, _ := value.(map[string]any)
	changes, _ := root["changes"].([]any)
	defs := map[string]map[string]any{}
	types := map[string]string{}
	for _, raw := range changes {
		change, _ := raw.(map[string]any)
		kind := cleanString(change["kind"])
		entityType := ""
		switch kind {
		case "character_add":
			entityType = "character"
		case "location_add":
			entityType = "location"
		case "resource_add":
			entityType = "resource"
		default:
			continue
		}
		localID := cleanString(change["local_id"])
		name := cleanString(change["name"])
		if localID == "" || name == "" {
			*violations = append(*violations, kind+" requires local_id and name")
			continue
		}
		if rawDescription, exists := change["description"]; exists {
			if _, ok := rawDescription.(string); !ok {
				*violations = append(*violations, kind+" description must be a string")
				continue
			}
		}
		if _, exists := change["canon_id"]; exists {
			*violations = append(*violations, kind+" must not predeclare canonical id")
			continue
		}
		key := entityType + "\x00" + localID
		if _, exists := defs[key]; exists {
			*violations = append(*violations, "duplicate "+kind+" local_id: "+localID)
			continue
		}
		defs[key] = change
		types[key] = entityType
	}
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mappings := make([]IDMapping, 0, len(keys))
	characters := map[string]string{}
	locations := map[string]string{}
	resources := map[string]string{}
	for _, key := range keys {
		entityType := types[key]
		localID := strings.TrimPrefix(key, entityType+"\x00")
		canonID := fmt.Sprintf("%s-%06d", entityType, seq)
		seq++
		change := defs[key]
		change["canon_id"] = canonID
		delete(change, "local_id")
		mappings = append(mappings, IDMapping{EntityType: entityType, LocalID: localID, CanonID: canonID})
		switch entityType {
		case "character":
			characters[localID] = canonID
		case "location":
			locations[localID] = canonID
		case "resource":
			resources[localID] = canonID
		}
	}
	return mappings, characters, locations, resources, seq
}
func canonicalizeChapterForeshadows(value any, seq int, violations *[]string) ([]IDMapping, map[string]string, int) {
	root, _ := value.(map[string]any)
	changes, _ := root["changes"].([]any)
	defs := map[string]map[string]any{}
	for _, raw := range changes {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "foreshadow" {
			continue
		}
		localID := cleanString(change["local_id"])
		if localID == "" {
			continue
		}
		if _, exists := change["foreshadow_id"]; exists {
			*violations = append(*violations, "new foreshadow must not predeclare canonical id")
			continue
		}
		if cleanString(change["description"]) == "" {
			*violations = append(*violations, "new foreshadow requires local_id and description")
			continue
		}
		if _, exists := defs[localID]; exists {
			*violations = append(*violations, "duplicate foreshadow local_id: "+localID)
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
		canonID := fmt.Sprintf("foreshadow-%06d", seq)
		seq++
		lookup[localID] = canonID
		change := defs[localID]
		change["foreshadow_id"] = canonID
		delete(change, "local_id")
		mappings = append(mappings, IDMapping{EntityType: "foreshadow", LocalID: localID, CanonID: canonID})
	}
	return mappings, lookup, seq
}
func rewriteChapterLocalForeshadowRefs(value any, lookup map[string]string) {
	if len(lookup) == 0 {
		return
	}
	root, _ := value.(map[string]any)
	for _, raw := range anySlice(root["changes"]) {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "foreshadow" {
			continue
		}
		if local := cleanString(change["foreshadow_id"]); lookup[local] != "" {
			change["foreshadow_id"] = lookup[local]
		}
	}
}
func canonicalizeChapterReaderPromises(value any, seq int, violations *[]string) ([]IDMapping, map[string]string, int) {
	root, _ := value.(map[string]any)
	changes, _ := root["changes"].([]any)
	defs := map[string]map[string]any{}
	for _, raw := range changes {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "reader_promise" {
			continue
		}
		localID := cleanString(change["local_id"])
		if localID == "" {
			continue
		}
		if _, exists := change["promise_id"]; exists {
			*violations = append(*violations, "new reader promise must not predeclare canonical id")
			continue
		}
		if cleanString(change["statement"]) == "" {
			*violations = append(*violations, "new reader promise requires local_id and statement")
			continue
		}
		if _, exists := defs[localID]; exists {
			*violations = append(*violations, "duplicate reader promise local_id: "+localID)
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
		canonID := fmt.Sprintf("reader-promise-%06d", seq)
		seq++
		lookup[localID] = canonID
		change := defs[localID]
		change["promise_id"] = canonID
		delete(change, "local_id")
		mappings = append(mappings, IDMapping{EntityType: "reader_promise", LocalID: localID, CanonID: canonID})
	}
	return mappings, lookup, seq
}
func rewriteChapterLocalReaderPromiseRefs(value any, lookup map[string]string) {
	if len(lookup) == 0 {
		return
	}
	root, _ := value.(map[string]any)
	for _, raw := range anySlice(root["changes"]) {
		change, _ := raw.(map[string]any)
		if cleanString(change["kind"]) != "reader_promise" {
			continue
		}
		if local := cleanString(change["promise_id"]); lookup[local] != "" {
			change["promise_id"] = lookup[local]
		}
	}
}
