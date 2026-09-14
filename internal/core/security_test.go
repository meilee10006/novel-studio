package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func TestSubmissionSecurityRejectsTraversalSymlinkOversizeAndUnknownArtifact(t *testing.T) {
	t.Run("traversal", func(t *testing.T) {
		project, _, workspace := newChapterReadyProject(t)
		project.submissionQuietPeriod = 0
		ready := readReady(t, workspace)
		artifacts := validChapterArtifacts(1)
		writeSubmission(t, workspace, ready, artifacts, false)
		manifest := manifestForReady(ready, artifacts)
		manifest.Files = append(manifest.Files, "../escape.md")
		writeManifestJSON(t, workspace, ready, manifest)
		assertSubmissionInvalid(t, project, "file list")
	})

	t.Run("symlink", func(t *testing.T) {
		project, _, workspace := newChapterReadyProject(t)
		project.submissionQuietPeriod = 0
		ready := readReady(t, workspace)
		artifacts := validChapterArtifacts(1)
		writeSubmission(t, workspace, ready, artifacts, false)
		outside := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(outside, artifacts["chapter.md"], 0o644); err != nil {
			t.Fatal(err)
		}
		chapterPath := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "chapter.md")
		if err := os.Remove(chapterPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, chapterPath); err != nil {
			t.Fatal(err)
		}
		assertSubmissionInvalid(t, project, "symlink")
	})

	t.Run("oversize manifest", func(t *testing.T) {
		project, _, workspace := newChapterReadyProject(t)
		project.submissionQuietPeriod = 0
		ready := readReady(t, workspace)
		artifacts := validChapterArtifacts(1)
		writeSubmission(t, workspace, ready, artifacts, false)
		manifestPath := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "manifest.json")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, []byte(strings.Repeat(" ", (64<<10)+1))...)
		if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		assertSubmissionInvalid(t, project, "max size")
	})

	t.Run("oversize chapter", func(t *testing.T) {
		project, _, workspace := newChapterReadyProject(t)
		project.submissionQuietPeriod = 0
		ready := readReady(t, workspace)
		artifacts := validChapterArtifacts(1)
		artifacts["chapter.md"] = []byte(strings.Repeat("x", protocol.DefaultMaxTextSize+1))
		writeSubmission(t, workspace, ready, artifacts, false)
		assertSubmissionInvalid(t, project, "max size")
	})

	t.Run("unknown artifact", func(t *testing.T) {
		project, _, workspace := newChapterReadyProject(t)
		project.submissionQuietPeriod = 0
		ready := readReady(t, workspace)
		artifacts := validChapterArtifacts(1)
		writeSubmission(t, workspace, ready, artifacts, false)
		inbox := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID)
		if err := os.WriteFile(filepath.Join(inbox, "extra.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSubmissionInvalid(t, project, "unknown file")
	})
}

func writeManifestJSON(t *testing.T, workspace string, ready readyView, manifest protocol.SubmissionManifest) {
	t.Helper()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "exchange", "inbox", ready.TaskID, ready.AttemptID, "manifest.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSubmissionInvalid(t *testing.T, project *Project, problemContains string) {
	t.Helper()
	status, err := project.ScanActiveSubmission()
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !strings.Contains(strings.ToLower(status.Problem), strings.ToLower(problemContains)) {
		t.Fatalf("status=%+v want INVALID containing %q", status, problemContains)
	}
}
