package core

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

const (
	storyLockedCheckpointPolicyVersion     = 1
	foundationReadyCheckpointPolicyVersion = 1
)

func (p *Project) validateStoryLocked(bundleRef string, evidence map[string]string) error {
	bundle, err := p.loadDesignBundleRef(bundleRef)
	if err != nil {
		return err
	}
	if err := validateBundleClosure(p, bundle); err != nil {
		return err
	}
	requiredSlots := []string{"creative_brief", "story_decisions", "story_concept"}
	if len(bundle.Selections) != len(requiredSlots) {
		return fmt.Errorf("story_locked bundle must select exactly creative_brief, story_decisions, story_concept")
	}
	for _, slot := range requiredSlots {
		if bundle.Selections[slot] == "" {
			return fmt.Errorf("story_locked bundle is missing %s", slot)
		}
	}

	briefRef := bundle.Selections["creative_brief"]
	decisionsRef := bundle.Selections["story_decisions"]
	conceptRef := bundle.Selections["story_concept"]

	brief, err := p.loadDesignArtifactRef(briefRef)
	if err != nil {
		return err
	}
	if err := validateCreativeBriefPayload(brief.Payload); err != nil {
		return fmt.Errorf("creative_brief: %w", err)
	}
	decisions, err := p.loadDesignArtifactRef(decisionsRef)
	if err != nil {
		return err
	}
	if err := validateStoryDecisionsPayload(decisions.Payload); err != nil {
		return fmt.Errorf("story_decisions: %w", err)
	}
	concept, err := p.loadDesignArtifactRef(conceptRef)
	if err != nil {
		return err
	}
	if err := validateStoryConceptPayload(concept.Payload); err != nil {
		return fmt.Errorf("story_concept: %w", err)
	}
	if !containsAllRefs(concept.Inputs, briefRef, decisionsRef) {
		return fmt.Errorf("story_concept inputs must include current creative_brief and story_decisions")
	}

	if len(evidence) != 2 || evidence["story_review"] == "" || evidence["author_confirmation"] == "" {
		return fmt.Errorf("story_locked evidence must contain exactly story_review and author_confirmation")
	}
	review, err := p.loadDesignArtifactRef(evidence["story_review"])
	if err != nil {
		return fmt.Errorf("story_review: %w", err)
	}
	if err := validateReviewArtifact(review, "story_review", conceptRef); err != nil {
		return fmt.Errorf("story_review: %w", err)
	}
	confirmation, err := p.loadDesignArtifactRef(evidence["author_confirmation"])
	if err != nil {
		return fmt.Errorf("author_confirmation: %w", err)
	}
	if err := validateAuthorConfirmation(confirmation, conceptRef); err != nil {
		return fmt.Errorf("author_confirmation: %w", err)
	}
	return nil
}

