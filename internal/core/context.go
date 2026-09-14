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
		"state":           input.CanonState,
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
	foundationReference := make(map[string]any, 5)
	for _, item := range []struct {
		Key  string
		Name string
	}{
		{Key: "foundation", Name: "foundation.json"},
		{Key: "characters", Name: "characters.json"},
		{Key: "world", Name: "world.json"},
		{Key: "style_profile", Name: "style_profile.json"},
		{Key: "platform_profile", Name: "platform_profile.json"},
	} {
		value, err := readMap(item.Name)
		if err != nil {
			return contextCompilerOutput{}, err
		}
		foundationReference[item.Key] = value
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
