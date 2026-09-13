package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/store"
)

type Project struct {
	root  string
	store *store.Store
}

type Status struct {
	Root              string       `json:"root"`
	Initialized       bool         `json:"initialized"`
	NovelName         string       `json:"novel_name,omitempty"`
	Phase             domain.Phase `json:"phase,omitempty"`
	CurrentChapter    int          `json:"current_chapter,omitempty"`
	TotalChapters     int          `json:"total_chapters,omitempty"`
	CompletedChapters int          `json:"completed_chapters,omitempty"`
	Warnings          []string     `json:"warnings,omitempty"`
}

type Verification struct {
	Root     string   `json:"root"`
	OK       bool     `json:"ok"`
	Problems []string `json:"problems,omitempty"`
}

func OpenProject(root string) (*Project, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("project root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("open project %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root %q is not a directory", abs)
	}
	return &Project{root: abs, store: store.NewStore(abs)}, nil
}

func (p *Project) Status() (Status, error) {
	progress, err := p.store.Progress.Load()
	if err != nil {
		return Status{}, fmt.Errorf("load progress: %w", err)
	}
	out := Status{Root: p.root, Warnings: p.store.CheckConsistency()}
	if progress == nil {
		return out, nil
	}
	out.Initialized = true
	out.NovelName = progress.NovelName
	out.Phase = progress.Phase
	out.CurrentChapter = progress.CurrentChapter
	out.TotalChapters = progress.TotalChapters
	out.CompletedChapters = len(progress.CompletedChapters)
	return out, nil
}

func (p *Project) Verify() (Verification, error) {
	status, err := p.Status()
	if err != nil {
		return Verification{}, err
	}
	problems := append([]string(nil), status.Warnings...)
	return Verification{Root: p.root, OK: len(problems) == 0, Problems: problems}, nil
}
