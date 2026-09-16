package core

import (
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"sort"
)

func (p *Project) validateCanonicalEntityReferences(canonical map[string][]byte, longform domain.CoreLongformState, mappings []IDMapping) ([]string, error) {
	ids, err := p.canonicalCharacterIDs()
	if err != nil {
		return nil, err
	}
	locations, err := p.canonicalLocationIDs()
	if err != nil {
		return nil, err
	}
	resources, err := p.canonicalResourceIDs()
	if err != nil {
		return nil, err
	}
	for id := range longform.Resources {
		resources[id] = true
	}
	events := map[string]bool{}
	for id := range longform.Events {
		events[id] = true
	}
	foreshadows := map[string]bool{}
	for id := range longform.Foreshadows {
		foreshadows[id] = true
	}
	for _, mapping := range mappings {
		if mapping.EntityType == "foreshadow" && mapping.CanonID != "" {
			foreshadows[mapping.CanonID] = true
		}
	}
	readerPromises := map[string]bool{}
	for id := range longform.ReaderPromises {
		readerPromises[id] = true
	}
	conflicts := map[string]bool{}
	for id := range longform.Conflicts {
		conflicts[id] = true
	}
	for _, mapping := range mappings {
		if mapping.EntityType == "reader_promise" && mapping.CanonID != "" {
			readerPromises[mapping.CanonID] = true
		}
		if mapping.EntityType == "conflict" && mapping.CanonID != "" {
			conflicts[mapping.CanonID] = true
		}
		if mapping.EntityType == "story_event" && mapping.CanonID != "" {
			events[mapping.CanonID] = true
		}
	}
	for id, entity := range longform.Entities {
		switch entity.EntityType {
		case "character":
			ids[id] = true
		case "location":
			locations[id] = true
		case "resource":
			resources[id] = true
		}
	}
	var delta map[string]any
	if err := protocol.DecodeJSON(canonical["state_delta.json"], &delta); err != nil {
		return nil, err
	}
	for _, raw := range anySlice(delta["changes"]) {
		change, _ := raw.(map[string]any)
		switch cleanString(change["kind"]) {
		case "character_add":
			if id := cleanString(change["canon_id"]); id != "" {
				ids[id] = true
			}
		case "location_add":
			if id := cleanString(change["canon_id"]); id != "" {
				locations[id] = true
			}
		case "resource_add":
			if id := cleanString(change["canon_id"]); id != "" {
				resources[id] = true
			}
		}
	}
	var violations []string
	validateCanonicalEventReferences(delta["changes"], events, &violations)
	var contract map[string]any
	if err := protocol.DecodeJSON(canonical["chapter_contract.json"], &contract); err != nil {
		return nil, err
	}
	if pov := cleanString(contract["declared_pov"]); !ids[pov] {
		violations = append(violations, "chapter_contract.declared_pov is not a canonical character")
	}

	var eventsRoot map[string]any
	if err := protocol.DecodeJSON(canonical["events.json"], &eventsRoot); err != nil {
		return nil, err
	}
	for _, raw := range anySlice(eventsRoot["events"]) {
		item, _ := raw.(map[string]any)
		for _, field := range []string{"actors", "observers"} {
			for _, id := range stringSlice(item[field]) {
				if !ids[id] {
					violations = append(violations, "story event "+field+" references unknown canonical character: "+id)
				}
			}
		}
	}

	for _, raw := range anySlice(delta["changes"]) {
		change, _ := raw.(map[string]any)
		kind := cleanString(change["kind"])
		characterID := cleanString(change["character_id"])
		if kind == "location" && characterID == "" {
			violations = append(violations, "location change requires character_id")
		} else if characterID != "" && !ids[characterID] {
			violations = append(violations, kind+" change references unknown canonical character: "+characterID)
		}
		if kind == "location" {
			locationID := cleanString(change["location_id"])
			if locationID == "" {
				violations = append(violations, "location change requires location_id")
			} else if !locations[locationID] {
				violations = append(violations, "location change references unknown canonical location: "+locationID)
			}
		}
		if kind == "resource" {
			resourceID := cleanString(change["resource_id"])
			if resourceID == "" {
				violations = append(violations, "resource change requires resource_id")
			} else if !resources[resourceID] {
				violations = append(violations, "resource change references unknown canonical resource: "+resourceID)
			}
		}
		if kind == "foreshadow" {
			foreshadowID := cleanString(change["foreshadow_id"])
			if foreshadowID != "" && !foreshadows[foreshadowID] {
				violations = append(violations, "foreshadow change references unknown canonical foreshadow: "+foreshadowID)
			}
		}
		if kind == "reader_promise" {
			promiseID := cleanString(change["promise_id"])
			if promiseID != "" && !readerPromises[promiseID] {
				violations = append(violations, "reader promise change references unknown canonical promise: "+promiseID)
			}
		}
		if kind == "conflict" {
			conflictID := cleanString(change["conflict_id"])
			if conflictID != "" && !conflicts[conflictID] {
				violations = append(violations, "conflict change references unknown canonical conflict: "+conflictID)
			}
			var participants []string
			if rawParticipants, exists := change["participants"]; exists {
				var participantProblems []string
				participants, participantProblems = stringArray(rawParticipants, "conflict participants")
				violations = append(violations, participantProblems...)
			}
			for _, participant := range participants {
				if !ids[participant] {
					violations = append(violations, "conflict participant references unknown canonical character: "+participant)
				}
			}
		}
		if kind == "relationship" {
			if rawTags, exists := change["tags"]; exists {
				_, tagProblems := stringArray(rawTags, "relationship tags")
				violations = append(violations, tagProblems...)
			}
			left, right, ok := relationshipCharacterIDs(cleanString(change["relationship_id"]))
			if !ok {
				violations = append(violations, "relationship change requires two canonical character ids")
			} else {
				if left == right {
					violations = append(violations, "relationship change requires two different canonical characters")
				}
				if !ids[left] {
					violations = append(violations, "relationship change references unknown canonical character: "+left)
				}
				if !ids[right] {
					violations = append(violations, "relationship change references unknown canonical character: "+right)
				}
			}
		}
		if source, ok := change["source"].(map[string]any); ok {
			if from := cleanString(source["from_character_id"]); from != "" && !ids[from] {
				violations = append(violations, "knowledge source references unknown canonical character: "+from)
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}
