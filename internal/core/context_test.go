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

func TestContextCompilerBoundsKnowledgeInventoryAndKeepsRelevantFacts(t *testing.T) {
	const budget = 16 << 10
	events := make(map[string]domain.CoreEventEvidence, 600)
	facts := make(map[string]domain.CoreKnowledgeFact, 600)
	for i := 1; i <= 600; i++ {
		eventID := fmt.Sprintf("story-event-%06d", i)
		factID := fmt.Sprintf("fact-%03d", i)
		statement := fmt.Sprintf("常规背景事实 %03d：%s", i, strings.Repeat("背景", 40))
		if i == 17 {
			factID = "fact-red-umbrella"
			statement = "主角知道红色纸伞藏在钟楼地下室"
		}
		events[eventID] = domain.CoreEventEvidence{EventID: eventID, Chapter: i, Observers: []string{"character-000001"}}
		facts[factID] = domain.CoreKnowledgeFact{Statement: statement, SourceKind: "observed", EvidenceEventID: eventID}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 600, ProjectID: "book", LatestChapter: 600,
		Longform: domain.CoreLongformState{
			Events:    events,
			Knowledge: map[string]map[string]domain.CoreKnowledgeFact{"character-000001": facts},
		},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:601", CanonRoot: "root-600",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "这一章必须重新呼应红色纸伞"}},
		CanonState:  canon,
	}, budget)
	if err != nil {
		t.Fatalf("knowledge inventory should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"fact-red-umbrella", "主角知道红色纸伞藏在钟楼地下室"} {
		if !strings.Contains(text, want) {
			t.Fatalf("relevant knowledge fact missing %q: %s", want, text)
		}
	}
}

func TestContextCompilerBoundsActiveObligationsAndKeepsRelevantUrgentItems(t *testing.T) {
	const budget = 16 << 10
	foreshadows := make(map[string]domain.CoreForeshadowState, 240)
	promises := make(map[string]domain.CoreReaderPromiseState, 240)
	conflicts := make(map[string]domain.CoreConflictState, 240)
	events := make(map[string]domain.CoreEventEvidence, 240)
	for i := 1; i <= 240; i++ {
		eventID := fmt.Sprintf("story-event-%06d", i)
		fsID := fmt.Sprintf("fs-%03d", i)
		description := fmt.Sprintf("常规伏笔 %03d：%s", i, strings.Repeat("伏笔", 40))
		if i == 7 {
			fsID = "fs-red-umbrella"
			description = "红色纸伞与十年前旧案直接相关"
		}
		events[eventID] = domain.CoreEventEvidence{EventID: eventID, Chapter: i}
		foreshadows[fsID] = domain.CoreForeshadowState{Description: description, State: "reinforced", EvidenceEventID: eventID}
		promiseID := fmt.Sprintf("promise-%03d", i)
		deadline := 900 + i
		statement := fmt.Sprintf("常规读者承诺 %03d：%s", i, strings.Repeat("承诺", 40))
		if i == 11 {
			promiseID = "promise-urgent"
			deadline = 601
			statement = "下一章必须揭示匿名信的寄件人身份"
		}
		promises[promiseID] = domain.CoreReaderPromiseState{Statement: statement, State: "deferred", DeadlineChapter: deadline}
		conflictID := fmt.Sprintf("conflict-%03d", i)
		conflictDescription := fmt.Sprintf("常规冲突 %03d：%s", i, strings.Repeat("冲突", 40))
		state := "open"
		if i%3 == 0 {
			state = "escalated"
		}
		if i == 13 {
			conflictID = "conflict-red-umbrella"
			conflictDescription = "红色纸伞引发主角与管家的公开争夺"
		}
		conflicts[conflictID] = domain.CoreConflictState{
			Description: conflictDescription, Participants: []string{"character-000001"}, State: state,
			EscalationCondition: "争夺公开化", CloseCondition: "争夺结束", EvidenceEventID: eventID,
		}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 600, ProjectID: "book", LatestChapter: 600,
		Longform: domain.CoreLongformState{Events: events, Foreshadows: foreshadows, Conflicts: conflicts, ReaderPromises: promises},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:601", CanonRoot: "root-600",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "这一章必须重新呼应红色纸伞"}},
		CanonState:  canon,
	}, budget)
	if err != nil {
		t.Fatalf("active obligations should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"fs-red-umbrella", "红色纸伞与十年前旧案直接相关", "promise-urgent", "下一章必须揭示匿名信的寄件人身份", "conflict-red-umbrella", "红色纸伞引发主角与管家的公开争夺"} {
		if !strings.Contains(text, want) {
			t.Fatalf("priority obligation missing %q: %s", want, text)
		}
	}
}

