package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
	"github.com/chenhongyang/novel-studio/internal/retrieval"
)

const taskContextBudget = 64 << 10

type contextChapter struct {
	Chapter int
	Path    string
	Text    string
}

type contextCompilerInput struct {
	Target              string
	CanonRoot           string
	EndingContract      map[string]any
	BookPlan            map[string]any
	FoundationReference map[string]any
	Constraints         []domain.CoreTaskConstraint
	CanonState          domain.CoreCanonState
	Chapters            []contextChapter
	RevisionCandidate   map[string]any
}
type contextCompilerOutput struct {
	ContextJSON      []byte
	CanonExcerptJSON []byte
	RecentProse      []byte
	TotalBytes       int
}

func compileTaskContext(input contextCompilerInput, budget int) (contextCompilerOutput, error) {
	if budget <= 0 {
		return contextCompilerOutput{}, fmt.Errorf("context budget must be positive")
	}
	query := contextQuery(input)
	contextDoc := map[string]any{
		"target":              input.Target,
		"book_plan":           input.BookPlan,
		"ending_contract":     input.EndingContract,
		"control_constraints": input.Constraints,
		"retrieved_old_prose": []map[string]any{},
	}
	if input.FoundationReference != nil {
		contextDoc["foundation_reference"] = compactFoundationReferenceForContext(input.FoundationReference, query, budget/4)
	}
	if input.RevisionCandidate != nil {
		contextDoc["revision_candidate"] = input.RevisionCandidate
	}
	canonDoc := map[string]any{
		"base_canon_root": input.CanonRoot,
		"state":           compactCanonStateForContext(input.CanonState, query, budget/4),
	}
	contextRaw, canonRaw, err := marshalContextDocs(contextDoc, canonDoc)
	if err != nil {
		return contextCompilerOutput{}, err
	}
	if len(contextRaw)+len(canonRaw) > budget {
		return contextCompilerOutput{}, fmt.Errorf("hard context exceeds byte budget")
	}
	chapters := append([]contextChapter(nil), input.Chapters...)
	sort.Slice(chapters, func(i, j int) bool { return chapters[i].Chapter < chapters[j].Chapter })
	recent := buildRecentProse(chapters, budget/4)
	if len(contextRaw)+len(canonRaw)+len(recent) > budget {
		recent = truncateUTF8(recent, budget-len(contextRaw)-len(canonRaw))
	}
	docs := make([]retrieval.Document, 0, len(chapters))
	for _, chapter := range chapters {
		docs = append(docs, retrieval.Document{ID: chapter.Path, Text: chapter.Text})
	}
	hits := retrieval.RankKeyword(docs, query, 8)
	retrieved := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		candidate := append(retrieved, map[string]any{"path": hit.ID, "text": hit.Text})
		contextDoc["retrieved_old_prose"] = candidate
		candidateRaw, _, err := marshalContextDocs(contextDoc, canonDoc)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		if len(candidateRaw)+len(canonRaw)+len(recent) > budget {
			break
		}
		retrieved = candidate
		contextRaw = candidateRaw
	}
	contextDoc["retrieved_old_prose"] = retrieved
	contextRaw, canonRaw, err = marshalContextDocs(contextDoc, canonDoc)
	if err != nil {
		return contextCompilerOutput{}, err
	}
	return contextCompilerOutput{ContextJSON: contextRaw, CanonExcerptJSON: canonRaw, RecentProse: recent, TotalBytes: len(contextRaw) + len(canonRaw) + len(recent)}, nil
}
func marshalContextDocs(contextDoc, canonDoc map[string]any) ([]byte, []byte, error) {
	contextRaw, err := json.Marshal(contextDoc)
	if err != nil {
		return nil, nil, err
	}
	canonRaw, err := json.Marshal(canonDoc)
	if err != nil {
		return nil, nil, err
	}
	return contextRaw, canonRaw, nil
}

func contextQuery(input contextCompilerInput) string {
	parts := []string{input.Target}
	for _, constraint := range input.Constraints {
		if constraint.Instruction != "" {
			parts = append(parts, constraint.Instruction)
		}
		if constraint.Choice != "" {
			parts = append(parts, constraint.Choice)
		}
	}
	if direction, _ := input.BookPlan["direction"].(string); direction != "" {
		parts = append(parts, direction)
	}
	return strings.Join(parts, "\n")
}

