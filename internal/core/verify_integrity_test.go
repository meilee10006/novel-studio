package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyDetectsCurrentCanonTamper(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章原正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	chapterPath := filepath.Join(local, "meta", "core", "canon", "artifacts", "chapters", "000001", "chapter.md")
	if err := os.WriteFile(chapterPath, []byte("被篡改的正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !containsProblem(result.Problems, "canon") {
		t.Fatalf("Verify=%+v, want canon integrity failure", result)
	}
}

func containsProblem(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(strings.ToLower(item), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func TestVerifyDetectsBrokenActiveReceiptChain(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	settled := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章正文"))
	if settled.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", settled)
	}
	receipts, err := project.store.ListCoreReceipts()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, receipt := range receipts {
		if receipt.NewRoot != settled.NewCanonRoot {
			continue
		}
		receipt.PreviousRoot = "sha256:broken-parent"
		writeJSONFile(t, filepath.Join(local, "meta", "core", "receipts", receipt.AttemptID+".json"), receipt)
		found = true
		break
	}
	if !found {
		t.Fatal("active receipt not found")
	}
	result, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if result.OK || !containsProblem(result.Problems, "receipt") {
		t.Fatalf("Verify=%+v, want receipt chain failure", result)
	}
}

func TestVerifyAcceptsCurrentReceiptChainAfterRevisionReplay(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	for chapter := 1; chapter <= 2; chapter++ {
		got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(chapter, "旧分支正文"))
		if got.Result != "ACCEPTED" {
			t.Fatalf("original chapter %d=%+v", chapter, got)
		}
		ready = readReady(t, workspace)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	revision := startHistoricalRevision(t, project, workspace, status.CanonRoot, 1, "重写第一章")
	if got := submitAndSettleChapter(t, project, workspace, revision, revisionChapterArtifacts(1, "新分支第一章")); got.Result != "ACCEPTED" {
		t.Fatalf("revision chapter 1=%+v", got)
	}
	rebase := readReady(t, workspace)
	if got := submitAndSettleChapter(t, project, workspace, rebase, revisionChapterArtifacts(2, "新分支第二章")); got.Result != "ACCEPTED" {
		t.Fatalf("revision chapter 2=%+v", got)
	}
	result, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("Verify after revision replay=%+v", result)
	}
}

func TestVerifyRefusesConcurrentProjectWriter(t *testing.T) {
	project, local, workspace, ready := acceptedFoundationProject(t)
	if got := submitAndSettleChapter(t, project, workspace, ready, revisionChapterArtifacts(1, "第一章校验正文")); got.Result != "ACCEPTED" {
		t.Fatalf("chapter settlement=%+v", got)
	}
	lockReady := filepath.Join(t.TempDir(), "lock-ready")
	release := filepath.Join(t.TempDir(), "lock-release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectLockHelper$")
	cmd.Env = append(os.Environ(),
		"NOVEL_CORE_LOCK_HELPER=1",
		"NOVEL_CORE_LOCK_ROOT="+local,
		"NOVEL_CORE_LOCK_READY="+lockReady,
		"NOVEL_CORE_LOCK_RELEASE="+release,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(release, []byte("release"), 0o600)
		_ = cmd.Wait()
	}()
	waitForFile(t, lockReady)

	_, err := project.Verify()
	if !errors.Is(err, ErrProjectLocked) {
		t.Fatalf("Verify err=%v, want ErrProjectLocked", err)
	}
}
