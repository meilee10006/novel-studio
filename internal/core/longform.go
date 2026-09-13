package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func validateLongformState(current domain.CoreLongformState, canonical map[string][]byte, chapter int) (domain.CoreLongformState, []string, error) {
	next := cloneLongformState(current)
	ensureLongformMaps(&next)
	currentEvents, err := decodeCurrentEvents(canonical["events.json"], chapter)
	if err != nil {
		return domain.CoreLongformState{}, nil, err
	}
	for id, event := range currentEvents {
		next.Events[id] = event
	}
	var delta map[string]any
	if err := protocol.DecodeJSON(canonical["state_delta.json"], &delta); err != nil {
		return domain.CoreLongformState{}, nil, err
	}
	changes, _ := delta["changes"].([]any)
	var violations []string
	for _, raw := range changes {
		change, ok := raw.(map[string]any)
		if !ok {
			violations = append(violations, "state change must be an object")
			continue
		}
		applyLongformChange(&next, change, chapter, &violations)
	}
	sort.Strings(violations)
	return next, violations, nil
}
func decodeCurrentEvents(raw []byte, chapter int) (map[string]domain.CoreEventEvidence, error) {
	var root map[string]any
	if err := protocol.DecodeJSON(raw, &root); err != nil {
		return nil, err
	}
	items, _ := root["events"].([]any)
	out := make(map[string]domain.CoreEventEvidence, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		id, _ := item["canon_id"].(string)
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = domain.CoreEventEvidence{
			EventID: id, Chapter: chapter,
			Observers: stringSlice(item["observers"]), Actors: stringSlice(item["actors"]),
		}
	}
	return out, nil
}

func applyLongformChange(state *domain.CoreLongformState, change map[string]any, chapter int, violations *[]string) {
	kind, _ := change["kind"].(string)
	switch strings.TrimSpace(kind) {
	case "knowledge_add":
		applyKnowledgeChange(state, change, violations)
	case "resource":
		applyResourceChange(state, change, violations)
	case "location":
		applyLocationChange(state, change, violations)
	case "relationship":
		applyRelationshipChange(state, change, violations)
	case "foreshadow":
		applyForeshadowChange(state, change, violations)
	case "reader_promise":
		applyReaderPromiseChange(state, change, chapter, violations)
	}
}
func applyKnowledgeChange(state *domain.CoreLongformState, change map[string]any, violations *[]string) {
	characterID := cleanString(change["character_id"])
	factID := cleanString(change["fact_id"])
	if characterID == "" || factID == "" {
		*violations = append(*violations, "knowledge change requires character_id and fact_id")
		return
	}
	source, ok := change["source"].(map[string]any)
	if !ok {
		*violations = append(*violations, "knowledge change requires a legal source")
		return
	}
	kind := cleanString(source["kind"])
	eventID := cleanString(source["event_canon_id"])
	event, exists := state.Events[eventID]
	switch kind {
	case "observed":
		if !exists || !containsString(event.Observers, characterID) {
			*violations = append(*violations, "knowledge observed source is not evidenced for character")
			return
		}
	case "transmitted":
		from := cleanString(source["from_character_id"])
		if !exists || from == "" || !containsString(event.Observers, characterID) {
			*violations = append(*violations, "knowledge transmission source is not evidenced")
			return
		}
		if _, ok := state.Knowledge[from][factID]; !ok {
			*violations = append(*violations, "knowledge transmission source character does not know fact")
			return
		}
	default:
		*violations = append(*violations, "knowledge source kind is unsupported")
		return
	}
	if state.Knowledge[characterID] == nil {
		state.Knowledge[characterID] = map[string]domain.CoreKnowledgeFact{}
	}
	state.Knowledge[characterID][factID] = domain.CoreKnowledgeFact{
		SourceKind: kind, EvidenceEventID: eventID, FromCharacterID: cleanString(source["from_character_id"]),
	}
}

