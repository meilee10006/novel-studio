package core

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func (p *Project) validateExtendedChapterContractSemantics(canonical map[string][]byte, canon domain.CoreCanonState, task *domain.CoreTask) ([]string, error) {
	var contract map[string]any
	if err := protocol.DecodeJSON(canonical["chapter_contract.json"], &contract); err != nil {
		return nil, err
	}
	var delta map[string]any
	if err := protocol.DecodeJSON(canonical["state_delta.json"], &delta); err != nil {
		return nil, err
	}
	changes, _ := delta["changes"].([]any)
	var violations []string

	if raw, exists := contract["target_length"]; exists {
		target, _ := raw.(map[string]any)
		minChars, _ := integer64(target["min_chars"])
		maxChars, _ := integer64(target["max_chars"])
		bodyChars := int64(utf8.RuneCountInString(strings.TrimSpace(string(canonical["chapter.md"]))))
		if bodyChars < minChars || bodyChars > maxChars {
			violations = append(violations, "chapter_contract.target_length does not include chapter.md rune length")
		}
	}

	actualKinds := map[string]bool{}
	foreshadowStates := map[string][]string{}
	for _, raw := range changes {
		change, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind := cleanString(change["kind"])
		if kind != "" {
			actualKinds[kind] = true
		}
		if kind == "foreshadow" {
			id := cleanString(change["foreshadow_id"])
			state := cleanString(change["state"])
			if id != "" && state != "" {
				foreshadowStates[id] = append(foreshadowStates[id], state)
			}
		}
	}
	if raw, exists := contract["expected_changes"]; exists {
		items, _ := stringArray(raw, "chapter_contract.expected_changes")
		for _, kind := range items {
			if !actualKinds[kind] {
				violations = append(violations, "chapter_contract.expected_changes missing submitted state change kind: "+kind)
			}
		}
	}

	knownRefs, err := p.knownChapterContractRefs(canon.Longform)
	if err != nil {
		return nil, err
	}
	for _, field := range []string{"obligation_refs", "immutable_refs"} {
		if raw, exists := contract[field]; exists {
			items, _ := stringArray(raw, "chapter_contract."+field)
			for _, ref := range items {
				if !knownRefs[ref] {
					violations = append(violations, "chapter_contract."+field+" references unknown canonical object: "+ref)
				}
			}
		}
	}

	if raw, exists := contract["foreshadow_operations"]; exists {
		items, _ := raw.([]any)
		for _, rawItem := range items {
			item, _ := rawItem.(map[string]any)
			id := cleanString(item["foreshadow_id"])
			if _, exists := canon.Longform.Foreshadows[id]; !exists {
				violations = append(violations, "chapter_contract.foreshadow_operations references unknown canonical foreshadow: "+id)
				continue
			}
			allowedItems, _ := stringArray(item["allowed_states"], "chapter_contract.foreshadow_operations allowed_states")
			allowed := map[string]bool{}
			for _, state := range allowedItems {
				allowed[state] = true
			}
			for _, next := range foreshadowStates[id] {
				if !allowed[next] {
					violations = append(violations, "chapter_contract.foreshadow_operations allowed_states does not permit submitted state: "+next)
				}
			}
		}
	}

	issuedConstraints := map[string]bool{}
	for _, constraint := range task.Constraints {
		issuedConstraints[constraint.Kind] = true
	}
	if raw, exists := contract["planning_obligations"]; exists {
		items, _ := stringArray(raw, "chapter_contract.planning_obligations")
		for _, obligation := range items {
			if !isRollingPlanningObligation(obligation) || !issuedConstraints[obligation] {
				violations = append(violations, "chapter_contract.planning_obligations was not issued by the active task: "+obligation)
			}
		}
	}

	sort.Strings(violations)
	return violations, nil
}

func (p *Project) knownChapterContractRefs(longform domain.CoreLongformState) (map[string]bool, error) {
	known := map[string]bool{}
	characters, err := p.canonicalCharacterIDs()
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
	for id := range characters {
		known[id] = true
	}
	for id := range locations {
		known[id] = true
	}
	for id := range resources {
		known[id] = true
	}
	for id := range longform.Entities {
		known[id] = true
	}
	for id := range longform.Events {
		known[id] = true
	}
	for characterID, facts := range longform.Knowledge {
		known[characterID] = true
		for factID := range facts {
			known[factID] = true
		}
	}
	for characterID, location := range longform.Locations {
		known[characterID] = true
		known[location.LocationID] = true
	}
	for id := range longform.Resources {
		known[id] = true
	}
	for id := range longform.Relationships {
		known[id] = true
	}
	for id := range longform.Foreshadows {
		known[id] = true
	}
	for id := range longform.Conflicts {
		known[id] = true
	}
	for id := range longform.ReaderPromises {
		known[id] = true
	}
	return known, nil
}

func isRollingPlanningObligation(kind string) bool {
	return kind == "rolling_planning_due" || kind == "planning_repair_required"
}
