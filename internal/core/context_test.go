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
			BookPlan: map[string]any{"direction": "持续推进主线"},
			CanonState: domain.CoreCanonState{Longform: domain.CoreLongformState{}},
			Chapters: makeContextChapters(count),
		}, budget)
		if err != nil { t.Fatal(err) }
		if out.TotalBytes > budget { t.Fatalf("count=%d bytes=%d budget=%d", count, out.TotalBytes, budget) }
	}
}
func TestContextCompilerKeepsHardContractsAheadOfOldProse(t *testing.T) {
	const budget = 8 << 10
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:9", CanonRoot: "root",
		EndingContract: map[string]any{"main_resolution": "必须保留的结局硬约束"},
		BookPlan: map[string]any{"direction": "必须保留的整书方向"},
		Constraints: []domain.CoreTaskConstraint{{MessageID: "c1", Instruction: "必须保留的作者约束"}},
		CanonState: domain.CoreCanonState{Longform: domain.CoreLongformState{}},
		Chapters: makeContextChapters(80),
	}, budget)
	if err != nil { t.Fatal(err) }
	joined := string(append(append([]byte{}, out.ContextJSON...), out.CanonExcerptJSON...))
	for _, want := range []string{"必须保留的结局硬约束", "必须保留的整书方向", "必须保留的作者约束"} {
		if !strings.Contains(joined, want) { t.Fatalf("hard context lost %q", want) }
	}
}

func TestContextCompilerRejectsHardContextLargerThanBudget(t *testing.T) {
	_, err := compileTaskContext(contextCompilerInput{
		EndingContract: map[string]any{"main_resolution": strings.Repeat("硬约束", 2000)},
		BookPlan: map[string]any{"direction": "x"},
		CanonState: domain.CoreCanonState{},
	}, 1024)
	if err == nil || !strings.Contains(err.Error(), "hard context") { t.Fatalf("err=%v", err) }
}
func TestContextCompilerRetrievesRelevantOldProseDeterministically(t *testing.T) {
	chapters := makeContextChapters(8)
	chapters[0].Text = "第一章：雨夜里出现红色纸伞，这个细节之后无人再提。"
	input := contextCompilerInput{
		Target: "chapter:9", CanonRoot: "root",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "重新呼应红色纸伞"}},
		CanonState: domain.CoreCanonState{}, Chapters: chapters,
	}
	first, err := compileTaskContext(input, 12<<10)
	if err != nil { t.Fatal(err) }
	second, err := compileTaskContext(input, 12<<10)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(first.ContextJSON, second.ContextJSON) || !bytes.Equal(first.RecentProse, second.RecentProse) {
		t.Fatal("context compilation is not deterministic")
	}
	if !strings.Contains(string(first.ContextJSON), "红色纸伞") { t.Fatalf("context=%s", first.ContextJSON) }
}

func TestTaskPackContainsCompiledCanonContext(t *testing.T) {
	_, _, workspace := newChapterReadyProject(t)
	ready := readReady(t, workspace)
	base := filepath.Join(workspace, "exchange", "outbox", ready.TaskID, ready.AttemptID)
	contextRaw, err := os.ReadFile(filepath.Join(base, "context.json")); if err != nil { t.Fatal(err) }
	canonRaw, err := os.ReadFile(filepath.Join(base, "canon_excerpt.json")); if err != nil { t.Fatal(err) }
	if !strings.Contains(string(contextRaw), "完成主线") || !strings.Contains(string(contextRaw), "主线得到明确收束") {
		t.Fatalf("context missing foundation contracts: %s", contextRaw)
	}
	if !strings.Contains(string(canonRaw), ready.BaseCanonRoot) { t.Fatalf("canon excerpt=%s", canonRaw) }
}
func makeContextChapters(count int) []contextChapter {
	out := make([]contextChapter, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, contextChapter{
			Chapter: i,
			Path: fmt.Sprintf("chapters/%06d/chapter.md", i),
			Text: fmt.Sprintf("第%d章。常规剧情推进。%s", i, strings.Repeat("正文", 80)),
		})
	}
	return out
}
