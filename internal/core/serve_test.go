package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func TestServePollsAndSettlesWithoutFileEvent(t *testing.T) {
	project, _, workspace := newCapabilityPassedProject(t)
	project.submissionQuietPeriod = 0
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Serve: %v", err)
		}
	}()

	waitForFile(t, workspace+"/exchange/READY.json")
	ready := readReady(t, workspace)
	if ready.TaskKind != "foundation" {
		t.Fatalf("initial READY=%+v", ready)
	}
	writeSubmission(t, workspace, ready, validFoundationArtifacts(), false)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current := readReady(t, workspace)
		if current.TaskKind == "chapter" && current.Target == "chapter:1" {
			status, err := project.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.CanonRoot == "" {
				t.Fatal("serve advanced READY without canon root")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("serve did not settle foundation on a later polling scan")
}

func TestServeHoldsProjectWriterLockForLifetime(t *testing.T) {
	project, local, workspace := newCapabilityPassedProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 20*time.Millisecond) }()
	waitForFile(t, workspace+"/exchange/READY.json")

	other, err := OpenProject(local)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.Reconcile(); !errors.Is(err, ErrProjectLocked) {
		cancel()
		<-done
		t.Fatalf("second writer err=%v, want ErrProjectLocked", err)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if err := other.Reconcile(); err != nil {
		t.Fatalf("writer lock not released after serve stopped: %v", err)
	}
}

func TestServePollsControlInboxWithoutFileEvent(t *testing.T) {
	project, _, workspace, current := acceptedFoundationProject(t)
	project.submissionQuietPeriod = 0
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Serve: %v", err)
		}
	}()

	msgID := "ctrl-serve-poll"
	writeControlMessage(t, workspace, msgID, map[string]any{
		"kind": "author_directive", "base_canon_root": current.BaseCanonRoot,
		"directive_scope": "future_plan", "instruction": "后续减少追逐",
	})
	resultPath := workspace + "/exchange/control/result/" + msgID + ".json"
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var result ControlResult
		if err := readJSONFileIfExists(resultPath, &result); err == nil && result.Result == "ACCEPTED" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("serve did not process control inbox on a later polling scan")
}

