package core

import (
	"encoding/json"
	"fmt"
	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"sort"
	"strings"
)

func validateAndCanonicalizeFoundation(artifacts map[string][]byte, state *domain.CoreProductionState) (map[string][]byte, []IDMapping, []string, error) {
	if err := exactArtifactSet(artifacts, foundationArtifactNames); err != nil {
		return nil, nil, nil, err
	}
	values := make(map[string]any, len(artifacts))
	for _, name := range foundationArtifactNames {
		var value any
		if err := protocol.DecodeJSON(artifacts[name], &value); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		values[name] = value
	}
	var violations []string
	requireStringField(values["foundation.json"], "title", "foundation.title", &violations)
	requireFoundationTypedReference(values["foundation.json"], "protagonist", "character", &violations)
	requireFoundationTypedReference(values["foundation.json"], "opening_location", "location", &violations)
	requireStringField(values["book_plan.json"], "direction", "book_plan.direction", &violations)
	requireStringField(values["ending_contract.json"], "main_resolution", "ending_contract.main_resolution", &violations)
	requireStringField(values["style_profile.json"], "language", "style_profile.language", &violations)
	requireStringField(values["platform_profile.json"], "platform", "platform_profile.platform", &violations)

	defs := collectFoundationEntities(values, &violations)
	if len(violations) > 0 {
		return nil, nil, violations, nil
	}
	keys := make([]string, 0, len(defs))
	for key := range defs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mappings := make([]IDMapping, 0, len(keys))
	lookup := make(map[string]string, len(keys))
	seq := state.NextEntitySeq
	for _, key := range keys {
		def := defs[key]
		id := fmt.Sprintf("%s-%06d", safeIDPrefix(def.entityType), seq)
		seq++
		lookup[key] = id
		mappings = append(mappings, IDMapping{EntityType: def.entityType, LocalID: def.localID, CanonID: id})
	}
	applyDefinitionIDs(values, lookup)
	for _, value := range values {
		rewriteFoundationRefs(value, lookup, &violations)
	}
	if len(violations) > 0 {
		return nil, nil, violations, nil
	}
	canonical := make(map[string][]byte, len(values))
	for name, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return nil, nil, nil, err
		}
		canonical[name] = data
	}
	return canonical, mappings, nil, nil
}

type foundationEntityDef struct{ entityType, localID string }

func collectFoundationEntities(values map[string]any, violations *[]string) map[string]foundationEntityDef {
	defs := map[string]foundationEntityDef{}
	if root, ok := values["characters.json"].(map[string]any); ok {
		items, ok := root["characters"].([]any)
		if !ok {
			*violations = append(*violations, "characters must be an array")
		} else {
			if len(items) == 0 {
				*violations = append(*violations, "characters must contain at least one character")
			}
			for _, item := range items {
				collectEntityDef(item, "character", defs, violations)
			}
		}
	} else {
		*violations = append(*violations, "characters.json must be an object")
	}
	if root, ok := values["world.json"].(map[string]any); ok {
		if rawEntities, exists := root["entities"]; exists {
			items, ok := rawEntities.([]any)
			if !ok {
				*violations = append(*violations, "world.entities must be an array")
			} else {
				for _, item := range items {
					m, _ := item.(map[string]any)
					entityType, _ := m["entity_type"].(string)
					collectEntityDef(item, entityType, defs, violations)
				}
			}
		}
	} else {
		*violations = append(*violations, "world.json must be an object")
	}
	return defs
}
func collectEntityDef(value any, entityType string, defs map[string]foundationEntityDef, violations *[]string) {
	m, ok := value.(map[string]any)
	if !ok {
		*violations = append(*violations, "entity definition must be an object")
		return
	}
	entityType = strings.TrimSpace(entityType)
	localID, _ := m["local_id"].(string)
	localID = strings.TrimSpace(localID)
	name, _ := m["name"].(string)
	name = strings.TrimSpace(name)
	if entityType == "" || localID == "" || name == "" {
		*violations = append(*violations, "entity definition requires entity_type, local_id, and name")
		return
	}
	if _, exists := m["canon_id"]; exists {
		*violations = append(*violations, "entity definition must not predeclare canonical id")
		return
	}
	key := entityType + "\x00" + localID
	if _, exists := defs[key]; exists {
		*violations = append(*violations, "duplicate local id for entity type: "+entityType+"/"+localID)
		return
	}
	defs[key] = foundationEntityDef{entityType: entityType, localID: localID}
}
func requireFoundationTypedReference(value any, field, expectedType string, violations *[]string) {
	root, ok := value.(map[string]any)
	if !ok {
		return
	}
	ref, ok := root[field].(map[string]any)
	if !ok {
		*violations = append(*violations, "foundation."+field+" must be an object reference")
		return
	}
	entityType, ok := ref["entity_type"].(string)
	if !ok || strings.TrimSpace(entityType) != expectedType {
		*violations = append(*violations, "foundation."+field+" must reference entity_type "+expectedType)
		return
	}
	localRef, ok := ref["local_ref"].(string)
	if !ok || strings.TrimSpace(localRef) == "" {
		*violations = append(*violations, "foundation."+field+" requires non-empty local_ref")
	}
}
func requireStringField(value any, field, label string, violations *[]string) {
	m, ok := value.(map[string]any)
	if !ok {
		*violations = append(*violations, label+" container must be an object")
		return
	}
	s, _ := m[field].(string)
	if strings.TrimSpace(s) == "" {
		*violations = append(*violations, label+" is required")
	}
}
func applyDefinitionIDs(values map[string]any, lookup map[string]string) {
	if root, ok := values["characters.json"].(map[string]any); ok {
		if items, ok := root["characters"].([]any); ok {
			for _, item := range items {
				replaceDefinitionID(item, "character", lookup)
			}
		}
	}
	if root, ok := values["world.json"].(map[string]any); ok {
		if items, ok := root["entities"].([]any); ok {
			for _, item := range items {
				m, _ := item.(map[string]any)
				entityType, _ := m["entity_type"].(string)
				replaceDefinitionID(item, entityType, lookup)
			}
		}
	}
}
func replaceDefinitionID(value any, entityType string, lookup map[string]string) {
	m, ok := value.(map[string]any)
	if !ok {
		return
	}
	localID, _ := m["local_id"].(string)
	if id := lookup[strings.TrimSpace(entityType)+"\x00"+strings.TrimSpace(localID)]; id != "" {
		m["canon_id"] = id
		delete(m, "local_id")
	}
}
