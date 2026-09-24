package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
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

func TestDesignInboxRejectsAbsoluteAndTraversalPaths(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "absolute", file: "/tmp/escape.json"},
		{name: "traversal", file: "../escape.json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace := newRequiredDesignProjectForTest(t)
			submissionID := "design-path-" + tc.name
			base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
			if err := os.MkdirAll(base, 0o755); err != nil {
				t.Fatal(err)
			}
			writeJSONFile(t, filepath.Join(base, "manifest.json"), protocol.DesignManifest{
				SchemaVersion:   protocol.MachineSchemaVersion,
				ProjectID:       "book-1",
				SubmissionID:    submissionID,
				ProtocolVersion: protocol.CurrentVersion,
				Operation:       "import",
				Files:           []string{tc.file},
			})
			status, err := project.ScanDesignSubmission(submissionID)
			if err != nil {
				t.Fatal(err)
			}
			if status.State != "INVALID" || !strings.Contains(strings.ToLower(status.Problem), "file list") {
				t.Fatalf("status=%+v", status)
			}
		})
	}
}

func TestDesignInboxRejectsNestedDirectory(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	submissionID := "design-nested"
	base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(filepath.Join(base, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(artifactImportEnvelope("creative_brief", nil, validCreativeBriefPayload()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "artifact.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(base, "manifest.json"), protocol.DesignManifest{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       "book-1",
		SubmissionID:    submissionID,
		ProtocolVersion: protocol.CurrentVersion,
		Operation:       "import",
		Files:           []string{"artifact.json"},
	})
	status, err := project.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !strings.Contains(strings.ToLower(status.Problem), "directory") {
		t.Fatalf("status=%+v", status)
	}
}

func TestDesignInboxRejectsInvalidUTF8(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	submissionID := "design-invalid-utf8"
	writeRawDesignSubmissionForSecurityTest(
		t, workspace, submissionID, []byte{0xff, 0xfe, 0xfd},
	)
	status, err := project.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "INVALID" || !strings.Contains(strings.ToLower(status.Problem), "utf") {
		t.Fatalf("status=%+v", status)
	}
}

func TestDesignInboxRejectsExcessiveJSONDepth(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	submissionID := "design-deep-json"
	deep := strings.Repeat("[", protocol.MaxJSONDepth+1) +
		"0" +
		strings.Repeat("]", protocol.MaxJSONDepth+1)
	raw := []byte(
		`{"kind":"artifact","artifact":{"schema_version":1,"artifact_type":"creative_brief","inputs":[],"sources":[],"payload":` +
			deep + `}}`,
	)
	writeRawDesignSubmissionForSecurityTest(t, workspace, submissionID, raw)
	result := processRawDesignSubmissionForSecurityTest(t, project, submissionID)
	if result.Result != "INVALID" || !strings.Contains(strings.ToLower(result.Problem), "depth") {
		t.Fatalf("result=%+v", result)
	}
}

func TestDesignInboxRejectsExcessiveJSONArrayLength(t *testing.T) {
	project, _, workspace := newRequiredDesignProjectForTest(t)
	submissionID := "design-wide-json"
	wide := "[" + strings.Repeat("0,", protocol.MaxJSONArrayLength) + "0]"
	raw := []byte(
		`{"kind":"artifact","artifact":{"schema_version":1,"artifact_type":"creative_brief","inputs":[],"sources":[],"payload":` +
			wide + `}}`,
	)
	writeRawDesignSubmissionForSecurityTest(t, workspace, submissionID, raw)
	result := processRawDesignSubmissionForSecurityTest(t, project, submissionID)
	if result.Result != "INVALID" || !strings.Contains(strings.ToLower(result.Problem), "array") {
		t.Fatalf("result=%+v", result)
	}
}

func TestServeIgnoresInvalidDesignSubmissionIDDirectory(t *testing.T) {
	project, local, workspace := newRequiredDesignProjectForTest(t)
	project.submissionQuietPeriod = 0
	submissionID := "INVALID!"
	base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(artifactImportEnvelope("creative_brief", nil, validCreativeBriefPayload()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "artifact.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(base, "manifest.json"), protocol.DesignManifest{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       "book-1",
		SubmissionID:    submissionID,
		ProtocolVersion: protocol.CurrentVersion,
		Operation:       "import",
		Files:           []string{"artifact.json"},
	})
	if err := project.servePassLocked(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "design", "result", submissionID+".json")); !os.IsNotExist(err) {
		t.Fatalf("invalid submission id produced result: %v", err)
	}
	if _, err := os.Stat(filepath.Join(local, "meta", "core", "design", "reconcile", submissionID+".json")); !os.IsNotExist(err) {
		t.Fatalf("invalid submission id produced reconcile record: %v", err)
	}
}

func writeRawDesignSubmissionForSecurityTest(
	t *testing.T,
	workspace, submissionID string,
	raw []byte,
) {
	t.Helper()
	base := filepath.Join(workspace, "exchange", "design", "inbox", submissionID)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "artifact.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, filepath.Join(base, "manifest.json"), protocol.DesignManifest{
		SchemaVersion:   protocol.MachineSchemaVersion,
		ProjectID:       "book-1",
		SubmissionID:    submissionID,
		ProtocolVersion: protocol.CurrentVersion,
		Operation:       "import",
		Files:           []string{"artifact.json"},
	})
}

func processRawDesignSubmissionForSecurityTest(
	t *testing.T,
	project *Project,
	submissionID string,
) domain.CoreDesignSubmissionResult {
	t.Helper()
	if _, err := project.ScanDesignSubmission(submissionID); err != nil {
		t.Fatal(err)
	}
	status, err := project.ScanDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "READY_TO_VALIDATE" {
		t.Fatalf("status=%+v", status)
	}
	result, err := project.ProcessDesignSubmission(submissionID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