func (p *Project) validateFoundationReady(
	head *domain.CoreDesignHead,
	bundleRef string,
	evidence map[string]string,
	state *domain.CoreProductionState,
) error {
	if head == nil || head.DesignRoot == "" || head.Checkpoint != domain.DesignCheckpointStoryLocked {
		return fmt.Errorf("foundation_ready requires current story_locked design head")
	}
	parentCommit, err := p.store.LoadCoreDesignCommit(head.DesignRoot)
	if err != nil {
		return err
	}
	if parentCommit == nil || parentCommit.Checkpoint != domain.DesignCheckpointStoryLocked {
		return fmt.Errorf("current story_locked design commit is missing or invalid")
	}
	parentBundle, err := p.loadDesignBundleRef(parentCommit.BundleRef)
	if err != nil {
		return fmt.Errorf("story_locked parent bundle: %w", err)
	}
	bundle, err := p.loadDesignBundleRef(bundleRef)
	if err != nil {
		return err
	}

	requiredSlots := []string{
		"creative_brief",
		"story_decisions",
		"story_concept",
		"foundation",
		"characters",
		"world",
		"book_plan",
		"ending_contract",
		"style_profile",
		"platform_profile",
	}
	if len(bundle.Selections) != len(requiredSlots) {
		return fmt.Errorf("foundation_ready bundle must select exactly 10 fixed slots")
	}
	for _, slot := range requiredSlots {
		if bundle.Selections[slot] == "" {
			return fmt.Errorf("foundation_ready bundle is missing %s", slot)
		}
	}
	for _, slot := range []string{"creative_brief", "story_decisions", "story_concept"} {
		if bundle.Selections[slot] != parentBundle.Selections[slot] {
			return fmt.Errorf("%s does not match current story_locked bundle", slot)
		}
	}

	artifacts := make(map[string]domain.CoreDesignArtifact, len(requiredSlots))
	for _, slot := range requiredSlots {
		artifact, err := p.loadDesignArtifactRef(bundle.Selections[slot])
		if err != nil {
			return fmt.Errorf("%s: %w", slot, err)
		}
		if artifact.ArtifactType != slot {
			return fmt.Errorf("%s selects artifact type %s", slot, artifact.ArtifactType)
		}
		artifacts[slot] = artifact
	}

	briefRef := bundle.Selections["creative_brief"]
	conceptRef := bundle.Selections["story_concept"]
	charactersRef := bundle.Selections["characters"]
	worldRef := bundle.Selections["world"]
	endingRef := bundle.Selections["ending_contract"]

	type hardInputRule struct {
		slot     string
		required []struct {
			name string
			ref  string
		}
	}
	rules := []hardInputRule{
		{
			slot: "characters",
			required: []struct {
				name string
				ref  string
			}{{"story_concept", conceptRef}},
		},
		{
			slot: "world",
			required: []struct {
				name string
				ref  string
			}{{"story_concept", conceptRef}},
		},
		{
			slot: "ending_contract",
			required: []struct {
				name string
				ref  string
			}{{"story_concept", conceptRef}},
		},
		{
			slot: "foundation",
			required: []struct {
				name string
				ref  string
			}{
				{"story_concept", conceptRef},
				{"characters", charactersRef},
				{"world", worldRef},
			},
		},
		{
			slot: "book_plan",
			required: []struct {
				name string
				ref  string
			}{
				{"story_concept", conceptRef},
				{"characters", charactersRef},
				{"world", worldRef},
				{"ending_contract", endingRef},
			},
		},
		{
			slot: "style_profile",
			required: []struct {
				name string
				ref  string
			}{{"creative_brief", briefRef}},
		},
		{
			slot: "platform_profile",
			required: []struct {
				name string
				ref  string
			}{{"creative_brief", briefRef}},
		},
	}
	for _, rule := range rules {
		for _, required := range rule.required {
			if !containsAllRefs(artifacts[rule.slot].Inputs, required.ref) {
				return fmt.Errorf("%s inputs must include current %s", rule.slot, required.name)
			}
		}
	}

	if err := validateStoryDecisionsPayload(artifacts["story_decisions"].Payload); err != nil {
		return fmt.Errorf("story_decisions: %w", err)
	}
	if violations := validateWholeBookSkeleton(artifacts["book_plan"].Payload); len(violations) != 0 {
		return fmt.Errorf("book_plan: %s", strings.Join(violations, "; "))
	}
	if err := validateBundleClosure(p, bundle); err != nil {
		return err
	}

	foundationArtifacts, err := p.foundationArtifactsFromDesignBundle(bundle)
	if err != nil {
		return err
	}
	if state == nil {
		state = newCoreProductionState()
	}
	stateCopy := *state
	_, violations, err := prepareFoundationArtifacts(foundationArtifacts, &stateCopy)
	if err != nil {
		return err
	}
	if len(violations) != 0 {
		return fmt.Errorf("foundation preparation failed: %s", strings.Join(violations, "; "))
	}

	if len(evidence) != 1 || evidence["foundation_readiness_review"] == "" {
		return fmt.Errorf("foundation_ready evidence must contain exactly foundation_readiness_review")
	}
	review, err := p.loadDesignArtifactRef(evidence["foundation_readiness_review"])
	if err != nil {
		return fmt.Errorf("foundation_readiness_review: %w", err)
	}
	if err := validateReviewArtifact(review, "foundation_readiness_review", bundleRef); err != nil {
		return fmt.Errorf("foundation_readiness_review: %w", err)
	}
	return nil
}

func validateCreativeBriefPayload(payload any) error {
	obj, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be an object")
	}
	for _, field := range []string{"purpose", "target_genre"} {
		if !nonEmptyString(obj[field]) {
			return fmt.Errorf("%s must be a non-empty string", field)
		}
	}
	for _, field := range []string{"platform_constraints", "preferences", "exclusions", "author_principles"} {
		if err := validateUniqueNonEmptyStringArray(obj[field]); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}
	return nil
}

