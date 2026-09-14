package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/protocol"
)

type ExportResult struct {
	Path      string `json:"path"`
	CanonRoot string `json:"canon_root"`
	Chapters  int    `json:"chapters"`
}

func (p *Project) ExportBook(path string) (ExportResult, error) {
	release, err := p.acquireProjectReadLock()
	if err != nil {
		return ExportResult{}, err
	}
	defer release()
	raw, err := p.store.ReadCoreCanonStateBytes()
	if err != nil {
		return ExportResult{}, err
	}
	var state domain.CoreCanonState
	if err := protocol.DecodeJSON(raw, &state); err != nil {
		return ExportResult{}, err
	}
	if problems := endingConstraintProblems(state.Longform, state.LatestChapter); len(problems) > 0 {
		return ExportResult{}, fmt.Errorf("final export blocked: ending constraints not satisfied: %s", strings.Join(problems, "; "))
	}
	production, err := p.store.LoadCoreProductionState()
	if err != nil || production == nil {
		if err == nil {
			err = fmt.Errorf("production state does not exist")
		}
		return ExportResult{}, err
	}
	if production.RevisionReplay != nil {
		return ExportResult{}, fmt.Errorf("final export blocked: revision replay is still active")
	}
	head, err := p.store.LoadCoreCanonHead()
	if err != nil || head == nil {
		if err == nil {
			err = fmt.Errorf("canon head does not exist")
		}
		return ExportResult{}, err
	}
	recomputed, err := p.RecomputeCanonRoot()
	if err != nil {
		return ExportResult{}, fmt.Errorf("verify canon before export: %w", err)
	}
	if recomputed != head.Root || production.CanonRoot != head.Root {
		return ExportResult{}, fmt.Errorf("final export blocked: active canon root does not verify")
	}
	if state.LatestChapter <= 0 {
		return ExportResult{}, fmt.Errorf("final export blocked: no accepted chapters")
	}
	var body bytes.Buffer
	for chapter := 1; chapter <= state.LatestChapter; chapter++ {
		name := filepath.ToSlash(filepath.Join("chapters", fmt.Sprintf("%06d", chapter), "chapter.md"))
		if _, ok := head.ArtifactDigests[name]; !ok {
			return ExportResult{}, fmt.Errorf("final export blocked: active canon is missing chapter %d", chapter)
		}
		rawChapter, err := p.store.ReadCoreCanonArtifact(name)
		if err != nil {
			return ExportResult{}, err
		}
		if chapter > 1 {
			body.WriteString("\n\n")
		}
		body.WriteString(strings.TrimRight(string(rawChapter), "\n"))
		body.WriteByte('\n')
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return ExportResult{}, fmt.Errorf("export path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ExportResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return ExportResult{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".novel-core-export-*")
	if err != nil {
		return ExportResult{}, err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(body.Bytes()); err != nil {
		return ExportResult{}, err
	}
	if err := tmp.Sync(); err != nil {
		return ExportResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return ExportResult{}, err
	}
	if err := os.Rename(tmpPath, abs); err != nil {
		return ExportResult{}, err
	}
	ok = true
	return ExportResult{Path: abs, CanonRoot: head.Root, Chapters: state.LatestChapter}, nil
}
func endingConstraintProblems(state domain.CoreLongformState, latestChapter int) []string {
	var problems []string
	if state.Ending == nil || state.Ending.MainResolutionEventID == "" {
		problems = append(problems, "ending resolution is missing")
	} else if event, ok := state.Events[state.Ending.MainResolutionEventID]; !ok {
		problems = append(problems, "ending resolution evidence event is missing")
	} else if event.Chapter != latestChapter {
		problems = append(problems, fmt.Sprintf("ending resolution is from chapter %d, not latest accepted chapter %d", event.Chapter, latestChapter))
	}
	for id, item := range state.Foreshadows {
		if item.State != "closed" && item.State != "retired" {
			problems = append(problems, fmt.Sprintf("foreshadow %s is %s", id, item.State))
		}
	}
	for id, item := range state.ReaderPromises {
		if item.State != "fulfilled" && item.State != "retired" {
			problems = append(problems, fmt.Sprintf("reader promise %s is %s", id, item.State))
		}
	}
	sort.Strings(problems)
	return problems
}
