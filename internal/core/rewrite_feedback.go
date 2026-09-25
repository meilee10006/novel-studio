package core

import (
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

type RewriteFeedback struct {
	Code         string   `json:"code"`
	EntityRef    string   `json:"entity_ref,omitempty"`
	Expected     string   `json:"expected"`
	Observed     string   `json:"observed"`
	EvidenceRefs []string `json:"evidence_refs"`
	AllowedScope []string `json:"allowed_scope"`
}

type rewriteFeedbackClass struct {
	Code     string
	Artifact string
	Expected string
}

func buildRewriteFeedback(violations []string, task *domain.CoreTask) []RewriteFeedback {
	feedback := make([]RewriteFeedback, len(violations))
	for i, violation := range violations {
		class := classifyRewriteViolation(violation, task)
		item := RewriteFeedback{
			Code:         class.Code,
			Expected:     class.Expected,
			Observed:     violation,
			EvidenceRefs: []string{class.Artifact},
			AllowedScope: []string{class.Artifact},
		}
		if task != nil && strings.TrimSpace(task.Target) != "" {
			item.EntityRef = task.Target
		}
		feedback[i] = item
	}
	return feedback
}

func classifyRewriteViolation(violation string, task *domain.CoreTask) rewriteFeedbackClass {
	lower := strings.ToLower(violation)
	class := rewriteFeedbackClass{
		Code:     "validation.failed",
		Artifact: "manifest.json",
		Expected: "submission satisfies deterministic Core validation",
	}

	if task != nil && task.Kind == "foundation" {
		switch {
		case strings.Contains(lower, "characters") || strings.Contains(lower, "character local"):
			return rewriteFeedbackClass{"foundation.characters.invalid", "characters.json", "characters.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "world") || strings.Contains(lower, "travel constraint") || strings.Contains(lower, "location local") || strings.Contains(lower, "resource local"):
			return rewriteFeedbackClass{"foundation.world.invalid", "world.json", "world.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "book_plan") || strings.Contains(lower, "book plan") || strings.Contains(lower, "arc"):
			return rewriteFeedbackClass{"foundation.book_plan.invalid", "book_plan.json", "book_plan.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "ending"):
			return rewriteFeedbackClass{"foundation.ending_contract.invalid", "ending_contract.json", "ending_contract.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "style"):
			return rewriteFeedbackClass{"foundation.style_profile.invalid", "style_profile.json", "style_profile.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "platform"):
			return rewriteFeedbackClass{"foundation.platform_profile.invalid", "platform_profile.json", "platform_profile.json satisfies deterministic Foundation validation"}
		case strings.Contains(lower, "foundation") || strings.Contains(lower, "protagonist") || strings.Contains(lower, "opening_location"):
			return rewriteFeedbackClass{"foundation.invalid", "foundation.json", "foundation.json satisfies deterministic Foundation validation"}
		}
	}

	switch {
	case strings.Contains(lower, "chapter body"):
		return rewriteFeedbackClass{"chapter.body.invalid", "chapter.md", "chapter.md contains a non-empty chapter body"}
	case strings.Contains(lower, "chapter_plan"):
		return rewriteFeedbackClass{"chapter_plan.invalid", "chapter_plan.json", "chapter_plan.json matches the active chapter and base Canon with at least two valid beats"}
	case strings.Contains(lower, "arc_rehearsal"):
		return rewriteFeedbackClass{"arc_rehearsal.invalid", "arc_rehearsal.json", "arc_rehearsal.json contains at least two valid alternatives and selects the exact planning_patch.next_arc"}
	case strings.Contains(lower, "chapter_review") && strings.Contains(lower, "verdict=revise"):
		return rewriteFeedbackClass{"chapter_review.revise", "chapter_review.json", "chapter_review.json records verdict=pass for the replacement draft"}
	case strings.Contains(lower, "chapter_review"):
		return rewriteFeedbackClass{"chapter_review.invalid", "chapter_review.json", "chapter_review.json satisfies deterministic exact-snapshot review validation"}
	case strings.Contains(lower, "self_review") || strings.Contains(lower, "author_decision_required") || strings.Contains(lower, "author decision"):
		return rewriteFeedbackClass{"self_review.invalid", "self_review.json", "self_review.json satisfies deterministic review validation"}
	case strings.Contains(lower, "chapter_contract.hard_constraints") || strings.Contains(lower, "duplicate hard constraint id"):
		return rewriteFeedbackClass{"chapter_contract.hard_constraints.invalid", "chapter_contract.json", "chapter_contract.hard_constraints is a valid unique-id array"}
	case strings.Contains(lower, "chapter_contract.declared_pov"):
		return rewriteFeedbackClass{"chapter_contract.declared_pov.invalid", "chapter_contract.json", "chapter_contract.declared_pov is a valid canonical character reference"}
	case strings.Contains(lower, "chapter_contract.chapter"):
		return rewriteFeedbackClass{"chapter_contract.chapter.invalid", "chapter_contract.json", "chapter_contract.chapter matches the active task target"}
	case strings.Contains(lower, "chapter_contract"):
		return rewriteFeedbackClass{"chapter_contract.invalid", "chapter_contract.json", "chapter_contract.json satisfies deterministic contract validation"}
	case strings.Contains(lower, "planning") || strings.Contains(lower, "next arc") || strings.Contains(lower, "current arc") || strings.HasPrefix(lower, "arc ") || strings.HasPrefix(lower, "arc."):
		return rewriteFeedbackClass{"planning.invalid", "planning_patch.json", "planning_patch.json satisfies deterministic planning validation"}
	case strings.Contains(lower, "state_delta") || strings.Contains(lower, "state delta") || strings.Contains(lower, "state change") || strings.Contains(lower, "character_add") ||
		strings.Contains(lower, "event_ref") || strings.Contains(lower, "event_canon_id") || strings.Contains(lower, "current attempt canonical") ||
		strings.Contains(lower, "current attempt story event") || strings.Contains(lower, "knowledge") || strings.Contains(lower, "resource") ||
		strings.Contains(lower, "location") || strings.Contains(lower, "relationship") || strings.Contains(lower, "foreshadow") ||
		strings.Contains(lower, "conflict") || strings.Contains(lower, "reader promise") || strings.Contains(lower, "ending resolution") ||
		strings.Contains(lower, "travel constraint"):
		return rewriteFeedbackClass{"state_delta.invalid", "state_delta.json", "state_delta.json satisfies deterministic state validation"}
	case strings.Contains(lower, "story event") || strings.Contains(lower, "events.json") || strings.Contains(lower, "offscreen") || strings.Contains(lower, "evidence anchor"):
		return rewriteFeedbackClass{"events.invalid", "events.json", "events.json satisfies deterministic event validation"}
	}

	if task != nil && task.Kind == "foundation" {
		return rewriteFeedbackClass{"foundation.validation.failed", "foundation.json", "Foundation submission satisfies deterministic Core validation"}
	}
	return class
}
