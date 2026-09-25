package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

var requiredChapterReviewDimensions = []string{
	"story_progress",
	"reader_payoff",
	"pacing",
	"character_consistency",
	"world_consistency",
	"continuity",
	"ending_hook",
}

func validateChapterReview(raw, chapterBody []byte, requirePlan bool) ([]byte, []string, error) {
	var review map[string]any
	if err := protocol.DecodeJSON(raw, &review); err != nil {
		return nil, nil, fmt.Errorf("chapter_review.json: %w", err)
	}

	var violations []string
	if cleanString(review["subject_ref"]) != "chapter.md" {
		violations = append(violations, "chapter_review.subject_ref must be chapter.md")
	}
	if rawPlanRef, exists := review["plan_ref"]; exists {
		if cleanString(rawPlanRef) != "chapter_plan.json" {
			violations = append(violations, "chapter_review.plan_ref must be chapter_plan.json")
		}
	} else if requirePlan {
		violations = append(violations, "chapter_review.plan_ref is required")
	}

	dimensions, ok := review["dimensions"].(map[string]any)
	if !ok {
		violations = append(violations, "chapter_review.dimensions must be an object")
		dimensions = map[string]any{}
	}
	reviseDimensions := 0
	for _, name := range requiredChapterReviewDimensions {
		rawDimension, exists := dimensions[name]
		if !exists {
			violations = append(violations, "chapter_review.dimensions."+name+" is required")
			continue
		}
		dimension, ok := rawDimension.(map[string]any)
		if !ok {
			violations = append(violations, "chapter_review.dimensions."+name+" must be an object")
			continue
		}
		status := cleanString(dimension["status"])
		switch status {
		case "pass":
		case "revise":
			reviseDimensions++
		default:
			violations = append(violations, "chapter_review.dimensions."+name+".status must be pass or revise")
		}
		if cleanString(dimension["note"]) == "" {
			violations = append(violations, "chapter_review.dimensions."+name+".note is required")
		}
	}

	blockingIssues := 0
	if rawIssues, exists := review["issues"]; exists {
		issues, ok := rawIssues.([]any)
		if !ok {
			violations = append(violations, "chapter_review.issues must be an array")
		} else {
			body := string(chapterBody)
			seenCodes := map[string]bool{}
			for i, rawIssue := range issues {
				issue, ok := rawIssue.(map[string]any)
				if !ok {
					violations = append(violations, fmt.Sprintf("chapter_review.issues[%d] must be an object", i))
					continue
				}
				code := cleanString(issue["code"])
				if code == "" {
					violations = append(violations, fmt.Sprintf("chapter_review.issues[%d].code is required", i))
				} else if seenCodes[code] {
					violations = append(violations, "chapter_review issue code must be unique: "+code)
				} else {
					seenCodes[code] = true
				}
				if cleanString(issue["description"]) == "" {
					violations = append(violations, fmt.Sprintf("chapter_review.issues[%d].description is required", i))
				}
				severity := cleanString(issue["severity"])
				switch severity {
				case "advisory":
				case "blocking":
					blockingIssues++
					anchor := cleanString(issue["evidence_anchor"])
					if anchor == "" {
						violations = append(violations, fmt.Sprintf("chapter_review.issues[%d].evidence_anchor is required for blocking issue", i))
					} else if !strings.Contains(body, anchor) {
						violations = append(violations, fmt.Sprintf("chapter_review.issues[%d].evidence_anchor is absent from chapter.md", i))
					}
				default:
					violations = append(violations, fmt.Sprintf("chapter_review.issues[%d].severity must be blocking or advisory", i))
				}
			}
		}
	}

	switch verdict := cleanString(review["verdict"]); verdict {
	case "pass":
		if reviseDimensions > 0 || blockingIssues > 0 {
			violations = append(violations, "chapter_review verdict=pass contradicts revise findings")
		}
	case "revise":
		if reviseDimensions == 0 && blockingIssues == 0 {
			violations = append(violations, "chapter_review verdict=revise requires a revise dimension or blocking issue")
		} else {
			violations = append(violations, "chapter_review verdict=revise requests rewrite")
		}
	default:
		violations = append(violations, "chapter_review.verdict must be pass or revise")
	}

	canonical, err := json.Marshal(review)
	if err != nil {
		return nil, nil, err
	}
	return canonical, violations, nil
}
