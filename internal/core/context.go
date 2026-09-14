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
	contextDoc := map[string]any{
		"target":              input.Target,
		"book_plan":           input.BookPlan,
		"ending_contract":     input.EndingContract,
		"control_constraints": input.Constraints,
		"retrieved_old_prose": []map[string]any{},
	}
	if input.FoundationReference != nil {
		contextDoc["foundation_reference"] = input.FoundationReference
	}
	if input.RevisionCandidate != nil {
		contextDoc["revision_candidate"] = input.RevisionCandidate
	}
	canonDoc := map[string]any{
		"base_canon_root": input.CanonRoot,
		"state":           compactCanonStateForContext(input.CanonState),
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
	query := contextQuery(input)
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

func compactCanonStateForContext(state domain.CoreCanonState) map[string]any {
	out := map[string]any{
		"schema_version": state.SchemaVersion,
		"revision":       state.Revision,
		"project_id":     state.ProjectID,
		"latest_chapter": state.LatestChapter,
	}
	longform := map[string]any{}

	knowledge := map[string][]map[string]string{}
	for characterID, facts := range state.Longform.Knowledge {
		factIDs := make([]string, 0, len(facts))
		for factID := range facts {
			factIDs = append(factIDs, factID)
		}
		sort.Strings(factIDs)
		items := make([]map[string]string, 0, len(factIDs))
		for _, factID := range factIDs {
			id := compactContextString(factID)
			if id == "" {
				continue
			}
			item := map[string]string{"fact_id": id}
			if statement := compactContextString(facts[factID].Statement); statement != "" {
				item["statement"] = statement
			}
			items = append(items, item)
		}
		if len(items) > 0 {
			knowledge[compactContextString(characterID)] = items
		}
	}
	if len(knowledge) > 0 {
		longform["knowledge"] = knowledge
	}

	locations := map[string]any{}
	for characterID, value := range state.Longform.Locations {
		locations[compactContextString(characterID)] = map[string]any{
			"location_id": compactContextString(value.LocationID),
			"start_tick":  value.StartTick,
			"end_tick":    value.EndTick,
		}
	}
	if len(locations) > 0 {
		longform["locations"] = locations
	}

	if len(state.Longform.Resources) > 0 {
		resources := map[string]int64{}
		for id, value := range state.Longform.Resources {
			resources[compactContextString(id)] = value
		}
		longform["resources"] = resources
	}

	relationships := map[string]any{}
	for id, value := range state.Longform.Relationships {
		tags := make([]string, 0, len(value.Tags))
		for _, tag := range value.Tags {
			if compact := compactContextString(tag); compact != "" {
				tags = append(tags, compact)
			}
		}
		relationships[compactContextString(id)] = map[string]any{"tags": tags}
	}
	if len(relationships) > 0 {
		longform["relationships"] = relationships
	}

	foreshadows := map[string]any{}
	for id, value := range state.Longform.Foreshadows {
		if value.State == "closed" || value.State == "retired" {
			continue
		}
		foreshadows[compactContextString(id)] = map[string]any{"state": compactContextString(value.State)}
	}
	if len(foreshadows) > 0 {
		longform["foreshadows"] = foreshadows
	}

	promises := map[string]any{}
	for id, value := range state.Longform.ReaderPromises {
		if value.State == "fulfilled" || value.State == "retired" {
			continue
		}
		item := map[string]any{"state": compactContextString(value.State)}
		if value.DeadlineChapter > 0 {
			item["deadline_chapter"] = value.DeadlineChapter
		}
		promises[compactContextString(id)] = item
	}
	if len(promises) > 0 {
		longform["reader_promises"] = promises
	}

	if len(state.Longform.TravelConstraints) > 0 {
		travel := make([]map[string]any, 0, len(state.Longform.TravelConstraints))
		for _, value := range state.Longform.TravelConstraints {
			travel = append(travel, map[string]any{
				"from_location_id": compactContextString(value.FromLocationID),
				"to_location_id":   compactContextString(value.ToLocationID),
				"min_ticks":        value.MinTicks,
			})
		}
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
