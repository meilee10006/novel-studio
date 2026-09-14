package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
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