func readJSONFileIfExists(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func TestServePollsAndSettlesChapterWithoutFileEvent(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	project.submissionQuietPeriod = 0
	before, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Serve: %v", err)
		}
	}()

	writeSubmission(t, workspace, ready, validChapterArtifacts(1), false)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		current := readReady(t, workspace)
		if current.TaskKind == "chapter" && current.Target == "chapter:2" {
			status, err := project.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.CanonRoot == "" || status.CanonRoot == before.CanonRoot {
				t.Fatalf("serve did not advance canon: before=%+v after=%+v", before, status)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("serve did not settle chapter on a later polling scan")
}

func TestServeWaitsWhileCapabilityPending(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := InitProject(InitOptions{ProjectID: "serve-capability-pending", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()

	select {
	case err := <-done:
		t.Fatalf("Serve exited before capability ack: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	if _, err := os.Stat(workspace + "/exchange/READY.json"); !os.IsNotExist(err) {
		cancel()
		<-done
		t.Fatalf("READY exists before capability ack: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve after cancel: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop after context cancellation")
	}
}

func TestInternalDesignFoundationAttemptRejectsDriveSubmission(t *testing.T) {
	project, _, workspace, _ := makeFoundationReadyProjectForTest(t)
	projectState, err := project.store.LoadCoreProjectState()
	if err != nil {
		t.Fatal(err)
	}
	head := mustDesignHeadForTest(t, project)
	state := newCoreProductionState()
	if err := project.ensureDesignFoundationAttemptLocked(projectState, state, head); err != nil {
		t.Fatal(err)
	}

	stored, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.ActiveAttempt == nil ||
		attemptInputSource(stored.ActiveAttempt) != "design" ||
		stored.ActiveAttempt.InputRef != head.DesignRoot {
		t.Fatalf("stored production=%+v head=%+v", stored, head)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("internal foundation published READY: %v", err)
	}
	if _, err := project.ScanActiveSubmission(); err == nil ||
		!strings.Contains(err.Error(), "does not accept Drive submission") {
		t.Fatalf("ScanActiveSubmission err=%v", err)
	}
}

func TestRequiredDesignReconcileWaitsForFoundationReady(t *testing.T) {
	t.Run("no design head", func(t *testing.T) {
		project, _, workspace := newRequiredDesignProjectForTest(t)
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		state, err := project.store.LoadCoreProductionState()
		if err != nil {
			t.Fatal(err)
		}
		if state != nil {
			t.Fatalf("required project created production state before design head: %+v", state)
		}
		if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
			t.Fatalf("required project published READY before design head: %v", err)
		}
	})

	t.Run("story locked", func(t *testing.T) {
		project, _, workspace, fixture := makeStoryLockedProjectForTest(t)
		if fixture.StoryRoot == "" {
			t.Fatal("story root is empty")
		}
		if err := project.Reconcile(); err != nil {
			t.Fatal(err)
		}
		state, err := project.store.LoadCoreProductionState()
		if err != nil {
			t.Fatal(err)
		}
		if state != nil {
			t.Fatalf("required project created production state at story_locked: %+v", state)
		}
		if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
			t.Fatalf("story_locked project published Foundation READY: %v", err)
		}
	})
}

func TestDesignFoundationReconcileSettlesIntoCanonAndPublishesChapterReady(t *testing.T) {
	project, _, workspace, fixture := makeFoundationReadyProjectForTest(t)
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("foundation_ready unexpectedly has READY before reconcile: %v", err)
	}
	preStatus, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if preStatus.DesignMode != domain.DesignModeRequired ||
		preStatus.DesignHead != fixture.FoundationRoot ||
		preStatus.DesignCheckpoint != domain.DesignCheckpointFoundationReady ||
		preStatus.FoundationDesignRoot != "" ||
		preStatus.CanonRoot != "" {
		t.Fatalf("pre-canon status=%+v fixture=%+v", preStatus, fixture)
	}

	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}

	receipts, err := project.store.ListCoreReceipts()
	if err != nil {
		t.Fatal(err)
	}
	accepted := foundationAcceptedReceiptsForTest(receipts)
	if len(accepted) != 1 {
		t.Fatalf("foundation accepted receipts=%+v all=%+v", accepted, receipts)
	}
	if accepted[0].FoundationDesignRoot != fixture.FoundationRoot {
		t.Fatalf("receipt foundation design root=%q want %q", accepted[0].FoundationDesignRoot, fixture.FoundationRoot)
	}

	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.DesignMode != domain.DesignModeRequired ||
		status.DesignHead != fixture.FoundationRoot ||
		status.DesignCheckpoint != domain.DesignCheckpointFoundationReady ||
		status.FoundationDesignRoot != fixture.FoundationRoot {
		t.Fatalf("status=%+v fixture=%+v", status, fixture)
	}

	state, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.CanonRoot == "" ||
		state.ActiveTask == nil || state.ActiveTask.Kind != "chapter" || state.ActiveTask.Target != "chapter:1" ||
		state.ActiveAttempt == nil || attemptInputSource(state.ActiveAttempt) != "drive" {
		t.Fatalf("production state=%+v", state)
	}
	ready := readReady(t, workspace)
	if ready.TaskKind != "chapter" || ready.Target != "chapter:1" ||
		ready.AttemptID != state.ActiveAttempt.AttemptID ||
		ready.BaseCanonRoot != state.CanonRoot {
		t.Fatalf("READY=%+v state=%+v", ready, state)
	}

	if _, err := os.Stat(filepath.Join(
		workspace, "exchange", "result", accepted[0].AttemptID+".json",
	)); !os.IsNotExist(err) {
		t.Fatalf("internal foundation created external result: %v", err)
	}
}

func TestServePollsRequiredDesignInboxWithoutFileEvent(t *testing.T) {
	local := t.TempDir()
	workspace := t.TempDir()
	project, err := InitProject(InitOptions{
		ProjectID:     "serve-design-idle",
		LocalRoot:     local,
		WorkspaceRoot: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeCapabilityAckForTest(t, workspace)
	project.submissionQuietPeriod = 0

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- project.Serve(ctx, 10*time.Millisecond) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("Serve: %v", err)
		}
	}()

	submissionID := "design-serve-import"
	writeDesignSubmissionForTest(t, project, submissionID, "import", map[string]any{
		"brief.json": artifactImportEnvelope("creative_brief", nil, validCreativeBriefPayload()),
	})
	resultPath := filepath.Join(workspace, "exchange", "design", "result", submissionID+".json")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var result map[string]any
		if err := readJSONFileIfExists(resultPath, &result); err == nil && result["result"] == "IMPORTED" {
			if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
				t.Fatalf("required design import published production READY: %v", err)
			}
			status, err := project.Status()
			if err != nil {
				t.Fatal(err)
			}
			if status.DesignMode != "required" || status.CanonRoot != "" || status.ActiveAttemptID != "" {
				t.Fatalf("status=%+v", status)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("serve did not process required design inbox on polling scan")
}
