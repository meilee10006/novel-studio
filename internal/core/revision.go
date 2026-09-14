package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func (p *Project) activeChapterLineage(root string) (map[int]domain.CoreCommitJournal, error) {
	journals, err := p.store.ListCoreCommitJournals()
	if err != nil {
		return nil, err
	}
	byRoot := make(map[string]domain.CoreCommitJournal, len(journals))
	for _, journal := range journals {
		if journal.State != "committed" || journal.NewRoot == "" {
			continue
		}
		if prior, ok := byRoot[journal.NewRoot]; ok && prior.AttemptID != journal.AttemptID {
			return nil, fmt.Errorf("multiple commit journals claim canon root %s", journal.NewRoot)
		}
		byRoot[journal.NewRoot] = journal
	}
	lineage := map[int]domain.CoreCommitJournal{}
	seenRoots := map[string]bool{}
	for current := root; current != ""; {
		if seenRoots[current] {
			return nil, fmt.Errorf("canon lineage cycle at %s", current)
		}
		seenRoots[current] = true
		journal, ok := byRoot[current]
		if !ok {
			break
		}
		if _, exists := lineage[journal.Chapter]; exists {
			return nil, fmt.Errorf("active canon lineage repeats chapter %d", journal.Chapter)
		}
		lineage[journal.Chapter] = journal
		current = journal.PreviousRoot
	}
	return lineage, nil
}

func (p *Project) buildRevisionReplay(root string, start, head int) (*domain.CoreRevisionReplay, string, error) {
	lineage, err := p.activeChapterLineage(root)
	if err != nil {
		return nil, "", err
	}
	if start <= 0 || head < start {
		return nil, "", fmt.Errorf("invalid revision range %d..%d", start, head)
	}
	superseded := make([]domain.CoreSupersededChapter, 0, head-start+1)
	for chapter := start; chapter <= head; chapter++ {
		journal, ok := lineage[chapter]
		if !ok {
			return nil, "", fmt.Errorf("accepted chapter %d is missing from active canon lineage", chapter)
		}
		superseded = append(superseded, domain.CoreSupersededChapter{
			Chapter: chapter, AttemptID: journal.AttemptID, CanonRoot: journal.NewRoot, Status: "superseded",
		})
	}
	startJournal := lineage[start]
	return &domain.CoreRevisionReplay{
		StartChapter: start, OriginalHeadChapter: head, OriginalHeadRoot: root, Superseded: superseded,
	}, startJournal.PreviousRoot, nil
}

func newTaskPreservingPending(state *domain.CoreProductionState, kind, target, baseRoot string) *domain.CoreTask {
	pending := append([]domain.CoreTaskConstraint(nil), state.PendingControls...)
	state.PendingControls = nil
	task := newTask(state, kind, target, baseRoot)
	state.PendingControls = pending
	return task
}

