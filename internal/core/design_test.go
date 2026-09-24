package core

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/store"
)

func TestDesignArtifactRefBindsPayloadAndHardInputs(t *testing.T) {
	a := []byte(`{"schema_version":1,"artifact_type":"story_concept","inputs":["creative_brief@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"sources":[],"payload":{"story":"同一个故事"}}`)
	b := []byte(`{
      "payload":{"story":"同一个故事"},
      "sources":[],
      "inputs":["creative_brief@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],
      "artifact_type":"story_concept",
      "schema_version":1
    }`)

	_, _, refA, err := canonicalDesignArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	_, _, refB, err := canonicalDesignArtifact(b)
	if err != nil {
		t.Fatal(err)
	}
	if refA == refB {
		t.Fatalf("hard input change did not change artifact ref: %s", refA)
	}
}

func TestDesignBundleRefIsStableAcrossJSONFormatting(t *testing.T) {
	a := []byte(`{"schema_version":1,"selections":{"story_concept":"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	b := []byte("{\n  \"selections\": {\"story_concept\": \"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"},\n  \"schema_version\": 1\n}\n")

	_, _, refA, err := canonicalDesignBundle(a)
	if err != nil {
		t.Fatal(err)
	}
	_, _, refB, err := canonicalDesignBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if refA != refB {
		t.Fatalf("same bundle got refs %q and %q", refA, refB)
	}
}

func TestDesignStoreImmutableAndHeadRoundTrip(t *testing.T) {
	coreStore := store.NewCoreStore(t.TempDir())
	digest := strings.Repeat("a", 64)
	original := []byte(`{"schema_version":1,"artifact_type":"story_concept"}`)

	if err := coreStore.SaveCoreDesignArtifact(digest, original); err != nil {
		t.Fatalf("save original artifact: %v", err)
	}
	if err := coreStore.SaveCoreDesignArtifact(digest, original); err != nil {
		t.Fatalf("idempotent artifact save: %v", err)
	}
	if err := coreStore.SaveCoreDesignArtifact(digest, []byte(`{"schema_version":1,"artifact_type":"different"}`)); err == nil {
		t.Fatal("conflicting artifact bytes unexpectedly overwrote immutable object")
	}

	head := &domain.CoreDesignHead{
		SchemaVersion: 1,
		DesignRoot:    strings.Repeat("b", 64),
		Checkpoint:    domain.DesignCheckpointStoryLocked,
	}
	if err := coreStore.SaveCoreDesignHead(head); err != nil {
		t.Fatalf("save design head: %v", err)
	}
	got, err := coreStore.LoadCoreDesignHead()
	if err != nil {
		t.Fatalf("load design head: %v", err)
	}
	if got == nil {
		t.Fatal("design head missing after save")
	}
	if *got != *head {
		t.Fatalf("design head round-trip mismatch: got %+v want %+v", *got, *head)
	}
}

func TestFoundationReadyPromotesValidDesignBundle(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, reviewRef := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	result := promoteForTest(
		t, project, "design-promote-foundation-valid",
		fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady,
		bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "PROMOTED" || result.NewDesignRoot == "" {
		t.Fatalf("result=%+v", result)
	}
	head := mustDesignHeadForTest(t, project)
	if head.DesignRoot != result.NewDesignRoot || head.Checkpoint != domain.DesignCheckpointFoundationReady {
		t.Fatalf("head=%+v result=%+v", head, result)
	}
	commit, err := project.store.LoadCoreDesignCommit(result.NewDesignRoot)
	if err != nil {
		t.Fatal(err)
	}
	if commit == nil || commit.ParentDesignRoot != fixture.StoryRoot ||
		commit.Checkpoint != domain.DesignCheckpointFoundationReady ||
		commit.BundleRef != bundleRef {
		t.Fatalf("commit=%+v fixture=%+v bundle=%q", commit, fixture, bundleRef)
	}
}

func TestFoundationReadyRejectsBookPlanBuiltFromOldStoryConcept(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)

	oldConceptPayload := validStoryConceptPayload()
	oldConceptPayload["story"] = "未被当前故事锁定采用的旧故事"
	oldConceptRef := importDesignArtifactForTest(
		t, project, "story_concept",
		[]string{fixture.BriefRef, fixture.DecisionsRef},
		oldConceptPayload,
	)

	bundleRef, _ := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	bundle := readDesignBundleForTest(t, project, bundleRef)
	bookPlanRef := bundle.Selections["book_plan"]
	bookPlan := readDesignArtifactForTest(t, project, bookPlanRef)
	bookPlan.Inputs = replaceRefForTest(bookPlan.Inputs, fixture.ConceptRef, oldConceptRef)
	staleBookPlanRef := importDesignArtifactForTest(
		t, project, "book_plan", bookPlan.Inputs, bookPlan.Payload,
	)
	bundle.Selections["book_plan"] = staleBookPlanRef
	staleBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
	staleReviewRef := importFoundationReadinessReviewForTest(t, project, staleBundleRef)

	result := promoteForTest(
		t, project, "design-promote-foundation-stale-plan",
		fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady,
		staleBundleRef,
		map[string]string{"foundation_readiness_review": staleReviewRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "book_plan") || !strings.Contains(result.Problem, "story_concept") {
		t.Fatalf("result=%+v", result)
	}
}

func TestFoundationReadyRejectsStorySelectionDifferentFromLockedParent(t *testing.T) {
	tests := []struct {
		name string
		slot string
	}{
		{name: "creative brief", slot: "creative_brief"},
		{name: "story decisions", slot: "story_decisions"},
		{name: "story concept", slot: "story_concept"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, _, fixture := makeStoryLockedProjectForTest(t)
			bundleRef, _ := importValidFoundationDesignForTest(
				t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
			)
			bundle := readDesignBundleForTest(t, project, bundleRef)
			var replacement string
			switch tc.slot {
			case "creative_brief":
				payload := validCreativeBriefPayload()
				payload["purpose"] = "另一个创作目标"
				replacement = importDesignArtifactForTest(t, project, tc.slot, nil, payload)
			case "story_decisions":
				payload := validStoryDecisionsPayload()
				payload["decisions"] = []any{
					map[string]any{
						"id":        "story-decision-other",
						"status":    "locked",
						"statement": "另一个已经锁定的选择",
						"blocking":  true,
					},
				}
				replacement = importDesignArtifactForTest(t, project, tc.slot, nil, payload)
			case "story_concept":
				payload := validStoryConceptPayload()
				payload["story"] = "另一个候选故事版本"
				replacement = importDesignArtifactForTest(
					t, project, tc.slot,
					[]string{fixture.BriefRef, fixture.DecisionsRef}, payload,
				)
			}
			bundle.Selections[tc.slot] = replacement
			changedBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
			reviewRef := importFoundationReadinessReviewForTest(t, project, changedBundleRef)
			result := promoteForTest(
				t, project, nextDesignSubmissionIDForTest(), fixture.StoryRoot,
				domain.DesignCheckpointFoundationReady, changedBundleRef,
				map[string]string{"foundation_readiness_review": reviewRef},
			)
			if result.Result != "INVALID" || !strings.Contains(result.Problem, tc.slot) || !strings.Contains(result.Problem, "story_locked") {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestFoundationReadyRejectsMissingRequiredHardInputs(t *testing.T) {
	tests := []struct {
		name      string
		slot      string
		removeDep string
	}{
		{name: "characters story concept", slot: "characters", removeDep: "story_concept"},
		{name: "world story concept", slot: "world", removeDep: "story_concept"},
		{name: "ending story concept", slot: "ending_contract", removeDep: "story_concept"},
		{name: "foundation story concept", slot: "foundation", removeDep: "story_concept"},
		{name: "foundation characters", slot: "foundation", removeDep: "characters"},
		{name: "foundation world", slot: "foundation", removeDep: "world"},
		{name: "book plan story concept", slot: "book_plan", removeDep: "story_concept"},
		{name: "book plan characters", slot: "book_plan", removeDep: "characters"},
		{name: "book plan world", slot: "book_plan", removeDep: "world"},
		{name: "book plan ending", slot: "book_plan", removeDep: "ending_contract"},
		{name: "style creative brief", slot: "style_profile", removeDep: "creative_brief"},
		{name: "platform creative brief", slot: "platform_profile", removeDep: "creative_brief"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, _, fixture := makeStoryLockedProjectForTest(t)
			bundleRef, _ := importValidFoundationDesignForTest(
				t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
			)
			bundle := readDesignBundleForTest(t, project, bundleRef)
			artifact := readDesignArtifactForTest(t, project, bundle.Selections[tc.slot])
			depRef := bundle.Selections[tc.removeDep]
			artifact.Inputs = removeRefForTest(artifact.Inputs, depRef)
			changedRef := importDesignArtifactForTest(t, project, tc.slot, artifact.Inputs, artifact.Payload)
			bundle.Selections[tc.slot] = changedRef
			changedBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
			reviewRef := importFoundationReadinessReviewForTest(t, project, changedBundleRef)
			result := promoteForTest(
				t, project, nextDesignSubmissionIDForTest(), fixture.StoryRoot,
				domain.DesignCheckpointFoundationReady, changedBundleRef,
				map[string]string{"foundation_readiness_review": reviewRef},
			)
			if result.Result != "INVALID" ||
				!strings.Contains(result.Problem, tc.slot) ||
				!strings.Contains(result.Problem, tc.removeDep) {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestFoundationReadyRejectsShortWholeBookSkeleton(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, _ := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	bundle := readDesignBundleForTest(t, project, bundleRef)
	bookPlan := readDesignArtifactForTest(t, project, bundle.Selections["book_plan"])
	payload := bookPlan.Payload.(map[string]any)
	skeleton := payload["whole_book_skeleton"].(map[string]any)
	skeleton["stages"] = []any{
		map[string]any{
			"id":         "stage-only",
			"objective":  "只有一个阶段",
			"transition": "没有足够后续阶段",
		},
	}
	changedBookPlanRef := importDesignArtifactForTest(t, project, "book_plan", bookPlan.Inputs, payload)
	bundle.Selections["book_plan"] = changedBookPlanRef
	changedBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
	reviewRef := importFoundationReadinessReviewForTest(t, project, changedBundleRef)

	result := promoteForTest(
		t, project, "design-promote-foundation-short-skeleton", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, changedBundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "stages") {
		t.Fatalf("result=%+v", result)
	}
}

func TestFoundationReadyRejectsReadinessReviewForDifferentBundle(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, _ := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	wrongReviewRef := importFoundationReadinessReviewForTest(t, project, fixture.BundleRef)
	result := promoteForTest(
		t, project, "design-promote-foundation-wrong-review", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, bundleRef,
		map[string]string{"foundation_readiness_review": wrongReviewRef},
	)
	if result.Result != "INVALID" ||
		!strings.Contains(result.Problem, "foundation_readiness_review") ||
		!strings.Contains(result.Problem, "subject_ref") {
		t.Fatalf("result=%+v", result)
	}
}

func TestFoundationReadyRequiresExactEvidenceKey(t *testing.T) {
	tests := []struct {
		name     string
		evidence func(string) map[string]string
	}{
		{
			name: "missing",
			evidence: func(string) map[string]string {
				return map[string]string{}
			},
		},
		{
			name: "extra",
			evidence: func(reviewRef string) map[string]string {
				return map[string]string{
					"foundation_readiness_review": reviewRef,
					"unexpected":                  reviewRef,
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, _, fixture := makeStoryLockedProjectForTest(t)
			bundleRef, reviewRef := importValidFoundationDesignForTest(
				t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
			)
			result := promoteForTest(
				t, project, nextDesignSubmissionIDForTest(), fixture.StoryRoot,
				domain.DesignCheckpointFoundationReady, bundleRef, tc.evidence(reviewRef),
			)
			if result.Result != "INVALID" || !strings.Contains(result.Problem, "evidence") {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestFoundationReadyIgnoresNewSourceCandidateUntilStoryIsRelocked(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	sourceRef := importDesignArtifactForTest(t, project, "source_record", nil, map[string]any{
		"source": "later research",
	})
	candidatePayload := validStoryConceptPayload()
	candidatePayload["story"] = "带有新 source 的候选故事，但尚未重新锁定"
	candidateRef := importDesignArtifactWithSourcesForTest(
		t, project, "story_concept",
		[]string{fixture.BriefRef, fixture.DecisionsRef},
		[]string{sourceRef},
		candidatePayload,
	)
	if candidateRef == fixture.ConceptRef {
		t.Fatal("sourceful candidate unexpectedly equals locked story concept")
	}

	bundleRef, reviewRef := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	result := promoteForTest(
		t, project, "design-promote-foundation-source-candidate", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "PROMOTED" {
		t.Fatalf("result=%+v", result)
	}
}

func TestDesignOperationsDoNotConsumeProductionSequences(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	before, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if before == nil {
		before = newCoreProductionState()
	}
	snapshot := *before

	bundleRef, reviewRef := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	result := promoteForTest(
		t, project, "design-promote-foundation-seq",
		fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady,
		bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "PROMOTED" {
		t.Fatalf("result=%+v", result)
	}

	after, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if after == nil {
		after = newCoreProductionState()
	}
	if !reflect.DeepEqual(snapshot, *after) {
		t.Fatalf("design changed production state: before=%+v after=%+v", snapshot, *after)
	}
}

func removeRefForTest(items []string, remove string) []string {
	out := make([]string, 0, len(items))
	for _, ref := range items {
		if ref != remove {
			out = append(out, ref)
		}
	}
	return out
}

func TestImportingNewSourceDoesNotMoveStoryLockedHead(t *testing.T) {
	project, _, _ := newRequiredDesignProjectForTest(t)
	bundleRef, evidence := validStoryLockInputsForTest(t, project)
	locked := promoteForTest(
		t, project, "source-stability-lock", "",
		domain.DesignCheckpointStoryLocked, bundleRef, evidence,
	)
	if locked.Result != "PROMOTED" {
		t.Fatalf("locked=%+v", locked)
	}

	_ = importDesignArtifactForTest(
		t, project, "source_record", nil,
		map[string]any{
			"name":        "新资料",
			"source_type": "web",
			"locator":     "example",
			"summary":     "新增但尚未采用的资料",
			"analysis":    "只作为候选 source",
		},
	)

	head := mustDesignHeadForTest(t, project)
	if head.DesignRoot != locked.NewDesignRoot {
		t.Fatalf("source import moved head: %+v", head)
	}
}

func TestFoundationReadyFreezesFurtherPromote(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, reviewRef := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	first := promoteForTest(
		t, project, "design-promote-foundation-freeze-1", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if first.Result != "PROMOTED" {
		t.Fatalf("first=%+v", first)
	}
	second := promoteForTest(
		t, project, "design-promote-foundation-freeze-2", first.NewDesignRoot,
		domain.DesignCheckpointFoundationReady, bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if second.Result != "INVALID" || !strings.Contains(second.Problem, "frozen") {
		t.Fatalf("second=%+v", second)
	}
}

func TestFoundationReadyRejectsIncompleteTenSlotBundle(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, _ := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	bundle := readDesignBundleForTest(t, project, bundleRef)
	delete(bundle.Selections, "platform_profile")
	incompleteBundleRef := importDesignBundleForTest(t, project, bundle.Selections)
	reviewRef := importFoundationReadinessReviewForTest(t, project, incompleteBundleRef)
	result := promoteForTest(
		t, project, "design-promote-foundation-incomplete-bundle", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, incompleteBundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "exactly 10") {
		t.Fatalf("result=%+v", result)
	}
}

func TestFoundationReadyRejectsInvalidReadinessReviewPolicyVersion(t *testing.T) {
	project, _, _, fixture := makeStoryLockedProjectForTest(t)
	bundleRef, _ := importValidFoundationDesignForTest(
		t, project, fixture.BriefRef, fixture.DecisionsRef, fixture.ConceptRef,
	)
	reviewRef := importDesignArtifactForTest(t, project, "foundation_readiness_review", nil, map[string]any{
		"review_type":               "foundation_readiness_review",
		"subject_ref":               bundleRef,
		"policy_version":            0,
		"verdict":                   "PASS",
		"findings":                  []any{},
		"created_from_context_refs": []any{},
	})
	result := promoteForTest(
		t, project, "design-promote-foundation-bad-policy", fixture.StoryRoot,
		domain.DesignCheckpointFoundationReady, bundleRef,
		map[string]string{"foundation_readiness_review": reviewRef},
	)
	if result.Result != "INVALID" || !strings.Contains(result.Problem, "policy_version") {
		t.Fatalf("result=%+v", result)
	}
}