func validateStoryDecisionsPayload(payload any) error {
	obj, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be an object")
	}
	items, ok := obj["decisions"].([]any)
	if !ok {
		return fmt.Errorf("decisions must be an array")
	}
	ids := make(map[string]bool, len(items))
	type relation struct {
		id         string
		supersedes string
	}
	relations := make([]relation, 0, len(items))
	for i, item := range items {
		decision, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("decision %d must be an object", i)
		}
		id, ok := decision["id"].(string)
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			return fmt.Errorf("decision %d id must be a non-empty string", i)
		}
		if ids[id] {
			return fmt.Errorf("duplicate decision id %q", id)
		}
		ids[id] = true

		status, ok := decision["status"].(string)
		if !ok || (status != "locked" && status != "rejected" && status != "open") {
			return fmt.Errorf("decision %s has invalid status", id)
		}
		if !nonEmptyString(decision["statement"]) {
			return fmt.Errorf("decision %s statement must be non-empty", id)
		}
		blocking, ok := decision["blocking"].(bool)
		if !ok {
			return fmt.Errorf("decision %s blocking must be bool", id)
		}
		if blocking && status == "open" {
			return fmt.Errorf("decision %s is blocking and open", id)
		}
		supersedes := ""
		if raw, exists := decision["supersedes"]; exists && raw != nil {
			var ok bool
			supersedes, ok = raw.(string)
			supersedes = strings.TrimSpace(supersedes)
			if !ok || supersedes == "" {
				return fmt.Errorf("decision %s supersedes must be a non-empty string", id)
			}
		}
		relations = append(relations, relation{id: id, supersedes: supersedes})
	}
	for _, rel := range relations {
		if rel.supersedes == "" {
			continue
		}
		if rel.supersedes == rel.id {
			return fmt.Errorf("decision %s cannot supersede itself", rel.id)
		}
		if !ids[rel.supersedes] {
			return fmt.Errorf("decision %s supersedes unknown decision %s", rel.id, rel.supersedes)
		}
	}
	return nil
}

func validateStoryConceptPayload(payload any) error {
	obj, ok := payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be an object")
	}
	for _, field := range []string{
		"story", "protagonist_goal", "central_conflict",
		"story_engine", "change_path", "ending_direction",
	} {
		if !nonEmptyString(obj[field]) {
			return fmt.Errorf("%s must be a non-empty string", field)
		}
	}
	return nil
}

func containsAllRefs(got []string, required ...string) bool {
	seen := make(map[string]bool, len(got))
	for _, ref := range got {
		seen[ref] = true
	}
	for _, ref := range required {
		if !seen[ref] {
			return false
		}
	}
	return true
}

func validateBundleClosure(project *Project, bundle domain.CoreDesignBundle) error {
	for slot, selectedRef := range bundle.Selections {
		artifact, err := project.loadDesignArtifactRef(selectedRef)
		if err != nil {
			return err
		}
		if artifact.ArtifactType != slot {
			return fmt.Errorf("bundle slot %s selects artifact type %s", slot, artifact.ArtifactType)
		}
		for _, inputRef := range artifact.Inputs {
			inputArtifact, err := project.loadDesignArtifactRef(inputRef)
			if err != nil {
				return err
			}
			if selectedInput, ok := bundle.Selections[inputArtifact.ArtifactType]; ok && selectedInput != inputRef {
				return fmt.Errorf("%s depends on stale %s", slot, inputArtifact.ArtifactType)
			}
		}
		for _, sourceRef := range artifact.Sources {
			if _, err := project.loadDesignArtifactRef(sourceRef); err != nil {
				return err
			}
		}
		if artifact.Supersedes != "" {
			if _, err := project.loadDesignArtifactRef(artifact.Supersedes); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateReviewArtifact(artifact domain.CoreDesignArtifact, wantType, subjectRef string) error {
	if artifact.ArtifactType != wantType {
		return fmt.Errorf("artifact type is %s, want %s", artifact.ArtifactType, wantType)
	}
	obj, ok := artifact.Payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be an object")
	}
	if obj["review_type"] != wantType {
		return fmt.Errorf("review_type must be %s", wantType)
	}
	if obj["subject_ref"] != subjectRef {
		return fmt.Errorf("subject_ref does not match current subject")
	}
	if !positiveInteger(obj["policy_version"]) {
		return fmt.Errorf("policy_version must be a positive integer")
	}
	if obj["verdict"] != "PASS" {
		return fmt.Errorf("verdict must be PASS")
	}
	if _, ok := obj["findings"].([]any); !ok {
		return fmt.Errorf("findings must be an array")
	}
	if _, ok := obj["created_from_context_refs"].([]any); !ok {
		return fmt.Errorf("created_from_context_refs must be an array")
	}
	return nil
}

func validateAuthorConfirmation(artifact domain.CoreDesignArtifact, subjectRef string) error {
	if artifact.ArtifactType != "author_confirmation" {
		return fmt.Errorf("artifact type is %s, want author_confirmation", artifact.ArtifactType)
	}
	obj, ok := artifact.Payload.(map[string]any)
	if !ok {
		return fmt.Errorf("payload must be an object")
	}
	if obj["subject_ref"] != subjectRef {
		return fmt.Errorf("subject_ref does not match current story_concept")
	}
	if obj["decision"] != "APPROVED" {
		return fmt.Errorf("decision must be APPROVED")
	}
	return nil
}

func (p *Project) loadDesignBundleRef(ref string) (domain.CoreDesignBundle, error) {
	digest, err := parseDesignBundleRef(ref)
	if err != nil {
		return domain.CoreDesignBundle{}, err
	}
	raw, err := p.store.ReadCoreDesignBundle(digest)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.CoreDesignBundle{}, fmt.Errorf("design bundle ref does not exist: %s", ref)
		}
		return domain.CoreDesignBundle{}, err
	}
	bundle, _, actualRef, err := canonicalDesignBundle(raw)
	if err != nil {
		return domain.CoreDesignBundle{}, err
	}
	if actualRef != ref {
		return domain.CoreDesignBundle{}, fmt.Errorf("design bundle ref does not match stored object: %s", ref)
	}
	return bundle, nil
}

