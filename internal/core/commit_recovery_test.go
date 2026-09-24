package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
)

func TestFoundationCommitCrashRecoveryCompletesAtomically(t *testing.T) {
	for _, stage := range []string{"canon", "receipt", "production", "ready"} {
		t.Run(stage, func(t *testing.T) {
			project, local, workspace := newCapabilityPassedProject(t)
			if err := project.Reconcile(); err != nil {
				t.Fatal(err)
			}
			ready := readReady(t, workspace)
			artifacts := validFoundationArtifacts()

			fired := false
			project.commitFault = func(got string) error {
				if got == stage && !fired {
					fired = true
					return fmt.Errorf("simulated foundation crash after %s", stage)
				}
				return nil
			}
			if _, err := project.SettleFoundation(FoundationSubmission{
				Manifest:  manifestForReady(ready, artifacts),
				Artifacts: artifacts,
			}); err == nil {
				t.Fatalf("expected simulated crash at %s", stage)
			}

			reopened, err := OpenProject(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Reconcile(); err != nil {
				t.Fatalf("recover %s: %v", stage, err)
			}

			receipts, err := reopened.store.ListCoreReceipts()
			if err != nil {
				t.Fatal(err)
			}
			var foundationReceipts int
			for _, receipt := range receipts {
				if receipt.Result == "ACCEPTED" && receipt.PreviousRoot == "" {
					foundationReceipts++
					if receipt.FoundationDesignRoot != "" {
						t.Fatalf("legacy foundation receipt unexpectedly bound design root %q", receipt.FoundationDesignRoot)
					}
				}
			}
			if foundationReceipts != 1 {
				t.Fatalf("foundation accepted receipts=%d receipts=%+v", foundationReceipts, receipts)
			}

			head, err := reopened.store.LoadCoreCanonHead()
			if err != nil {
				t.Fatal(err)
			}
			if head == nil || head.Root == "" || head.ParentRoot != "" {
				t.Fatalf("canon head=%+v", head)
			}
			state, err := reopened.store.LoadCoreProductionState()
			if err != nil {
				t.Fatal(err)
			}
			if state == nil || state.CanonRoot != head.Root ||
				state.ActiveTask == nil || state.ActiveTask.Kind != "chapter" || state.ActiveTask.Target != "chapter:1" ||
				state.ActiveAttempt == nil {
				t.Fatalf("production state=%+v head=%+v", state, head)
			}
			firstAttemptID := state.ActiveAttempt.AttemptID
			next := readReady(t, workspace)
			if next.TaskKind != "chapter" || next.Target != "chapter:1" || next.AttemptID != firstAttemptID ||
				next.BaseCanonRoot != head.Root {
				t.Fatalf("READY=%+v state=%+v", next, state)
			}

			if err := reopened.Reconcile(); err != nil {
				t.Fatalf("second reconcile %s: %v", stage, err)
			}
			secondReceipts, err := reopened.store.ListCoreReceipts()
			if err != nil {
				t.Fatal(err)
			}
			if len(secondReceipts) != len(receipts) {
				t.Fatalf("second reconcile changed receipt count: before=%d after=%d", len(receipts), len(secondReceipts))
			}
			secondHead, err := reopened.store.LoadCoreCanonHead()
			if err != nil {
				t.Fatal(err)
			}
			secondState, err := reopened.store.LoadCoreProductionState()
			if err != nil {
				t.Fatal(err)
			}
			if secondHead == nil || secondHead.Root != head.Root ||
				secondState == nil || secondState.CanonRoot != state.CanonRoot ||
				secondState.ActiveAttempt == nil || secondState.ActiveAttempt.AttemptID != firstAttemptID {
				t.Fatalf("second reconcile changed authority: head=%+v state=%+v", secondHead, secondState)
			}
		})
	}
}

func TestDesignFoundationCommitCrashRecoveryCompletesAtomically(t *testing.T) {
	for _, stage := range []string{"canon", "receipt", "production", "ready"} {
		t.Run(stage, func(t *testing.T) {
			project, local, workspace, fixture := makeFoundationReadyProjectForTest(t)

			if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
				t.Fatalf("foundation_ready unexpectedly published READY before reconcile: %v", err)
			}

			fired := false
			project.commitFault = func(got string) error {
				if got == stage && !fired {
					fired = true
					return fmt.Errorf("simulated design foundation crash after %s", stage)
				}
				return nil
			}
			if err := project.Reconcile(); err == nil {
				t.Fatalf("expected simulated crash at %s", stage)
			}
			if stage != "ready" {
				if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
					t.Fatalf("internal foundation leaked READY before recovery at %s: %v", stage, err)
				}
			}

			journals, err := project.store.ListCoreCommitJournals()
			if err != nil {
				t.Fatal(err)
			}
			var foundationAttemptID string
			for _, journal := range journals {
				if journal.Kind == "foundation" {
					if foundationAttemptID != "" {
						t.Fatalf("multiple foundation journals=%+v", journals)
					}
					foundationAttemptID = journal.AttemptID
				}
			}
			if foundationAttemptID == "" {
				t.Fatalf("foundation journal missing: %+v", journals)
			}

			reopened, err := OpenProject(local)
			if err != nil {
				t.Fatal(err)
			}
			if err := reopened.Reconcile(); err != nil {
				t.Fatalf("recover %s: %v", stage, err)
			}
			if err := reopened.Reconcile(); err != nil {
				t.Fatalf("second recover %s: %v", stage, err)
			}

			designHead, err := reopened.store.LoadCoreDesignHead()
			if err != nil {
				t.Fatal(err)
			}
			if designHead == nil || designHead.DesignRoot != fixture.FoundationRoot ||
				designHead.Checkpoint != domain.DesignCheckpointFoundationReady {
				t.Fatalf("design head=%+v fixture=%+v", designHead, fixture)
			}

			receipts, err := reopened.store.ListCoreReceipts()
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

			state, err := reopened.store.LoadCoreProductionState()
			if err != nil {
				t.Fatal(err)
			}
			if state == nil || state.ActiveTask == nil || state.ActiveTask.Kind != "chapter" ||
				state.ActiveTask.Target != "chapter:1" || state.ActiveAttempt == nil ||
				attemptInputSource(state.ActiveAttempt) != "drive" {
				t.Fatalf("production state=%+v", state)
			}
			ready := readReady(t, workspace)
			if ready.TaskKind != "chapter" || ready.Target != "chapter:1" ||
				ready.AttemptID != state.ActiveAttempt.AttemptID ||
				ready.BaseCanonRoot != state.CanonRoot {
				t.Fatalf("READY=%+v state=%+v", ready, state)
			}
			if _, err := os.Stat(filepath.Join(workspace, "exchange", "result", foundationAttemptID+".json")); !os.IsNotExist(err) {
				t.Fatalf("internal foundation created external result: %v", err)
			}

			recomputed, err := reopened.RecomputeCanonRoot()
			if err != nil {
				t.Fatal(err)
			}
			if recomputed != state.CanonRoot {
				t.Fatalf("recomputed root=%q state root=%q", recomputed, state.CanonRoot)
			}
		})
	}
}
