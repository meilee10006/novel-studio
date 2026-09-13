package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/store"
)

func TestProjectStatusEmptyProject(t *testing.T) {
	root := t.TempDir()
	project, err := OpenProject(root)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Initialized {
		t.Fatalf("Initialized=true, want false")
	}
	if status.Root != root {
		t.Fatalf("Root=%q want %q", status.Root, root)
	}
}

func TestProjectStatusExistingStore(t *testing.T) {
	root := t.TempDir()
	st := store.NewStore(root)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init("本地测试书", 12); err != nil {
		t.Fatal(err)
	}

	project, err := OpenProject(root)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !status.Initialized || status.NovelName != "本地测试书" || status.TotalChapters != 12 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestProjectVerifyReportsDeterministicConsistencyProblem(t *testing.T) {
	root := t.TempDir()
	st := store.NewStore(root)
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if err := st.Progress.Init("校验测试", 1); err != nil {
		t.Fatal(err)
	}
	progress, err := st.Progress.Load()
	if err != nil {
		t.Fatal(err)
	}
	progress.CompletedChapters = []int{1}
	if err := st.Progress.Save(progress); err != nil {
		t.Fatal(err)
	}

	project, err := OpenProject(root)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	result, err := project.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.OK || len(result.Problems) == 0 {
		t.Fatalf("Verify=%+v, want deterministic problem", result)
	}

	if err := os.WriteFile(filepath.Join(root, "chapters", "01.md"), []byte("正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err = project.Verify()
	if err != nil {
		t.Fatalf("Verify after repair: %v", err)
	}
	if !result.OK {
		t.Fatalf("Verify after repair=%+v", result)
	}
}