func (p *Project) loadCanonSnapshotAtRoot(root string) (domain.CoreCanonState, domain.CoreCanonHead, error) {
	currentHead, err := p.store.LoadCoreCanonHead()
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	if currentHead != nil && currentHead.Root == root {
		raw, err := p.store.ReadCoreCanonStateBytes()
		if err != nil {
			return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
		}
		var current domain.CoreCanonState
		if err := protocol.DecodeJSON(raw, &current); err != nil {
			return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
		}
		return current, *currentHead, nil
	}
	journals, err := p.store.ListCoreCommitJournals()
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	for _, journal := range journals {
		if journal.State == "committed" && journal.NewRoot == root {
			return journal.CanonState, journal.CanonHead, nil
		}
	}

	var firstChapter *domain.CoreCommitJournal
	for i := range journals {
		journal := journals[i]
		if journal.State != "committed" || journal.Chapter != 1 || journal.PreviousRoot != root {
			continue
		}
		if firstChapter == nil || journal.CanonState.Revision < firstChapter.CanonState.Revision {
			copy := journal
			firstChapter = &copy
		}
	}
	if firstChapter == nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("canon root %s has no reconstructable snapshot", root)
	}
	project, err := p.store.LoadCoreProjectState()
	if err != nil || project == nil {
		if err == nil {
			err = fmt.Errorf("project is not initialized")
		}
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	world, err := p.store.ReadCoreCanonArtifact("world.json")
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	longform, violations, err := foundationLongformState(world)
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	if len(violations) > 0 {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("foundation longform is invalid: %s", strings.Join(violations, "; "))
	}
	bookPlan, err := p.store.ReadCoreCanonArtifact("book_plan.json")
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	planning, planningViolations, err := foundationPlanningState(bookPlan)
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	if len(planningViolations) > 0 {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("foundation planning is invalid: %s", strings.Join(planningViolations, "; "))
	}
	receipts, err := p.store.ListCoreReceipts()
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	var foundationReceipt *domain.CoreReceipt
	for i := range receipts {
		receipt := receipts[i]
		if receipt.Result == "ACCEPTED" && receipt.PreviousRoot == "" && receipt.NewRoot == root {
			copy := receipt
			foundationReceipt = &copy
			break
		}
	}
	if foundationReceipt == nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("foundation receipt for canon root %s is missing", root)
	}
	revision := firstChapter.CanonState.Revision - 1
	if revision <= 0 {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("invalid foundation revision before chapter 1")
	}
	state := domain.CoreCanonState{
		SchemaVersion: coreSchemaVersion, Revision: revision, ProjectID: project.ProjectID,
		LastTaskID: foundationReceipt.TaskID, LastAttemptID: foundationReceipt.AttemptID,
		Longform: longform, Planning: planning,
	}
	artifactDigests := map[string]string{}
	for name, digest := range firstChapter.CanonHead.ArtifactDigests {
		if !strings.HasPrefix(filepath.ToSlash(name), "chapters/") {
			artifactDigests[name] = digest
		}
	}
	stateDigest, err := digestJSON(state)
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	head := domain.CoreCanonHead{
		SchemaVersion: coreSchemaVersion, Revision: revision, ParentRoot: "", Root: root,
		StateDigest: stateDigest, ArtifactDigests: artifactDigests,
	}
	recomputed, err := computeCanonRoot(revision, "", stateDigest, artifactDigests)
	if err != nil {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, err
	}
	if recomputed != root {
		return domain.CoreCanonState{}, domain.CoreCanonHead{}, fmt.Errorf("reconstructed foundation root mismatch: got %s want %s", recomputed, root)
	}
	return state, head, nil
}

func (p *Project) revisionCandidate(state *domain.CoreProductionState, chapter int) ([]byte, error) {
	if state == nil || state.RevisionReplay == nil {
		return nil, fmt.Errorf("revision replay is not active")
	}
	for _, item := range state.RevisionReplay.Superseded {
		if item.Chapter != chapter {
			continue
		}
		logical := filepath.ToSlash(filepath.Join("chapters", fmt.Sprintf("%06d", chapter), "chapter.md"))
		return p.store.ReadCorePreparedArtifact(item.AttemptID, logical)
	}
	return nil, fmt.Errorf("revision source for chapter %d is missing", chapter)
}

func mergeSuperseded(existing, added []domain.CoreSupersededChapter) []domain.CoreSupersededChapter {
	out := append([]domain.CoreSupersededChapter(nil), existing...)
	seen := map[string]bool{}
	for _, item := range out {
		seen[fmt.Sprintf("%d:%s", item.Chapter, item.AttemptID)] = true
	}
	for _, item := range added {
		key := fmt.Sprintf("%d:%s", item.Chapter, item.AttemptID)
		if !seen[key] {
			out = append(out, item)
			seen[key] = true
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Chapter != out[j].Chapter {
			return out[i].Chapter < out[j].Chapter
		}
		return out[i].AttemptID < out[j].AttemptID
	})
	return out
}