func TestContextCompilerBoundsRelationshipInventoryAndKeepsRelevantRelationship(t *testing.T) {
	const budget = 16 << 10
	relationships := make(map[string]domain.CoreEvidenceState, 500)
	events := make(map[string]domain.CoreEventEvidence, 500)
	for i := 1; i <= 500; i++ {
		eventID := fmt.Sprintf("story-event-%06d", i)
		relationshipID := fmt.Sprintf("character-%06d|character-%06d", i, i+1000)
		tags := []string{fmt.Sprintf("常规关系 %03d %s", i, strings.Repeat("关系", 30))}
		if i == 13 {
			relationshipID = "character-000013|character-001013"
			tags = []string{"师徒决裂", "互不信任"}
		}
		events[eventID] = domain.CoreEventEvidence{EventID: eventID, Chapter: i}
		relationships[relationshipID] = domain.CoreEvidenceState{Tags: tags, EvidenceEventID: eventID}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 500, ProjectID: "book", LatestChapter: 500,
		Longform: domain.CoreLongformState{Events: events, Relationships: relationships},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:501", CanonRoot: "root-500",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "这一章必须处理师徒决裂"}}, CanonState: canon,
	}, budget)
	if err != nil {
		t.Fatalf("relationship inventory should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"character-000013|character-001013", "师徒决裂"} {
		if !strings.Contains(text, want) {
			t.Fatalf("relevant relationship missing %q: %s", want, text)
		}
	}
}

func TestContextCompilerBoundsTravelConstraintsAndKeepsRelevantRoute(t *testing.T) {
	const budget = 16 << 10
	travel := make([]domain.CoreTravelConstraint, 0, 600)
	for i := 1; i <= 600; i++ {
		from := fmt.Sprintf("location-%06d", i)
		to := fmt.Sprintf("location-%06d", i+1000)
		if i == 17 {
			from = "location-current"
			to = "location-red-umbrella"
		}
		travel = append(travel, domain.CoreTravelConstraint{FromLocationID: from, ToLocationID: to, MinTicks: int64(i + 1)})
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 600, ProjectID: "book", LatestChapter: 600,
		Longform: domain.CoreLongformState{
			Locations:         map[string]domain.CoreLocationState{"character-000001": {LocationID: "location-current", StartTick: 600, EndTick: 600}},
			TravelConstraints: travel,
		},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:601", CanonRoot: "root-600",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "本章准备前往 location-red-umbrella"}}, CanonState: canon,
	}, budget)
	if err != nil {
		t.Fatalf("travel constraints should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"location-current", "location-red-umbrella"} {
		if !strings.Contains(text, want) {
			t.Fatalf("relevant travel route missing %q: %s", want, text)
		}
	}
}

func TestContextCompilerBoundsLocationInventoryAndKeepsRelevantCharacter(t *testing.T) {
	const budget = 16 << 10
	locations := make(map[string]domain.CoreLocationState, 600)
	for i := 1; i <= 600; i++ {
		characterID := fmt.Sprintf("character-%06d", i)
		locationID := fmt.Sprintf("location-%06d", i)
		if i == 23 {
			characterID = "character-red-umbrella"
			locationID = "location-clocktower"
		}
		locations[characterID] = domain.CoreLocationState{LocationID: locationID, StartTick: int64(i), EndTick: int64(i + 1)}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 600, ProjectID: "book", LatestChapter: 600,
		Longform: domain.CoreLongformState{Locations: locations},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:601", CanonRoot: "root-600",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "这一章必须跟进 character-red-umbrella 的位置"}}, CanonState: canon,
	}, budget)
	if err != nil {
		t.Fatalf("location inventory should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	for _, want := range []string{"character-red-umbrella", "location-clocktower"} {
		if !strings.Contains(text, want) {
			t.Fatalf("relevant location missing %q: %s", want, text)
		}
	}
}

