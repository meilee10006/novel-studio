package core

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func validateArcRehearsal(raw, planningPatch []byte) ([]byte, []string, error) {
	var rehearsal map[string]any
	if err := protocol.DecodeJSON(raw, &rehearsal); err != nil {
		return nil, nil, fmt.Errorf("arc_rehearsal.json: %w", err)
	}
	var patch map[string]any
	if err := protocol.DecodeJSON(planningPatch, &patch); err != nil {
		return nil, []string{"arc_rehearsal cannot bind invalid planning_patch.json"}, nil
	}

	var violations []string
	if cleanString(rehearsal["subject_ref"]) != "planning_patch.json" {
		violations = append(violations, "arc_rehearsal.subject_ref must be planning_patch.json")
	}
	if cleanString(rehearsal["selection_reason"]) == "" {
		violations = append(violations, "arc_rehearsal.selection_reason is required")
	}

	selectedID := cleanString(rehearsal["selected_scenario_id"])
	if selectedID == "" {
		violations = append(violations, "arc_rehearsal.selected_scenario_id is required")
	}

	scenarios, ok := rehearsal["scenarios"].([]any)
	if !ok {
		violations = append(violations, "arc_rehearsal.scenarios must be an array")
		scenarios = nil
	} else if len(scenarios) < 2 {
		violations = append(violations, "arc_rehearsal.scenarios requires at least two items")
	}

	seen := map[string]bool{}
	var selectedNextArc any
	selectedFound := false
	for i, rawScenario := range scenarios {
		scenario, ok := rawScenario.(map[string]any)
		if !ok {
			violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d] must be an object", i))
			continue
		}
		id := cleanString(scenario["id"])
		if id == "" {
			violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d].id is required", i))
		} else if seen[id] {
			violations = append(violations, "arc_rehearsal scenario id must be unique: "+id)
		} else {
			seen[id] = true
		}
		if cleanString(scenario["opportunity"]) == "" {
			violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d].opportunity is required", i))
		}
		if cleanString(scenario["risk"]) == "" {
			violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d].risk is required", i))
		}
		nextArc, exists := scenario["next_arc"]
		if !exists {
			violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d].next_arc is required", i))
		} else {
			_, arcViolations := decodeCoreArc(nextArc, false)
			for _, problem := range arcViolations {
				violations = append(violations, fmt.Sprintf("arc_rehearsal.scenarios[%d].next_arc: %s", i, problem))
			}
		}
		if id != "" && id == selectedID {
			selectedFound = true
			selectedNextArc = nextArc
		}
	}
	if selectedID != "" && !selectedFound {
		violations = append(violations, "arc_rehearsal.selected_scenario_id does not reference a scenario")
	}
	if selectedFound {
		patchNextArc, exists := patch["next_arc"]
		if !exists || !reflect.DeepEqual(selectedNextArc, patchNextArc) {
			violations = append(violations, "arc_rehearsal selected scenario next_arc must equal planning_patch.next_arc")
		}
	}

	canonical, err := json.Marshal(rehearsal)
	if err != nil {
		return nil, nil, err
	}
	return canonical, violations, nil
}

func isQualityChapterAttempt(attempt *domain.CoreAttempt) bool {
	// chapter_review.json is the compatibility marker for a quality-enabled
	// attempt. A review-only attempt can exist while older active contracts are
	// being preserved, so requiring chapter_plan.json here would silently
	// weaken the planning/rehearsal coupling promised by the protocol.
	return requiresArtifact(attempt, "chapter_review.json")
}
