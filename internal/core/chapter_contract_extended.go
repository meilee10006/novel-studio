package core

import (
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func validateExtendedChapterContract(contract map[string]any, body string, state *domain.CoreProductionState, task *domain.CoreTask, changes []any, violations *[]string) {
	_ = body
	_ = state
	_ = changes

	if raw, exists := contract["start_state"]; exists {
		start, ok := raw.(map[string]any)
		if !ok {
			*violations = append(*violations, "chapter_contract.start_state must be an object")
		} else if rawRoot, provided := start["canon_root"]; provided {
			root, ok := rawRoot.(string)
			if !ok || strings.TrimSpace(root) != task.BaseCanonRoot {
				*violations = append(*violations, "chapter_contract.start_state.canon_root must match task base_canon_root")
			}
		}
	}

	for _, field := range []string{"purpose", "main_conflict", "reader_question", "ending_hook"} {
		if raw, exists := contract[field]; exists {
			value, ok := raw.(string)
			if !ok || strings.TrimSpace(value) == "" {
				*violations = append(*violations, "chapter_contract."+field+" must be a non-empty string")
			}
		}
	}

	for _, field := range []string{"obligation_refs", "immutable_refs", "planning_obligations"} {
		if raw, exists := contract[field]; exists {
			_, problems := stringArray(raw, "chapter_contract."+field)
			*violations = append(*violations, problems...)
		}
	}

	if raw, exists := contract["expected_changes"]; exists {
		items, problems := stringArray(raw, "chapter_contract.expected_changes")
		*violations = append(*violations, problems...)
		for _, kind := range items {
			if !isSupportedContractChangeKind(kind) {
				*violations = append(*violations, "chapter_contract.expected_changes contains unsupported state change kind: "+kind)
			}
		}
	}

	if raw, exists := contract["foreshadow_operations"]; exists {
		items, ok := raw.([]any)
		if !ok {
			*violations = append(*violations, "chapter_contract.foreshadow_operations must be an array")
		} else {
			for _, rawItem := range items {
				item, ok := rawItem.(map[string]any)
				if !ok {
					*violations = append(*violations, "chapter_contract.foreshadow_operations item must be an object")
					continue
				}
				id, ok := item["foreshadow_id"].(string)
				if !ok || strings.TrimSpace(id) == "" {
					*violations = append(*violations, "chapter_contract.foreshadow_operations foreshadow_id is required")
				}
				_, problems := stringArray(item["allowed_states"], "chapter_contract.foreshadow_operations allowed_states")
				*violations = append(*violations, problems...)
			}
		}
	}

	if raw, exists := contract["target_length"]; exists {
		target, ok := raw.(map[string]any)
		if !ok {
			*violations = append(*violations, "chapter_contract.target_length must be an object")
		} else {
			minChars, minOK := integer64(target["min_chars"])
			maxChars, maxOK := integer64(target["max_chars"])
			if !minOK || !maxOK || minChars < 0 || maxChars < 0 || minChars > maxChars {
				*violations = append(*violations, "chapter_contract.target_length is invalid")
			}
		}
	}
}

func isSupportedContractChangeKind(kind string) bool {
	switch kind {
	case "character_add", "location_add", "resource_add", "knowledge_add", "resource", "location", "relationship", "foreshadow", "conflict", "reader_promise", "ending_resolution":
		return true
	default:
		return false
	}
}