func computeDesignRoot(commit domain.CoreDesignCommit) (string, error) {
	return digestJSON(commit)
}

func nonEmptyString(value any) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) != ""
}

func validateUniqueNonEmptyStringArray(value any) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("must be an array")
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		s, ok := item.(string)
		s = strings.TrimSpace(s)
		if !ok || s == "" {
			return fmt.Errorf("must contain only non-empty strings")
		}
		if seen[s] {
			return fmt.Errorf("must not contain duplicate strings")
		}
		seen[s] = true
	}
	return nil
}

func positiveInteger(value any) bool {
	switch n := value.(type) {
	case float64:
		return n > 0 && math.Trunc(n) == n
	case float32:
		f := float64(n)
		return f > 0 && math.Trunc(f) == f
	case int:
		return n > 0
	case int64:
		return n > 0
	case json.Number:
		i, err := n.Int64()
		return err == nil && i > 0
	default:
		return false
	}
}

func validateWholeBookSkeleton(payload any) []string {
	root, ok := payload.(map[string]any)
	if !ok {
		return []string{"book_plan payload must be an object"}
	}
	var violations []string
	if !nonEmptyString(root["direction"]) {
		violations = append(violations, "book_plan.direction must be a non-empty string")
	}
	skeleton, ok := root["whole_book_skeleton"].(map[string]any)
	if !ok {
		return append(violations, "book_plan.whole_book_skeleton must be an object")
	}
	stages, ok := skeleton["stages"].([]any)
	if !ok || len(stages) < 2 {
		violations = append(violations, "book_plan.whole_book_skeleton.stages must contain at least two stages")
	} else {
		seen := make(map[string]bool, len(stages))
		for i, raw := range stages {
			stage, ok := raw.(map[string]any)
			if !ok {
				violations = append(violations, fmt.Sprintf("book_plan.whole_book_skeleton.stages[%d] must be an object", i))
				continue
			}
			id, _ := stage["id"].(string)
			id = strings.TrimSpace(id)
			if id == "" {
				violations = append(violations, fmt.Sprintf("book_plan.whole_book_skeleton.stages[%d].id must be a non-empty string", i))
			} else if seen[id] {
				violations = append(violations, fmt.Sprintf("book_plan.whole_book_skeleton stage id %q must be unique", id))
			} else {
				seen[id] = true
			}
			if !nonEmptyString(stage["objective"]) {
				violations = append(violations, fmt.Sprintf("book_plan.whole_book_skeleton.stages[%d].objective must be a non-empty string", i))
			}
			if !nonEmptyString(stage["transition"]) {
				violations = append(violations, fmt.Sprintf("book_plan.whole_book_skeleton.stages[%d].transition must be a non-empty string", i))
			}
		}
	}
	if !nonEmptyString(skeleton["ending_connection"]) {
		violations = append(violations, "book_plan.whole_book_skeleton.ending_connection must be a non-empty string")
	}
	return violations
}
