package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func TestContextCompilerBudgetDoesNotGrowWithChapterCount(t *testing.T) {
	const budget = 16 << 10
	for _, count := range []int{100, 200} {
		out, err := compileTaskContext(contextCompilerInput{
			Target: "chapter:201", CanonRoot: "root",
			EndingContract: map[string]any{"main_resolution": "主线必须收束"},
			BookPlan:       map[string]any{"direction": "持续推进主线"},
			CanonState:     domain.CoreCanonState{Longform: domain.CoreLongformState{}},
			Chapters:       makeContextChapters(count),
		}, budget)
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalBytes > budget {
			t.Fatalf("count=%d bytes=%d budget=%d", count, out.TotalBytes, budget)
		}
	}
}
func TestContextCompilerKeepsHardContractsAheadOfOldProse(t *testing.T) {
	const budget = 8 << 10
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:9", CanonRoot: "root",
		EndingContract: map[string]any{"main_resolution": "必须保留的结局硬约束"},
		BookPlan:       map[string]any{"direction": "必须保留的整书方向"},
		Constraints:    []domain.CoreTaskConstraint{{MessageID: "c1", Instruction: "必须保留的作者约束"}},
		CanonState:     domain.CoreCanonState{Longform: domain.CoreLongformState{}},
		Chapters:       makeContextChapters(80),
	}, budget)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(append(append([]byte{}, out.ContextJSON...), out.CanonExcerptJSON...))
	for _, want := range []string{"必须保留的结局硬约束", "必须保留的整书方向", "必须保留的作者约束"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("hard context lost %q", want)
		}
	}
}

func TestContextCompilerRejectsHardContextLargerThanBudget(t *testing.T) {
	_, err := compileTaskContext(contextCompilerInput{
		EndingContract: map[string]any{"main_resolution": strings.Repeat("硬约束", 2000)},
		BookPlan:       map[string]any{"direction": "x"},
		CanonState:     domain.CoreCanonState{},
	}, 1024)
	if err == nil || !strings.Contains(err.Error(), "hard context") {
		t.Fatalf("err=%v", err)
	}
}
func TestContextCompilerRetrievesRelevantOldProseDeterministically(t *testing.T) {
	chapters := makeContextChapters(8)
	chapters[0].Text = "第一章：雨夜里出现红色纸伞，这个细节之后无人再提。"
	input := contextCompilerInput{
		Target: "chapter:9", CanonRoot: "root",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "重新呼应红色纸伞"}},
		CanonState:  domain.CoreCanonState{}, Chapters: chapters,
	}
	first, err := compileTaskContext(input, 12<<10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compileTaskContext(input, 12<<10)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.ContextJSON, second.ContextJSON) || !bytes.Equal(first.RecentProse, second.RecentProse) {
		t.Fatal("context compilation is not deterministic")
	}
	if !strings.Contains(string(first.ContextJSON), "红色纸伞") {
		t.Fatalf("context=%s", first.ContextJSON)
	}
}

func TestTaskPackContainsCompiledCanonContext(t *testing.T) {
	_, _, workspace := newChapterReadyProject(t)
	ready := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID)
	contextRaw, err := os.ReadFile(filepath.Join(base, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	canonRaw, err := os.ReadFile(filepath.Join(base, "canon_excerpt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contextRaw), "完成主线") || !strings.Contains(string(contextRaw), "主线得到明确收束") {
		t.Fatalf("context missing foundation contracts: %s", contextRaw)
	}
	if !strings.Contains(string(canonRaw), ready.BaseCanonRoot) {
		t.Fatalf("canon excerpt=%s", canonRaw)
	}
}
func makeContextChapters(count int) []contextChapter {
	out := make([]contextChapter, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, contextChapter{
			Chapter: i,
			Path:    fmt.Sprintf("chapters/%06d/chapter.md", i),
			Text:    fmt.Sprintf("第%d章。常规剧情推进。%s", i, strings.Repeat("正文", 80)),
		})
	}
	return out
}

func TestContextCompilerDoesNotGrowWithHistoricalEventInventory(t *testing.T) {
	const budget = 16 << 10
	events := make(map[string]domain.CoreEventEvidence, 600)
	facts := make(map[string]domain.CoreKnowledgeFact, 120)
	for i := 1; i <= 600; i++ {
		id := fmt.Sprintf("story-event-%06d", i)
		events[id] = domain.CoreEventEvidence{EventID: id, Chapter: i, Observers: []string{"character-000001"}}
		if i <= 120 {
			facts[fmt.Sprintf("fact-%03d", i)] = domain.CoreKnowledgeFact{SourceKind: "observed", EvidenceEventID: id}
		}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 600, ProjectID: "book", LatestChapter: 600,
		Longform: domain.CoreLongformState{
			Events:         events,
			Knowledge:      map[string]map[string]domain.CoreKnowledgeFact{"character-000001": facts},
			Locations:      map[string]domain.CoreLocationState{"character-000001": {LocationID: "location-000002", StartTick: 599, EndTick: 600, EvidenceEventID: "story-event-000600"}},
			Resources:      map[string]int64{"money": 42},
			Relationships:  map[string]domain.CoreEvidenceState{"r-main": {Tags: []string{"互相信任"}, EvidenceEventID: "story-event-000600"}},
			Foreshadows:    map[string]domain.CoreForeshadowState{"fs-active": {State: "payoff_ready", EvidenceEventID: "story-event-000599"}, "fs-closed": {State: "closed", EvidenceEventID: "story-event-000100"}},
			ReaderPromises: map[string]domain.CoreReaderPromiseState{"promise-active": {State: "deferred", DeadlineChapter: 610}, "promise-done": {State: "fulfilled", EvidenceEventID: "story-event-000500"}},
		},
		Planning: domain.CorePlanningState{CurrentArc: domain.CoreArcPlan{ID: "arc-9", StartChapter: 590, EndChapter: 610, Goal: "收束旧线索"}},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:601", CanonRoot: "root-600",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		CanonState: canon,
	}, budget)
	if err != nil {
		t.Fatalf("historical event inventory should not overflow task context: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"fact-120", "location-000002", "money", "r-main", "fs-active", "promise-active", "arc-9"} {
		if !strings.Contains(text, want) {
			t.Errorf("compact canon excerpt lost current state %q: %s", want, text)
		}
	}
	for _, stale := range []string{"story-event-000001", "fs-closed", "promise-done"} {
		if strings.Contains(text, stale) {
			t.Errorf("compact canon excerpt retained historical/terminal state %q", stale)
		}
	}
}