func buildRecentProse(chapters []contextChapter, maxBytes int) []byte {
	if maxBytes <= 0 || len(chapters) == 0 {
		return nil
	}
	start := len(chapters) - 2
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	for _, chapter := range chapters[start:] {
		piece := fmt.Sprintf("## 第%d章\n%s\n", chapter.Chapter, chapter.Text)
		remaining := maxBytes - b.Len()
		if remaining <= 0 {
			break
		}
		b.Write(truncateUTF8([]byte(piece), remaining))
	}
	return []byte(b.String())
}

func truncateUTF8(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 {
		return nil
	}
	if len(data) <= maxBytes {
		return data
	}
	cut := maxBytes
	for cut > 0 && !utf8.Valid(data[:cut]) {
		cut--
	}
	return data[:cut]
}

func compactCanonStateForContext(state domain.CoreCanonState, query string, knowledgeBudget int) map[string]any {
	out := map[string]any{
		"schema_version": state.SchemaVersion,
		"revision":       state.Revision,
		"project_id":     state.ProjectID,
		"latest_chapter": state.LatestChapter,
	}
	longform := map[string]any{}

	entities := compactEntitiesForContext(state.Longform, query, knowledgeBudget/2)
	if len(entities) > 0 {
		longform["entities"] = entities
	}

	knowledge := compactKnowledgeForContext(state.Longform, query, knowledgeBudget)
	if len(knowledge) > 0 {
		longform["knowledge"] = knowledge
	}

	locations := compactLocationsForContext(state.Longform, query, knowledgeBudget/2)
	if len(locations) > 0 {
		longform["locations"] = locations
	}

	resources := compactResourcesForContext(state.Longform, query, knowledgeBudget/2)
	if len(resources) > 0 {
		longform["resources"] = resources
	}

	relationships := compactRelationshipsForContext(state.Longform, query, knowledgeBudget/2)
	if len(relationships) > 0 {
		longform["relationships"] = relationships
	}

	foreshadows := compactForeshadowsForContext(state.Longform, query, knowledgeBudget/2)
	if len(foreshadows) > 0 {
		longform["foreshadows"] = foreshadows
	}

	promises := compactReaderPromisesForContext(state.Longform, query, knowledgeBudget/2)
	if len(promises) > 0 {
		longform["reader_promises"] = promises
	}

	conflicts := compactConflictsForContext(state.Longform, query, knowledgeBudget/2)
	if len(conflicts) > 0 {
		longform["conflicts"] = conflicts
	}

	travel := compactTravelConstraintsForContext(state.Longform, query, knowledgeBudget/2)
	if len(travel) > 0 {
		longform["travel_constraints"] = travel
	}
	if state.Longform.Ending != nil {
		longform["ending"] = map[string]any{"main_resolution_event_id": compactContextString(state.Longform.Ending.MainResolutionEventID)}
	}
	if len(longform) > 0 {
		out["longform"] = longform
	}
	if state.Planning.CurrentArc.ID != "" || state.Planning.NextArc != nil {
		planning := map[string]any{}
		if state.Planning.CurrentArc.ID != "" {
			planning["current_arc"] = compactArcPlanForContext(state.Planning.CurrentArc)
		}
		if state.Planning.NextArc != nil {
			planning["next_arc"] = compactArcPlanForContext(*state.Planning.NextArc)
		}
		out["planning"] = planning
	}
	return out
}

type contextEntityCandidate struct {
	ID      string
	Entity  domain.CoreEntityState
	Chapter int
}

func compactEntitiesForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]string {
	if maxBytes <= 0 || len(state.Entities) == 0 {
		return nil
	}
	ids := make([]string, 0, len(state.Entities))
	for id := range state.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	candidates := make([]contextEntityCandidate, 0, len(ids))
	byID := map[string]contextEntityCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		entity := state.Entities[id]
		chapter := 0
		if event, ok := state.Events[entity.EvidenceEventID]; ok {
			chapter = event.Chapter
		}
		candidate := contextEntityCandidate{ID: id, Entity: entity, Chapter: chapter}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		docs = append(docs, retrieval.Document{ID: id, Text: id + "\n" + entity.Name + "\n" + entity.Description})
	}
	ordered := make([]contextEntityCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextEntityCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Chapter != remaining[j].Chapter {
			return remaining[i].Chapter > remaining[j].Chapter
		}
		return remaining[i].ID < remaining[j].ID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]string{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		item := map[string]string{
			"entity_type": compactContextString(candidate.Entity.EntityType),
			"name":        compactContextString(candidate.Entity.Name),
		}
		if description := compactContextString(candidate.Entity.Description); description != "" {
			item["description"] = description
		}
		selected[id] = item
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

type contextKnowledgeCandidate struct {
	ID          string
	CharacterID string
	FactID      string
	Fact        domain.CoreKnowledgeFact
	Chapter     int
}

func compactKnowledgeForContext(state domain.CoreLongformState, query string, maxBytes int) map[string][]map[string]string {
	if maxBytes <= 0 || len(state.Knowledge) == 0 {
		return nil
	}
	characterIDs := make([]string, 0, len(state.Knowledge))
	for characterID := range state.Knowledge {
		characterIDs = append(characterIDs, characterID)
	}
	sort.Strings(characterIDs)

	candidates := make([]contextKnowledgeCandidate, 0)
	byID := map[string]contextKnowledgeCandidate{}
	docs := make([]retrieval.Document, 0)
	for _, characterID := range characterIDs {
		facts := state.Knowledge[characterID]
		factIDs := make([]string, 0, len(facts))
		for factID := range facts {
			factIDs = append(factIDs, factID)
		}
		sort.Strings(factIDs)
		for _, factID := range factIDs {
			fact := facts[factID]
			chapter := 0
			if event, ok := state.Events[fact.EvidenceEventID]; ok {
				chapter = event.Chapter
			}
			id := characterID + "\x00" + factID
			candidate := contextKnowledgeCandidate{ID: id, CharacterID: characterID, FactID: factID, Fact: fact, Chapter: chapter}
			candidates = append(candidates, candidate)
			byID[id] = candidate
			docs = append(docs, retrieval.Document{ID: id, Text: factID + "\n" + fact.Statement})
		}
	}

	ordered := make([]contextKnowledgeCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextKnowledgeCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Chapter != remaining[j].Chapter {
			return remaining[i].Chapter > remaining[j].Chapter
		}
		if remaining[i].CharacterID != remaining[j].CharacterID {
			return remaining[i].CharacterID < remaining[j].CharacterID
		}
		return remaining[i].FactID < remaining[j].FactID
	})
	ordered = append(ordered, remaining...)

	selected := map[string][]map[string]string{}
	for _, candidate := range ordered {
		characterID := compactContextString(candidate.CharacterID)
		factID := compactContextString(candidate.FactID)
		if characterID == "" || factID == "" {
			continue
		}
		item := map[string]string{"fact_id": factID}
		if statement := compactContextString(candidate.Fact.Statement); statement != "" {
			item["statement"] = statement
		}
		selected[characterID] = append(selected[characterID], item)
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			items := selected[characterID]
			items = items[:len(items)-1]
			if len(items) == 0 {
				delete(selected, characterID)
			} else {
				selected[characterID] = items
			}
		}
	}
	return selected
}

type contextLocationCandidate struct {
	CharacterID string
	Value       domain.CoreLocationState
}

func compactLocationsForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]any {
	if maxBytes <= 0 || len(state.Locations) == 0 {
		return nil
	}
	characterIDs := make([]string, 0, len(state.Locations))
	for characterID := range state.Locations {
		characterIDs = append(characterIDs, characterID)
	}
	sort.Strings(characterIDs)
	candidates := make([]contextLocationCandidate, 0, len(characterIDs))
	byID := map[string]contextLocationCandidate{}
	docs := make([]retrieval.Document, 0, len(characterIDs))
	for _, characterID := range characterIDs {
		value := state.Locations[characterID]
		candidate := contextLocationCandidate{CharacterID: characterID, Value: value}
		candidates = append(candidates, candidate)
		byID[characterID] = candidate
		docs = append(docs, retrieval.Document{ID: characterID, Text: characterID + "\n" + value.LocationID})
	}
	ordered := make([]contextLocationCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.CharacterID] = true
	}
	remaining := make([]contextLocationCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.CharacterID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Value.EndTick != remaining[j].Value.EndTick {
			return remaining[i].Value.EndTick > remaining[j].Value.EndTick
		}
		return remaining[i].CharacterID < remaining[j].CharacterID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]any{}
	for _, candidate := range ordered {
		characterID := compactContextString(candidate.CharacterID)
		locationID := compactContextString(candidate.Value.LocationID)
		if characterID == "" || locationID == "" {
			continue
		}
		selected[characterID] = map[string]any{
			"location_id": locationID,
			"start_tick":  candidate.Value.StartTick,
			"end_tick":    candidate.Value.EndTick,
		}
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, characterID)
		}
	}
	return selected
}

