package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func TestReconcileRefreshesChatGPTProtocolWithoutMovingAuthority(t *testing.T) {
	project, _, workspace, _ := acceptedFoundationProject(t)

	before, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if before == nil || before.ActiveTask == nil || before.ActiveAttempt == nil {
		t.Fatalf("before=%+v", before)
	}

	protocolPath := filepath.Join(workspace, "CHATGPT_PROTOCOL.md")
	if err := os.WriteFile(protocolPath, []byte("stale protocol projection\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != protocol.RenderChatGPTProtocol("book-1") {
		t.Fatalf("protocol projection was not refreshed")
	}

	after, err := project.store.LoadCoreProductionState()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("reconcile changed production authority:\nbefore=%+v\nafter=%+v", before, after)
	}

	infoBefore, err := os.Stat(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	infoAfter, err := os.Stat(protocolPath)
	if err != nil {
		t.Fatal(err)
	}
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Fatalf("identical protocol projection was rewritten: before=%s after=%s", infoBefore.ModTime(), infoAfter.ModTime())
	}
}