func applyResourceChange(state *domain.CoreLongformState, change map[string]any, violations *[]string) {
	resourceID := cleanString(change["resource_id"])
	delta, ok := integer64(change["delta"])
	if resourceID == "" || !ok {
		*violations = append(*violations, "resource change requires resource_id and integer delta")
		return
	}
	if !requireEvidence(state, change, "resource", violations) {
		return
	}
	next := state.Resources[resourceID] + delta
	if next < 0 {
		*violations = append(*violations, "resource balance cannot become negative: "+resourceID)
		return
	}
	state.Resources[resourceID] = next
}
func applyLocationChange(state *domain.CoreLongformState, change map[string]any, violations *[]string) {
	characterID := cleanString(change["character_id"])
	if characterID == "" {
		return
	}
	locationID := cleanString(change["location_id"])
	start, okStart := integer64(change["start_tick"])
	end, okEnd := integer64(change["end_tick"])
	if locationID == "" || !okStart || !okEnd || end < start {
		*violations = append(*violations, "location change requires location_id and valid start/end ticks")
		return
	}
	if !requireEvidence(state, change, "location", violations) {
		return
	}
	prev, exists := state.Locations[characterID]
	if exists && prev.LocationID != locationID {
		if start < prev.EndTick {
			*violations = append(*violations, "location intervals overlap for character: "+characterID)
			return
		}
		if minTicks, modeled := modeledTravelMinTicks(state, prev.LocationID, locationID); modeled && start-prev.EndTick < minTicks {
			*violations = append(*violations, fmt.Sprintf("modeled travel constraint requires at least %d ticks: %s -> %s", minTicks, prev.LocationID, locationID))
			return
		}
	}
	state.Locations[characterID] = domain.CoreLocationState{
		LocationID: locationID, StartTick: start, EndTick: end,
		EvidenceEventID: cleanString(change["event_canon_id"]),
	}
}
func applyRelationshipChange(state *domain.CoreLongformState, change map[string]any, violations *[]string) {
	id := cleanString(change["relationship_id"])
	if id == "" {
		*violations = append(*violations, "relationship change requires relationship_id")
		return
	}
	if !requireEvidence(state, change, "relationship", violations) {
		return
	}
	state.Relationships[id] = domain.CoreEvidenceState{
		Tags: stringSlice(change["tags"]), EvidenceEventID: cleanString(change["event_canon_id"]),
	}
}

func applyForeshadowChange(state *domain.CoreLongformState, change map[string]any, violations *[]string) {
	id := cleanString(change["foreshadow_id"])
	nextState := cleanString(change["state"])
	if id == "" || nextState == "" {
		*violations = append(*violations, "foreshadow change requires foreshadow_id and state")
		return
	}
	if !requireEvidence(state, change, "foreshadow", violations) {
		return
	}
	prev := state.Foreshadows[id].State
	if !validForeshadowTransition(prev, nextState) {
		*violations = append(*violations, fmt.Sprintf("illegal foreshadow transition %q -> %q", prev, nextState))
		return
	}
	state.Foreshadows[id] = domain.CoreForeshadowState{State: nextState, EvidenceEventID: cleanString(change["event_canon_id"])}
}
func applyReaderPromiseChange(state *domain.CoreLongformState, change map[string]any, chapter int, violations *[]string) {
	id := cleanString(change["promise_id"])
	nextState := cleanString(change["state"])
	if id == "" || nextState == "" {
		*violations = append(*violations, "reader promise change requires promise_id and state")
		return
	}
	if nextState != "advanced" && nextState != "fulfilled" && nextState != "deferred" && nextState != "retired" {
		*violations = append(*violations, "reader promise state is unsupported: "+nextState)
		return
	}
	evidenceID := cleanString(change["event_canon_id"])
	if nextState == "fulfilled" && !requireEvidence(state, change, "reader promise", violations) {
		return
	}
	deadline := 0
	if nextState == "deferred" {
		v, ok := integer64(change["deadline_chapter"])
		if !ok || v <= int64(chapter) {
			*violations = append(*violations, "deferred reader promise requires a future deadline_chapter")
			return
		}
		deadline = int(v)
	}
	state.ReaderPromises[id] = domain.CoreReaderPromiseState{State: nextState, EvidenceEventID: evidenceID, DeadlineChapter: deadline}
}

