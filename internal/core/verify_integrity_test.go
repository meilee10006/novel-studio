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

func TestVerifyDetectsDesignArtifactTamper(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	ref := currentStoryConceptRefForTest(t, project)
	_, digest, err := parseDesignArtifactRef(ref)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, "meta", "core", "design", "objects", digest+".json")
	if err := os.WriteFile(path, []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "design artifact") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsDesignBundleTamper(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	head := mustDesignHeadForTest(t, project)
	commit, err := project.store.LoadCoreDesignCommit(head.DesignRoot)
	if err != nil || commit == nil {
		t.Fatalf("commit=%+v err=%v", commit, err)
	}
	digest, err := parseDesignBundleRef(commit.BundleRef)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, "meta", "core", "design", "bundles", digest+".json")
	writeJSONFile(t, path, map[string]any{
		"schema_version": 1,
		"selections":     map[string]string{},
	})
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "design bundle") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsDesignCommitTamper(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	head := mustDesignHeadForTest(t, project)
	commit, err := project.store.LoadCoreDesignCommit(head.DesignRoot)
	if err != nil || commit == nil {
		t.Fatalf("commit=%+v err=%v", commit, err)
	}
	commit.CheckpointPolicyVersion++
	path := filepath.Join(local, "meta", "core", "design", "commits", head.DesignRoot+".json")
	writeJSONFile(t, path, commit)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "design commit") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsDesignHeadMissingCommit(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	head := mustDesignHeadForTest(t, project)
	head.DesignRoot = strings.Repeat("0", 64)
	path := filepath.Join(local, "meta", "core", "design", "HEAD.json")
	writeJSONFile(t, path, head)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "design head") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsFoundationReceiptDesignRootMismatch(t *testing.T) {
	tests := []struct {
		name string
		root func(project *Project, foundationRoot string) string
	}{
		{
			name: "other existing design root",
			root: func(project *Project, foundationRoot string) string {
				head := mustDesignHeadForTest(t, project)
				commit, err := project.store.LoadCoreDesignCommit(head.DesignRoot)
				if err != nil || commit == nil || commit.ParentDesignRoot == "" {
					t.Fatalf("commit=%+v err=%v", commit, err)
				}
				return commit.ParentDesignRoot
			},
		},
		{
			name: "missing design root",
			root: func(project *Project, foundationRoot string) string {
				return strings.Repeat("f", 64)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, local, _, foundationRoot := acceptedRequiredDesignProjectForTest(t)
			receipts, err := project.store.ListCoreReceipts()
			if err != nil {
				t.Fatal(err)
			}
			accepted := foundationAcceptedReceiptsForTest(receipts)
			if len(accepted) != 1 {
				t.Fatalf("accepted foundation receipts=%+v", accepted)
			}
			receipt := accepted[0]
			receipt.FoundationDesignRoot = tc.root(project, foundationRoot)
			writeJSONFile(
				t,
				filepath.Join(local, "meta", "core", "receipts", receipt.AttemptID+".json"),
				receipt,
			)
			verification, err := project.Verify()
			if err != nil {
				t.Fatal(err)
			}
			if verification.OK || !containsProblem(verification.Problems, "foundation design root") {
				t.Fatalf("verification=%+v", verification)
			}
		})
	}
}

func TestVerifyAcceptsFoundationReadyBeforeFirstCanon(t *testing.T) {
	project, _, _, _ := makeFoundationReadyProjectForTest(t)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if !verification.OK {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsGhostDesignCommitOutsideHeadHistory(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	head := mustDesignHeadForTest(t, project)
	commit, err := project.store.LoadCoreDesignCommit(head.DesignRoot)
	if err != nil || commit == nil {
		t.Fatalf("commit=%+v err=%v", commit, err)
	}
	ghost := *commit
	ghost.CheckpointPolicyVersion += 100
	ghostRoot, err := computeDesignRoot(ghost)
	if err != nil {
		t.Fatal(err)
	}
	if ghostRoot == head.DesignRoot {
		t.Fatal("ghost root unexpectedly equals head")
	}
	writeJSONFile(
		t,
		filepath.Join(local, "meta", "core", "design", "commits", ghostRoot+".json"),
		ghost,
	)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "outside authoritative history") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyDetectsDesignReceiptFilenameSubmissionMismatch(t *testing.T) {
	project, local, _, _ := acceptedRequiredDesignProjectForTest(t)
	receipts, err := project.store.ListCoreDesignReceipts()
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) == 0 {
		t.Fatal("no design receipts")
	}
	receipt := receipts[0]
	originalID := receipt.SubmissionID
	receipt.SubmissionID = originalID + "-tampered"
	writeJSONFile(
		t,
		filepath.Join(local, "meta", "core", "design", "receipts", originalID+".json"),
		receipt,
	)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "design receipt") {
		t.Fatalf("verification=%+v", verification)
	}
}

func TestVerifyRejectsLegacyFoundationReceiptWithDesignRoot(t *testing.T) {
	project, local, _, _ := acceptedFoundationProject(t)
	receipts, err := project.store.ListCoreReceipts()
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) == 0 {
		t.Fatal("no legacy receipts")
	}
	receipt := receipts[0]
	receipt.FoundationDesignRoot = strings.Repeat("a", 64)
	writeJSONFile(
		t,
		filepath.Join(local, "meta", "core", "receipts", receipt.AttemptID+".json"),
		receipt,
	)
	verification, err := project.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if verification.OK || !containsProblem(verification.Problems, "legacy foundation receipt") {
		t.Fatalf("verification=%+v", verification)
	}
}