type contextResourceCandidate struct {
	ID    string
	Value int64
}

func compactResourcesForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]int64 {
	if maxBytes <= 0 || len(state.Resources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(state.Resources))
	for id := range state.Resources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	candidates := make([]contextResourceCandidate, 0, len(ids))
	byID := map[string]contextResourceCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		value := state.Resources[id]
		candidate := contextResourceCandidate{ID: id, Value: value}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		text := id
		if entity, ok := state.Entities[id]; ok {
			text += "\n" + entity.Name + "\n" + entity.Description
		}
		docs = append(docs, retrieval.Document{ID: id, Text: text})
	}
	ordered := make([]contextResourceCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			ordered = append(ordered, candidate)
		}
	}

	selected := map[string]int64{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		selected[id] = candidate.Value
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

type contextRelationshipCandidate struct {
	ID      string
	Value   domain.CoreEvidenceState
	Chapter int
}

func compactRelationshipsForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]any {
	if maxBytes <= 0 || len(state.Relationships) == 0 {
		return nil
	}
	ids := make([]string, 0, len(state.Relationships))
	for id := range state.Relationships {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	candidates := make([]contextRelationshipCandidate, 0, len(ids))
	byID := map[string]contextRelationshipCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		value := state.Relationships[id]
		chapter := 0
		if event, ok := state.Events[value.EvidenceEventID]; ok {
			chapter = event.Chapter
		}
		candidate := contextRelationshipCandidate{ID: id, Value: value, Chapter: chapter}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		docs = append(docs, retrieval.Document{ID: id, Text: id + "\n" + strings.Join(value.Tags, "\n")})
	}
	ordered := make([]contextRelationshipCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextRelationshipCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Chapter != remaining[j].Chapter {
			return remaining[i].Chapter > remaining[j].Chapter
		}
		return remaining[i].ID < remaining[j].ID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]any{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		tags := make([]string, 0, len(candidate.Value.Tags))
		for _, tag := range candidate.Value.Tags {
			if compact := compactContextString(tag); compact != "" {
				tags = append(tags, compact)
			}
		}
		selected[id] = map[string]any{"tags": tags}
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

type contextForeshadowCandidate struct {
	ID       string
	Value    domain.CoreForeshadowState
	Chapter  int
	Priority int
}

func compactForeshadowsForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]any {
	if maxBytes <= 0 {
		return nil
	}
	ids := make([]string, 0, len(state.Foreshadows))
	for id, value := range state.Foreshadows {
		if value.State != "closed" && value.State != "retired" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	candidates := make([]contextForeshadowCandidate, 0, len(ids))
	byID := map[string]contextForeshadowCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		value := state.Foreshadows[id]
		chapter := 0
		if event, ok := state.Events[value.EvidenceEventID]; ok {
			chapter = event.Chapter
		}
		candidate := contextForeshadowCandidate{ID: id, Value: value, Chapter: chapter, Priority: foreshadowContextPriority(value.State)}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		docs = append(docs, retrieval.Document{ID: id, Text: id + "\n" + value.Description + "\n" + value.State})
	}
	ordered := make([]contextForeshadowCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextForeshadowCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Priority != remaining[j].Priority {
			return remaining[i].Priority > remaining[j].Priority
		}
		if remaining[i].Chapter != remaining[j].Chapter {
			return remaining[i].Chapter > remaining[j].Chapter
		}
		return remaining[i].ID < remaining[j].ID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]any{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		item := map[string]any{"state": compactContextString(candidate.Value.State)}
		if description := compactContextString(candidate.Value.Description); description != "" {
			item["description"] = description
		}
		selected[id] = item
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

func foreshadowContextPriority(state string) int {
	switch state {
	case "paid_off":
		return 6
	case "payoff_ready":
		return 5
	case "reinforced":
		return 4
	case "misdirected":
		return 3
	case "seeded":
		return 2
	case "planned":
		return 1
	default:
		return 0
	}
}

type contextPromiseCandidate struct {
	ID    string
	Value domain.CoreReaderPromiseState
}

func compactReaderPromisesForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]any {
	if maxBytes <= 0 {
		return nil
	}
	ids := make([]string, 0, len(state.ReaderPromises))
	for id, value := range state.ReaderPromises {
		if value.State != "fulfilled" && value.State != "retired" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	candidates := make([]contextPromiseCandidate, 0, len(ids))
	byID := map[string]contextPromiseCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		value := state.ReaderPromises[id]
		candidate := contextPromiseCandidate{ID: id, Value: value}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		docs = append(docs, retrieval.Document{ID: id, Text: id + "\n" + value.Statement + "\n" + value.State})
	}
	ordered := make([]contextPromiseCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextPromiseCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		left := remaining[i].Value.DeadlineChapter
		right := remaining[j].Value.DeadlineChapter
		if left == 0 {
			left = int(^uint(0) >> 1)
		}
		if right == 0 {
			right = int(^uint(0) >> 1)
		}
		if left != right {
			return left < right
		}
		return remaining[i].ID < remaining[j].ID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]any{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		item := map[string]any{"state": compactContextString(candidate.Value.State)}
		if statement := compactContextString(candidate.Value.Statement); statement != "" {
			item["statement"] = statement
		}
		if candidate.Value.DeadlineChapter > 0 {
			item["deadline_chapter"] = candidate.Value.DeadlineChapter
		}
		selected[id] = item
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

type contextConflictCandidate struct {
	ID       string
	Value    domain.CoreConflictState
	Chapter  int
	Priority int
}

func compactConflictsForContext(state domain.CoreLongformState, query string, maxBytes int) map[string]map[string]any {
	if maxBytes <= 0 {
		return nil
	}
	ids := make([]string, 0, len(state.Conflicts))
	for id, value := range state.Conflicts {
		if value.State == "open" || value.State == "escalated" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	candidates := make([]contextConflictCandidate, 0, len(ids))
	byID := map[string]contextConflictCandidate{}
	docs := make([]retrieval.Document, 0, len(ids))
	for _, id := range ids {
		value := state.Conflicts[id]
		chapter := 0
		if event, ok := state.Events[value.EvidenceEventID]; ok {
			chapter = event.Chapter
		}
		priority := 1
		if value.State == "escalated" {
			priority = 2
		}
		candidate := contextConflictCandidate{ID: id, Value: value, Chapter: chapter, Priority: priority}
		candidates = append(candidates, candidate)
		byID[id] = candidate
		docs = append(docs, retrieval.Document{ID: id, Text: id + "\n" + value.Description + "\n" + value.State + "\n" + value.EscalationCondition + "\n" + value.CloseCondition})
	}
	ordered := make([]contextConflictCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byID[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.ID] = true
	}
	remaining := make([]contextConflictCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.ID] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Priority != remaining[j].Priority {
			return remaining[i].Priority > remaining[j].Priority
		}
		if remaining[i].Chapter != remaining[j].Chapter {
			return remaining[i].Chapter > remaining[j].Chapter
		}
		return remaining[i].ID < remaining[j].ID
	})
	ordered = append(ordered, remaining...)

	selected := map[string]map[string]any{}
	for _, candidate := range ordered {
		id := compactContextString(candidate.ID)
		if id == "" {
			continue
		}
		participants := make([]string, 0, len(candidate.Value.Participants))
		for _, participant := range candidate.Value.Participants {
			if compact := compactContextString(participant); compact != "" {
				participants = append(participants, compact)
			}
		}
		item := map[string]any{
			"state":        compactContextString(candidate.Value.State),
			"participants": participants,
		}
		if description := compactContextString(candidate.Value.Description); description != "" {
			item["description"] = description
		}
		if condition := compactContextString(candidate.Value.EscalationCondition); condition != "" {
			item["escalation_condition"] = condition
		}
		if condition := compactContextString(candidate.Value.CloseCondition); condition != "" {
			item["close_condition"] = condition
		}
		selected[id] = item
		raw, err := json.Marshal(selected)
		if err != nil || len(raw) > maxBytes {
			delete(selected, id)
		}
	}
	return selected
}

type contextTravelCandidate struct {
	Key     string
	Value   domain.CoreTravelConstraint
	Current bool
}

func compactTravelConstraintsForContext(state domain.CoreLongformState, query string, maxBytes int) []map[string]any {
	if maxBytes <= 0 || len(state.TravelConstraints) == 0 {
		return nil
	}
	currentLocations := map[string]bool{}
	for _, location := range state.Locations {
		if id := compactContextString(location.LocationID); id != "" {
			currentLocations[id] = true
		}
	}
	candidates := make([]contextTravelCandidate, 0, len(state.TravelConstraints))
	for _, value := range state.TravelConstraints {
		from := compactContextString(value.FromLocationID)
		to := compactContextString(value.ToLocationID)
		if from == "" || to == "" {
			continue
		}
		candidates = append(candidates, contextTravelCandidate{
			Key: from + "→" + to, Value: domain.CoreTravelConstraint{FromLocationID: from, ToLocationID: to, MinTicks: value.MinTicks},
			Current: currentLocations[from] || currentLocations[to],
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Value.FromLocationID != candidates[j].Value.FromLocationID {
			return candidates[i].Value.FromLocationID < candidates[j].Value.FromLocationID
		}
		if candidates[i].Value.ToLocationID != candidates[j].Value.ToLocationID {
			return candidates[i].Value.ToLocationID < candidates[j].Value.ToLocationID
		}
		return candidates[i].Value.MinTicks < candidates[j].Value.MinTicks
	})
	byKey := map[string]contextTravelCandidate{}
	docs := make([]retrieval.Document, 0, len(candidates))
	for _, candidate := range candidates {
		byKey[candidate.Key] = candidate
		docs = append(docs, retrieval.Document{ID: candidate.Key, Text: candidate.Value.FromLocationID + "\n" + candidate.Value.ToLocationID})
	}
	ordered := make([]contextTravelCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		candidate := byKey[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.Key] = true
	}
	remaining := make([]contextTravelCandidate, 0, len(candidates)-len(ordered))
	for _, candidate := range candidates {
		if !seen[candidate.Key] {
			remaining = append(remaining, candidate)
		}
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if remaining[i].Current != remaining[j].Current {
			return remaining[i].Current
		}
		return remaining[i].Key < remaining[j].Key
	})
	ordered = append(ordered, remaining...)

	selected := make([]map[string]any, 0, len(ordered))
	for _, candidate := range ordered {
		item := map[string]any{
			"from_location_id": candidate.Value.FromLocationID,
			"to_location_id":   candidate.Value.ToLocationID,
			"min_ticks":        candidate.Value.MinTicks,
		}
		candidateList := append(selected, item)
		raw, err := json.Marshal(candidateList)
		if err != nil || len(raw) > maxBytes {
			continue
		}
		selected = candidateList
	}
	return selected
}

func compactArcPlanForContext(arc domain.CoreArcPlan) map[string]any {
	out := map[string]any{
		"id":            compactContextString(arc.ID),
		"start_chapter": arc.StartChapter,
		"end_chapter":   arc.EndChapter,
		"goal":          compactContextString(arc.Goal),
	}
	if arc.PlanningLeadChapters > 0 {
		out["planning_lead_chapters"] = arc.PlanningLeadChapters
	}
	return out
}

func (p *Project) compactFoundationReference(readMap func(string) (map[string]any, error)) (map[string]any, error) {
	foundation, err := readMap("foundation.json")
	if err != nil {
		return nil, err
	}
	characters, err := readMap("characters.json")
	if err != nil {
		return nil, err
	}
	world, err := readMap("world.json")
	if err != nil {
		return nil, err
	}
	style, err := readMap("style_profile.json")
	if err != nil {
		return nil, err
	}
	platform, err := readMap("platform_profile.json")
	if err != nil {
		return nil, err
	}

	foundationOut := map[string]any{}
	for _, key := range []string{"title", "protagonist", "opening_location"} {
		if value, ok := foundation[key]; ok {
			foundationOut[key] = compactFoundationValue(value)
		}
	}

	characterItems := make([]any, 0)
	if raw, ok := characters["characters"].([]any); ok {
		for _, value := range raw {
			if item, ok := value.(map[string]any); ok {
				characterItems = append(characterItems, compactFoundationEntity(item, false))
			}
		}
	}

	worldItems := make([]any, 0)
	if raw, ok := world["entities"].([]any); ok {
		for _, value := range raw {
			if item, ok := value.(map[string]any); ok {
				worldItems = append(worldItems, compactFoundationEntity(item, true))
			}
		}
	}

	return map[string]any{
		"foundation":       foundationOut,
		"characters":       map[string]any{"characters": characterItems},
		"world":            map[string]any{"entities": worldItems},
		"style_profile":    map[string]any{"language": compactContextString(style["language"])},
		"platform_profile": map[string]any{"platform": compactContextString(platform["platform"])},
	}, nil
}

func compactFoundationEntity(item map[string]any, includeType bool) map[string]any {
	out := map[string]any{}
	if includeType {
		if value := compactContextString(item["entity_type"]); value != "" {
			out["entity_type"] = value
		}
	}
	if value := compactContextString(item["canon_id"]); value != "" {
		out["canon_id"] = value
	}
	if value := compactContextString(item["name"]); value != "" {
		out["name"] = value
	}
	return out
}

func compactFoundationValue(value any) any {
	switch x := value.(type) {
	case string:
		return compactContextString(x)
	case map[string]any:
		out := map[string]any{}
		for _, key := range []string{"entity_type", "canon_id", "name"} {
			if v := compactContextString(x[key]); v != "" {
				out[key] = v
			}
		}
		return out
	default:
		return nil
	}
}

type contextFoundationCandidate struct {
	Key        string
	Collection string
	Item       map[string]any
	Mandatory  bool
}

func compactFoundationReferenceForContext(reference map[string]any, query string, maxBytes int) map[string]any {
	foundation, _ := reference["foundation"].(map[string]any)
	style, _ := reference["style_profile"].(map[string]any)
	platform, _ := reference["platform_profile"].(map[string]any)
	out := map[string]any{
		"foundation":       foundation,
		"characters":       map[string]any{"characters": []any{}},
		"world":            map[string]any{"entities": []any{}},
		"style_profile":    style,
		"platform_profile": platform,
	}
	if maxBytes <= 0 {
		return out
	}
	mandatoryIDs := map[string]bool{}
	for _, key := range []string{"protagonist", "opening_location"} {
		if ref, ok := foundation[key].(map[string]any); ok {
			if id := compactContextString(ref["canon_id"]); id != "" {
				mandatoryIDs[id] = true
			}
		}
	}
	candidates := make([]contextFoundationCandidate, 0)
	appendCandidates := func(collection string, raw any) {
		items, _ := raw.([]any)
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id := compactContextString(item["canon_id"])
			if id == "" {
				continue
			}
			copyItem := map[string]any{"canon_id": id}
			if name := compactContextString(item["name"]); name != "" {
				copyItem["name"] = name
			}
			if collection == "world" {
				if entityType := compactContextString(item["entity_type"]); entityType != "" {
					copyItem["entity_type"] = entityType
				}
			}
			candidates = append(candidates, contextFoundationCandidate{
				Key: collection + ":" + id, Collection: collection, Item: copyItem, Mandatory: mandatoryIDs[id],
			})
		}
	}
	if characters, ok := reference["characters"].(map[string]any); ok {
		appendCandidates("characters", characters["characters"])
	}
	if world, ok := reference["world"].(map[string]any); ok {
		appendCandidates("world", world["entities"])
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Key < candidates[j].Key })
	byKey := map[string]contextFoundationCandidate{}
	docs := make([]retrieval.Document, 0, len(candidates))
	ordered := make([]contextFoundationCandidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		byKey[candidate.Key] = candidate
		text := candidate.Key + "\n" + compactContextString(candidate.Item["name"]) + "\n" + compactContextString(candidate.Item["entity_type"])
		docs = append(docs, retrieval.Document{ID: candidate.Key, Text: text})
		if candidate.Mandatory {
			ordered = append(ordered, candidate)
			seen[candidate.Key] = true
		}
	}
	for _, hit := range retrieval.RankKeyword(docs, query, len(docs)) {
		if seen[hit.ID] {
			continue
		}
		candidate := byKey[hit.ID]
		ordered = append(ordered, candidate)
		seen[candidate.Key] = true
	}
	for _, candidate := range candidates {
		if !seen[candidate.Key] {
			ordered = append(ordered, candidate)
		}
	}

	characterItems := []any{}
	worldItems := []any{}
	for _, candidate := range ordered {
		if candidate.Collection == "characters" {
			characterItems = append(characterItems, candidate.Item)
		} else {
			worldItems = append(worldItems, candidate.Item)
		}
		out["characters"] = map[string]any{"characters": characterItems}
		out["world"] = map[string]any{"entities": worldItems}
		raw, err := json.Marshal(out)
		if err == nil && (len(raw) <= maxBytes || candidate.Mandatory) {
			continue
		}
		if candidate.Collection == "characters" {
			characterItems = characterItems[:len(characterItems)-1]
		} else {
			worldItems = worldItems[:len(worldItems)-1]
		}
		out["characters"] = map[string]any{"characters": characterItems}
		out["world"] = map[string]any{"entities": worldItems}
	}
	return out
}

func compactContextString(value any) string {
	s, _ := value.(string)
	return string(truncateUTF8([]byte(strings.TrimSpace(s)), 256))
}

func (p *Project) compileTaskPackContext(state *domain.CoreProductionState) (contextCompilerOutput, error) {
	raw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return contextCompilerOutput{}, err
	}
	var canon domain.CoreCanonState
	if err := protocol.DecodeJSON(raw, &canon); err != nil {
		return contextCompilerOutput{}, err
	}
	head, err := p.store.LoadCoreCanonHead()
	if err != nil || head == nil {
		if err == nil {
			err = fmt.Errorf("canon head does not exist")
		}
		return contextCompilerOutput{}, err
	}
	var revisionCandidate map[string]any
	if state.ActiveTask != nil && state.ActiveTask.Kind == "revision" {
		baseCanon, baseHead, err := p.loadCanonSnapshotAtRoot(state.ActiveTask.BaseCanonRoot)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		canon = baseCanon
		head = &baseHead
		chapter, err := chapterNumberFromTarget(state.ActiveTask.Target)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		candidate, err := p.revisionCandidate(state, chapter)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		revisionCandidate = map[string]any{"chapter": chapter, "chapter_md": string(candidate)}
	}
	readMap := func(name string) (map[string]any, error) {
		data, err := p.store.ReadCoreCanonArtifact(name)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		if err := protocol.DecodeJSON(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	bookPlan, err := readMap("book_plan.json")
	if err != nil {
		return contextCompilerOutput{}, err
	}
	if canon.Planning.CurrentArc.ID != "" {
		bookPlan["current_arc"] = canon.Planning.CurrentArc
		if canon.Planning.NextArc != nil {
			bookPlan["next_arc"] = canon.Planning.NextArc
		} else {
			delete(bookPlan, "next_arc")
		}
	}
	ending, err := readMap("ending_contract.json")
	if err != nil {
		return contextCompilerOutput{}, err
	}
	foundationReference, err := p.compactFoundationReference(readMap)
	if err != nil {
		return contextCompilerOutput{}, err
	}
	chapters := make([]contextChapter, 0)
	for name := range head.ArtifactDigests {
		clean := filepath.ToSlash(name)
		if !strings.HasPrefix(clean, "chapters/") || !strings.HasSuffix(clean, "/chapter.md") {
			continue
		}
		parts := strings.Split(clean, "/")
		if len(parts) != 3 {
			continue
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil || n <= 0 {
			continue
		}
		data, err := p.store.ReadCoreCanonArtifact(name)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		chapters = append(chapters, contextChapter{Chapter: n, Path: clean, Text: string(data)})
	}
	input := contextCompilerInput{
		Target: state.ActiveTask.Target, CanonRoot: state.ActiveTask.BaseCanonRoot,
		EndingContract: ending, BookPlan: bookPlan, FoundationReference: foundationReference,
		Constraints: state.ActiveTask.Constraints, CanonState: canon, Chapters: chapters,
		RevisionCandidate: revisionCandidate,
	}
	return compileTaskContext(input, taskContextBudget)
}
