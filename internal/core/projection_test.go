package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotionProjectionReflectsCurrentAuthority(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章投影正文")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if err := project.WriteNotionProjection(); err != nil {
		t.Fatalf("WriteNotionProjection: %v", err)
	}
	var projection struct {
		ProjectID     string `json:"project_id"`
		CanonRoot     string `json:"canon_root"`
		Revision      int    `json:"revision"`
		LatestChapter int    `json:"latest_chapter"`
		ActiveTask    string `json:"active_task"`
		ActiveTarget  string `json:"active_target"`
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "projection", "notion.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &projection); err != nil {
		t.Fatal(err)
	}
	if projection.ProjectID != "book-1" || projection.CanonRoot != status.CanonRoot || projection.LatestChapter != 1 || projection.ActiveTask != "chapter" || projection.ActiveTarget != "chapter:2" {
		t.Fatalf("projection=%+v status=%+v", projection, status)
	}
	if projection.Revision <= 0 {
		t.Fatalf("projection revision=%d", projection.Revision)
	}
}

func TestServeProjectionFailureDoesNotBlockAcceptedChapter(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	project.submissionQuietPeriod = 0
	if err := os.RemoveAll(filepath.Join(workspace, "projection")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "projection"), []byte("not-a-directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSubmission(t, workspace, ready, revisionChapterArtifacts(1, "第一章即使投影失败也要验收"), false)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()
	waitForCondition(t, 3*time.Second, func() bool {
		current := readReady(t, workspace)
		return current.Target == "chapter:2"
	})
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve returned projection failure: %v", err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.CanonRoot == ready.BaseCanonRoot {
		t.Fatalf("chapter did not advance canon: %+v", status)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