func TestContextCompilerBoundsResourceInventoryAndKeepsRelevantResource(t *testing.T) {
	const budget = 16 << 10
	resources := make(map[string]int64, 2000)
	entities := make(map[string]domain.CoreEntityState, 2000)
	for i := 1; i <= 2000; i++ {
		resourceID := fmt.Sprintf("resource-%06d", i)
		name := fmt.Sprintf("常规资源-%03d", i)
		if i == 29 {
			resourceID = "resource-red-umbrella"
			name = "红色纸伞基金"
		}
		resources[resourceID] = int64(i)
		entities[resourceID] = domain.CoreEntityState{EntityType: "resource", Name: name}
	}
	canon := domain.CoreCanonState{
		SchemaVersion: 1, Revision: 2000, ProjectID: "book", LatestChapter: 2000,
		Longform: domain.CoreLongformState{Resources: resources, Entities: entities},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:2001", CanonRoot: "root-2000",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		Constraints: []domain.CoreTaskConstraint{{Instruction: "这一章必须核对红色纸伞基金余额"}}, CanonState: canon,
	}, budget)
	if err != nil {
		t.Fatalf("resource inventory should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.CanonExcerptJSON)
	if !strings.Contains(text, "resource-red-umbrella") {
		t.Fatalf("relevant resource missing: %s", text)
	}
}

func TestContextCompilerBoundsFoundationReferenceAndKeepsRelevantEntities(t *testing.T) {
	const budget = 16 << 10
	characters := make([]any, 0, 1200)
	world := make([]any, 0, 1200)
	for i := 1; i <= 1200; i++ {
		characterID := fmt.Sprintf("character-%06d", i)
		characterName := fmt.Sprintf("常规人物-%04d", i)
		locationID := fmt.Sprintf("location-%06d", i)
		locationName := fmt.Sprintf("常规地点-%04d", i)
		if i == 31 {
			characterID = "character-red-umbrella"
			characterName = "红色纸伞侦探"
			locationID = "location-red-umbrella"
			locationName = "红伞钟楼"
		}
		characters = append(characters, map[string]any{"canon_id": characterID, "name": characterName})
		world = append(world, map[string]any{"entity_type": "location", "canon_id": locationID, "name": locationName})
	}
	foundationReference := map[string]any{
		"foundation": map[string]any{
			"title":            "大型设定书",
			"protagonist":      map[string]any{"entity_type": "character", "canon_id": "character-000001"},
			"opening_location": map[string]any{"entity_type": "location", "canon_id": "location-000001"},
		},
		"characters":       map[string]any{"characters": characters},
		"world":            map[string]any{"entities": world},
		"style_profile":    map[string]any{"language": "zh-CN"},
		"platform_profile": map[string]any{"platform": "fanqie"},
	}
	out, err := compileTaskContext(contextCompilerInput{
		Target: "chapter:2", CanonRoot: "root-1",
		EndingContract: map[string]any{"main_resolution": "收束"}, BookPlan: map[string]any{"direction": "推进"},
		FoundationReference: foundationReference,
		Constraints:         []domain.CoreTaskConstraint{{Instruction: "本章必须让红色纸伞侦探前往红伞钟楼"}},
		CanonState:          domain.CoreCanonState{},
	}, budget)
	if err != nil {
		t.Fatalf("foundation reference should stay within fixed context budget: %v", err)
	}
	if out.TotalBytes > budget {
		t.Fatalf("bytes=%d budget=%d", out.TotalBytes, budget)
	}
	text := string(out.ContextJSON)
	for _, want := range []string{"character-000001", "location-000001", "character-red-umbrella", "location-red-umbrella"} {
		if !strings.Contains(text, want) {
			t.Fatalf("foundation reference missing %q: %s", want, text)
		}
	}
}
