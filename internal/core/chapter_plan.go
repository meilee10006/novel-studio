package core

import (
	"encoding/json"
	"fmt"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func validateChapterPlan(raw []byte, task *domain.CoreTask) ([]byte, []string, error) {
	var plan map[string]any
	if err := protocol.DecodeJSON(raw, &plan); err != nil {
		return nil, nil, fmt.Errorf("chapter_plan.json: %w", err)
	}
	var violations []string
	chapter, err := chapterNumberFromTarget(task.Target)
	if err != nil {
		return nil, nil, err
	}
	declared := exactJSONInt(plan["chapter"])
	if declared != chapter {
		violations = append(violations, "chapter_plan.chapter does not match task target")
	}
	if cleanString(plan["base_canon_root"]) != task.BaseCanonRoot {
		violations = append(violations, "chapter_plan.base_canon_root does not match task base")
	}
	for _, field := range []string{"objective", "reader_payoff", "ending_hook_intent"} {
		if cleanString(plan[field]) == "" {
			violations = append(violations, "chapter_plan."+field+" is required")
		}
	}
	beats, ok := plan["beats"].([]any)
	if !ok {
		violations = append(violations, "chapter_plan.beats must be an array")
	} else {
		if len(beats) < 2 {
			violations = append(violations, "chapter_plan.beats requires at least two items")
		}
		seen := map[string]bool{}
		for i, rawBeat := range beats {
			beat, ok := rawBeat.(map[string]any)
			if !ok {
				violations = append(violations, fmt.Sprintf("chapter_plan.beats[%d] must be an object", i))
				continue
			}
			id := cleanString(beat["id"])
			if id == "" {
				violations = append(violations, fmt.Sprintf("chapter_plan.beats[%d].id is required", i))
			} else if seen[id] {
				violations = append(violations, "chapter_plan beat id must be unique: "+id)
			} else {
				seen[id] = true
			}
			if cleanString(beat["intent"]) == "" {
				violations = append(violations, fmt.Sprintf("chapter_plan.beats[%d].intent is required", i))
			}
		}
	}
	canonical, err := json.Marshal(plan)
	if err != nil {
		return nil, nil, err
	}
	return canonical, violations, nil
}
