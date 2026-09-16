package core

import (
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"strings"
)

func rewriteChapterLocalEntityRefs(values map[string]any, characters, locations, resources map[string]string) {
	if len(characters) > 0 {
		if contract, ok := values["chapter_contract.json"].(map[string]any); ok {
			if local := cleanString(contract["declared_pov"]); characters[local] != "" {
				contract["declared_pov"] = characters[local]
			}
		}
		if eventsRoot, ok := values["events.json"].(map[string]any); ok {
			for _, raw := range anySlice(eventsRoot["events"]) {
				item, _ := raw.(map[string]any)
				for _, field := range []string{"actors", "observers"} {
					items, _ := item[field].([]any)
					for i, value := range items {
						local := cleanString(value)
						if canonID := characters[local]; canonID != "" {
							items[i] = canonID
						}
					}
				}
			}
		}
	}
	root, _ := values["state_delta.json"].(map[string]any)
	for _, raw := range anySlice(root["changes"]) {
		change, _ := raw.(map[string]any)
		if local := cleanString(change["character_id"]); characters[local] != "" {
			change["character_id"] = characters[local]
		}
		if local := cleanString(change["location_id"]); locations[local] != "" {
			change["location_id"] = locations[local]
		}
		if local := cleanString(change["resource_id"]); resources[local] != "" {
			change["resource_id"] = resources[local]
		}
		if cleanString(change["kind"]) == "relationship" {
			left, right, ok := relationshipCharacterIDs(cleanString(change["relationship_id"]))
			if ok {
				if characters[left] != "" {
					left = characters[left]
				}
				if characters[right] != "" {
					right = characters[right]
				}
				change["relationship_id"] = left + "|" + right
			}
		}
		if source, ok := change["source"].(map[string]any); ok {
			if local := cleanString(source["from_character_id"]); characters[local] != "" {
				source["from_character_id"] = characters[local]
			}
		}
	}
}
func rejectPredeclaredCurrentEventIDs(value any, current map[string]bool, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		if raw, exists := x["event_canon_id"]; exists {
			if id := cleanString(raw); id != "" && current[id] {
				*violations = append(*violations, "current attempt story event must be referenced by event_ref, not a predeclared canonical event id: "+id)
			}
		}
		for _, child := range x {
			rejectPredeclaredCurrentEventIDs(child, current, violations)
		}
	case []any:
		for _, child := range x {
			rejectPredeclaredCurrentEventIDs(child, current, violations)
		}
	}
}
func rewriteEventRefs(value any, lookup map[string]string, violations *[]string) {
	switch x := value.(type) {
	case map[string]any:
		if rawRef, exists := x["event_ref"]; exists {
			local, ok := rawRef.(string)
			local = strings.TrimSpace(local)
			if !ok || local == "" {
				*violations = append(*violations, "event_ref must be a non-empty string")
				return
			}
			if _, canonExists := x["event_canon_id"]; canonExists {
				*violations = append(*violations, "current event_ref must not include a predeclared canonical event id")
				return
			}
			if canonID := lookup[local]; canonID != "" {
				x["event_canon_id"] = canonID
				delete(x, "event_ref")
			} else {
				*violations = append(*violations, "state delta references unknown story event: "+local)
			}
		} else if rawCanonID, exists := x["event_canon_id"]; exists {
			canonID, ok := rawCanonID.(string)
			if !ok || strings.TrimSpace(canonID) == "" {
				*violations = append(*violations, "event_canon_id must be a non-empty string")
				return
			}
		}
		for _, child := range x {
			rewriteEventRefs(child, lookup, violations)
		}
	case []any:
		for _, child := range x {
			rewriteEventRefs(child, lookup, violations)
		}
	}
}
func (p *Project) canonicalCharacterIDs() (map[string]bool, error) {
	charactersRaw, err := p.store.ReadCoreCanonArtifact("characters.json")
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := protocol.DecodeJSON(charactersRaw, &root); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	items, _ := root["characters"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if id := cleanString(item["canon_id"]); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
func (p *Project) canonicalLocationIDs() (map[string]bool, error) {
	worldRaw, err := p.store.ReadCoreCanonArtifact("world.json")
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := protocol.DecodeJSON(worldRaw, &root); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, raw := range anySlice(root["entities"]) {
		item, _ := raw.(map[string]any)
		if cleanString(item["entity_type"]) != "location" {
			continue
		}
		if id := cleanString(item["canon_id"]); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
func (p *Project) canonicalResourceIDs() (map[string]bool, error) {
	worldRaw, err := p.store.ReadCoreCanonArtifact("world.json")
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := protocol.DecodeJSON(worldRaw, &root); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, raw := range anySlice(root["entities"]) {
		item, _ := raw.(map[string]any)
		if cleanString(item["entity_type"]) != "resource" {
			continue
		}
		if id := cleanString(item["canon_id"]); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}