func requireEvidence(state *domain.CoreLongformState, change map[string]any, label string, violations *[]string) bool {
	eventID := cleanString(change["event_canon_id"])
	if eventID == "" {
		*violations = append(*violations, label+" change requires evidence event")
		return false
	}
	if _, ok := state.Events[eventID]; !ok {
		*violations = append(*violations, label+" evidence event is not accepted")
		return false
	}
	return true
}
func validForeshadowTransition(prev, next string) bool {
	allowed := map[string]map[string]bool{
		"":             {"planned": true, "seeded": true},
		"planned":      {"seeded": true, "retired": true},
		"seeded":       {"reinforced": true, "payoff_ready": true, "misdirected": true, "retired": true},
		"reinforced":   {"reinforced": true, "payoff_ready": true, "misdirected": true, "retired": true},
		"misdirected":  {"reinforced": true, "payoff_ready": true, "retired": true},
		"payoff_ready": {"paid_off": true, "retired": true},
		"paid_off":     {"closed": true},
	}
	return allowed[prev][next]
}

func cleanString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func integer64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		i := int64(n)
		return i, float64(i) == n
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}
func stringSlice(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := cleanString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func cloneLongformState(in domain.CoreLongformState) domain.CoreLongformState {
	data, _ := json.Marshal(in)
	var out domain.CoreLongformState
	_ = json.Unmarshal(data, &out)
	ensureLongformMaps(&out)
	return out
}

func ensureLongformMaps(state *domain.CoreLongformState) {
	if state.Events == nil {
		state.Events = map[string]domain.CoreEventEvidence{}
	}
	if state.Knowledge == nil {
		state.Knowledge = map[string]map[string]domain.CoreKnowledgeFact{}
	}
	if state.Locations == nil {
		state.Locations = map[string]domain.CoreLocationState{}
	}
	if state.Resources == nil {
		state.Resources = map[string]int64{}
	}
	if state.Relationships == nil {
		state.Relationships = map[string]domain.CoreEvidenceState{}
	}
	if state.Foreshadows == nil {
		state.Foreshadows = map[string]domain.CoreForeshadowState{}
	}
	if state.ReaderPromises == nil {
		state.ReaderPromises = map[string]domain.CoreReaderPromiseState{}
	}
}

func foundationLongformState(worldRaw []byte) (domain.CoreLongformState, []string, error) {
	state := domain.CoreLongformState{}
	ensureLongformMaps(&state)
	var world map[string]any
	if err := protocol.DecodeJSON(worldRaw, &world); err != nil {
		return domain.CoreLongformState{}, nil, err
	}
	items, ok := world["travel_constraints"]
	if !ok {
		return state, nil, nil
	}
	list, ok := items.([]any)
	if !ok {
		return state, []string{"world.travel_constraints must be an array"}, nil
	}
	seen := map[string]bool{}
	var violations []string
	for _, raw := range list {
		item, ok := raw.(map[string]any)
		if !ok {
			violations = append(violations, "travel constraint must be an object")
			continue
		}
		from := travelNestedCanonID(item["from"])
		to := travelNestedCanonID(item["to"])
		minTicks, validTicks := integer64(item["min_ticks"])
		if from == "" || to == "" || !validTicks || minTicks <= 0 {
			violations = append(violations, "travel constraint requires from/to canon ids and positive min_ticks")
			continue
		}
		key := from + "\x00" + to
		if seen[key] {
			violations = append(violations, "duplicate travel constraint: "+from+" -> "+to)
			continue
		}
		seen[key] = true
		state.TravelConstraints = append(state.TravelConstraints, domain.CoreTravelConstraint{
			FromLocationID: from, ToLocationID: to, MinTicks: minTicks,
		})
	}
	sort.Slice(state.TravelConstraints, func(i, j int) bool {
		a, b := state.TravelConstraints[i], state.TravelConstraints[j]
		if a.FromLocationID != b.FromLocationID {
			return a.FromLocationID < b.FromLocationID
		}
		return a.ToLocationID < b.ToLocationID
	})
	sort.Strings(violations)
	return state, violations, nil
}

func travelNestedCanonID(v any) string {
	m, _ := v.(map[string]any)
	return cleanString(m["canon_id"])
}

func modeledTravelMinTicks(state *domain.CoreLongformState, from, to string) (int64, bool) {
	for _, constraint := range state.TravelConstraints {
		if constraint.FromLocationID == from && constraint.ToLocationID == to {
			return constraint.MinTicks, true
		}
	}
	return 0, false
}
