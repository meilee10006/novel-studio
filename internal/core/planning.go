package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func foundationPlanningState(bookPlan []byte) (domain.CorePlanningState, []string, error) {
	var root map[string]any
	if err := protocol.DecodeJSON(bookPlan, &root); err != nil {
		return domain.CorePlanningState{}, nil, err
	}
	value, ok := root["current_arc"]
	if !ok {
		return domain.CorePlanningState{}, nil, nil
	}
	arc, violations := decodeCoreArc(value, true)
	if len(violations) > 0 {
		return domain.CorePlanningState{}, violations, nil
	}
	return domain.CorePlanningState{CurrentArc: arc}, nil, nil
}

func validateChapterPlanning(files map[string][]byte, current domain.CorePlanningState, chapter int, required bool) (string, domain.CorePlanningState, []byte, bool, []string, error) {
	next := current
	raw, present := files["planning_patch.json"]
	status := "not_present"
	repair := false
	var violations []string
	var canonical []byte
	if !present {
		if required {
			return "rejected", next, nil, true, []string{"planning_patch.json is required to repair rolling arc planning"}, nil
		}
	} else {
		status = "rejected"
		var patch map[string]any
		if err := protocol.DecodeJSON(raw, &patch); err != nil {
			violations = append(violations, "planning_patch.json is invalid JSON: "+err.Error())
		} else if value, ok := patch["next_arc"]; !ok {
			violations = append(violations, "planning_patch.next_arc is required")
		} else if current.CurrentArc.ID == "" {
			violations = append(violations, "planning patch requires a current arc")
		} else {
			arc, arcViolations := decodeCoreArc(value, false)
			violations = append(violations, arcViolations...)
			if len(arcViolations) == 0 {
				if arc.ID == current.CurrentArc.ID {
					violations = append(violations, "next arc id must differ from current arc")
				}
				if arc.StartChapter != current.CurrentArc.EndChapter+1 {
					violations = append(violations, fmt.Sprintf("next arc must start at chapter %d", current.CurrentArc.EndChapter+1))
				}
			}
			if len(violations) == 0 {
				next.NextArc = &arc
				status = "accepted"
				canonical, _ = json.Marshal(map[string]any{"next_arc": arc})
			}
		}
		if status == "rejected" {
			repair = true
		}
	}
	if current.CurrentArc.ID != "" && chapter >= current.CurrentArc.EndChapter {
		if next.NextArc != nil && next.NextArc.StartChapter == current.CurrentArc.EndChapter+1 {
			next.CurrentArc = *next.NextArc
			next.NextArc = nil
			repair = false
		} else {
			repair = true
		}
	}
	return status, next, canonical, repair, violations, nil
}

func decodeCoreArc(value any, foundation bool) (domain.CoreArcPlan, []string) {
	m, ok := value.(map[string]any)
	if !ok {
		return domain.CoreArcPlan{}, []string{"arc must be an object"}
	}
	var violations []string
	arc := domain.CoreArcPlan{}
	arc.ID, _ = m["id"].(string)
	arc.ID = strings.TrimSpace(arc.ID)
	arc.Goal, _ = m["goal"].(string)
	arc.Goal = strings.TrimSpace(arc.Goal)
	arc.StartChapter = exactJSONInt(m["start_chapter"])
	arc.EndChapter = exactJSONInt(m["end_chapter"])
	if foundation {
		if _, exists := m["planning_lead_chapters"]; exists {
			arc.PlanningLeadChapters = exactJSONIntAllowZero(m["planning_lead_chapters"])
		}
	}
	if arc.ID == "" {
		violations = append(violations, "arc.id is required")
	}
	if arc.Goal == "" {
		violations = append(violations, "arc.goal is required")
	}
	if arc.StartChapter <= 0 || arc.EndChapter < arc.StartChapter {
		violations = append(violations, "arc chapter range is invalid")
	}
	if foundation && arc.PlanningLeadChapters < 0 {
		violations = append(violations, "arc.planning_lead_chapters cannot be negative")
	}
	return arc, violations
}

func exactJSONInt(v any) int {
	f, ok := v.(float64)
	if !ok || f <= 0 || f != float64(int(f)) {
		return 0
	}
	return int(f)
}

func exactJSONIntAllowZero(v any) int {
	f, ok := v.(float64)
	if !ok || f < 0 || f != float64(int(f)) {
		return -1
	}
	return int(f)
}

func addRollingPlanningObligation(task *domain.CoreTask, planning domain.CorePlanningState, chapter int) {
	if task == nil || task.Kind != "chapter" || planning.NextArc != nil {
		return
	}
	arc := planning.CurrentArc
	if arc.ID == "" || arc.PlanningLeadChapters <= 0 || chapter > arc.EndChapter {
		return
	}
	firstDue := arc.EndChapter - arc.PlanningLeadChapters
	if firstDue < arc.StartChapter {
		firstDue = arc.StartChapter
	}
	if chapter < firstDue {
		return
	}
	task.Constraints = append(task.Constraints, domain.CoreTaskConstraint{
		Kind:        "rolling_planning_due",
		Instruction: "Prepare and submit a valid next Arc with planning_patch.json before the current Arc ends; the patch remains optional until the Arc boundary.",
	})
}

func requiresArtifact(attempt *domain.CoreAttempt, name string) bool {
	for _, item := range attempt.RequiredArtifacts {
		if item == name {
			return true
		}
	}
	return false
}
