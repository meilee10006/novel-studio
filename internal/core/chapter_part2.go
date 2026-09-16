package core

import (
	"encoding/json"
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"strings"
)

func validateAndCanonicalizeChapter(files map[string][]byte, state *domain.CoreProductionState, task *domain.CoreTask) (map[string][]byte, []IDMapping, []string, int, error) {
	if err := exactArtifactSet(files, chapterArtifactNames); err != nil {
		return nil, nil, nil, 0, err
	}
	chapter, err := chapterNumberFromTarget(task.Target)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	body := strings.TrimSpace(string(files["chapter.md"]))
	var violations []string
	if body == "" {
		violations = append(violations, "chapter body is empty")
	}
	values := map[string]any{}
	for _, name := range []string{"chapter_contract.json", "events.json", "state_delta.json", "self_review.json"} {
		var value any
		if err := protocol.DecodeJSON(files[name], &value); err != nil {
			return nil, nil, nil, 0, fmt.Errorf("%s: %w", name, err)
		}
		values[name] = value
	}
	contract, contractOK := values["chapter_contract.json"].(map[string]any)
	knownConstraints := map[string]bool{}
	if !contractOK {
		violations = append(violations, "chapter_contract.json must be an object")
	} else {
		declared, _ := contract["chapter"].(float64)
		if int(declared) != chapter || declared != float64(chapter) {
			violations = append(violations, "chapter_contract.chapter does not match task target")
		}
		pov, _ := contract["declared_pov"].(string)
		if strings.TrimSpace(pov) == "" {
			violations = append(violations, "chapter_contract.declared_pov is required")
		}
		if rawConstraints, exists := contract["hard_constraints"]; exists {
			items, ok := rawConstraints.([]any)
			if !ok {
				violations = append(violations, "chapter_contract.hard_constraints must be an array")
			} else {
				for _, raw := range items {
					item, ok := raw.(map[string]any)
					if !ok {
						violations = append(violations, "chapter_contract.hard_constraints item must be an object")
						continue
					}
					id := cleanString(item["id"])
					if id == "" {
						violations = append(violations, "chapter_contract.hard_constraints id is required")
						continue
					}
					if knownConstraints[id] {
						violations = append(violations, "duplicate hard constraint id: "+id)
						continue
					}
					knownConstraints[id] = true
				}
			}
		}
	}
	eventsRoot, ok := values["events.json"].(map[string]any)
	if !ok {
		violations = append(violations, "events.json must be an object")
		return nil, nil, violations, chapter, nil
	}
	events, ok := eventsRoot["events"].([]any)
	if !ok {
		violations = append(violations, "events.json.events must be an array")
		return nil, nil, violations, chapter, nil
	}
	seen := map[string]bool{}
	for _, item := range events {
		m, ok := item.(map[string]any)
		if !ok {
			violations = append(violations, "story event must be an object")
			continue
		}
		localID, _ := m["local_id"].(string)
		localID = strings.TrimSpace(localID)
		if _, exists := m["canon_id"]; exists {
			violations = append(violations, "story event must not predeclare canonical id")
			continue
		}
		if localID == "" || seen[localID] {
			violations = append(violations, "story event local_id must be present and unique")
			continue
		}
		seen[localID] = true
		kind := cleanString(m["kind"])
		if kind != "onscreen" && kind != "offscreen" {
			violations = append(violations, "story event kind must be onscreen or offscreen")
			continue
		}
		validateOptionalStoryEventUniqueStringArray(m, "actors", &violations)
		validateOptionalStoryEventUniqueStringArray(m, "observers", &violations)
		anchor, _ := m["evidence_anchor"].(string)
		if kind == "offscreen" {
			validateOptionalStoryEventStringArray(m, "constraint_refs", &violations)
			refs := stringSlice(m["constraint_refs"])
			if len(refs) == 0 {
				violations = append(violations, "offscreen story event requires constraint_refs")
			}
			for _, ref := range refs {
				if !knownConstraints[ref] {
					violations = append(violations, "offscreen story event references unknown hard constraint: "+ref)
				}
			}
		} else if strings.TrimSpace(anchor) == "" || !strings.Contains(body, anchor) {
			violations = append(violations, "visible story event evidence anchor is absent from chapter")
		}
	}
	var deltaChanges []any
	if delta, ok := values["state_delta.json"].(map[string]any); !ok {
		violations = append(violations, "state_delta.json must be an object")
	} else if changes, ok := delta["changes"].([]any); !ok {
		violations = append(violations, "state_delta.json.changes must be an array")
	} else {
		deltaChanges = changes
	}
	if contractOK {
		validateExtendedChapterContract(contract, body, state, task, deltaChanges, &violations)
	}
	review, ok := values["self_review.json"].(map[string]any)
	if !ok {
		violations = append(violations, "self_review.json must be an object")
	} else {
		rawOK, exists := review["ok"]
		if !exists {
			violations = append(violations, "self_review.ok is required")
		} else if reviewOK, ok := rawOK.(bool); !ok {
			violations = append(violations, "self_review.ok must be boolean")
		} else if !reviewOK {
			decision, exists := review["author_decision_required"]
			if !exists || decision == nil {
				violations = append(violations, "self_review.ok=false requires author_decision_required")
			}
		} else if _, exists := review["author_decision_required"]; exists {
			violations = append(violations, "self_review.ok=true cannot include author_decision_required")
		}
	}
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}

	submittedObjectRefs := collectSubmittedChapterObjectRefs(values)
	entityMappings, characterLookup, locationLookup, resourceLookup, seq := canonicalizeChapterEntityAdds(values["state_delta.json"], state.NextEntitySeq, &violations)
	rewriteChapterLocalEntityRefs(values, characterLookup, locationLookup, resourceLookup)
	foreshadowMappings, foreshadowLookup, seq := canonicalizeChapterForeshadows(values["state_delta.json"], seq, &violations)
	rewriteChapterLocalForeshadowRefs(values["state_delta.json"], foreshadowLookup)
	promiseMappings, promiseLookup, seq := canonicalizeChapterReaderPromises(values["state_delta.json"], seq, &violations)
	rewriteChapterLocalReaderPromiseRefs(values["state_delta.json"], promiseLookup)
	conflictMappings, conflictLookup, seq := canonicalizeChapterConflicts(values["state_delta.json"], seq, &violations)
	rewriteChapterLocalConflictRefs(values["state_delta.json"], conflictLookup)
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}
	mappings := append([]IDMapping(nil), entityMappings...)
	mappings = append(mappings, foreshadowMappings...)
	mappings = append(mappings, promiseMappings...)
	mappings = append(mappings, conflictMappings...)
	rejectCurrentAttemptCanonicalObjectRefs(submittedObjectRefs, mappings, &violations)
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}
	lookup := make(map[string]string, len(events))
	currentEventIDs := make(map[string]bool, len(events))
	for _, item := range events {
		m := item.(map[string]any)
		localID := strings.TrimSpace(m["local_id"].(string))
		canonID := fmt.Sprintf("story-event-%06d", seq)
		seq++
		lookup[localID] = canonID
		currentEventIDs[canonID] = true
		mappings = append(mappings, IDMapping{EntityType: "story_event", LocalID: localID, CanonID: canonID})
		m["canon_id"] = canonID
		delete(m, "local_id")
	}
	rejectPredeclaredCurrentEventIDs(values["state_delta.json"], currentEventIDs, &violations)
	rewriteEventRefs(values["state_delta.json"], lookup, &violations)
	if len(violations) > 0 {
		return nil, nil, violations, chapter, nil
	}

	canonical := map[string][]byte{"chapter.md": files["chapter.md"]}
	for _, name := range []string{"chapter_contract.json", "events.json", "state_delta.json", "self_review.json"} {
		data, err := json.Marshal(values[name])
		if err != nil {
			return nil, nil, nil, 0, err
		}
		canonical[name] = data
	}
	return canonical, mappings, nil, chapter, nil
}
