package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/protocol"
)

func TestInitProjectSeparatesAuthorityAndWorkspace(t *testing.T) {
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	if _, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace}); err != nil {
		t.Fatalf("InitProject: %v", err)
	}
	for _, path := range []string{
		filepath.Join(local, "meta", "core", "project.json"),
		filepath.Join(workspace, "project.json"),
		filepath.Join(workspace, "CHATGPT_PROTOCOL.md"),
		filepath.Join(workspace, "setup", "capability-challenge.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("READY must not exist before capability ack: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(workspace, "CHATGPT_PROTOCOL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != protocol.RenderChatGPTProtocol("book-1") {
		t.Fatal("CHATGPT_PROTOCOL.md was not generated from canonical renderer")
	}
}

func TestInitProjectRejectsSameOrNestedRoots(t *testing.T) {
	root := t.TempDir()
	cases := [][2]string{{root, root}, {filepath.Join(root, "local"), root}, {root, filepath.Join(root, "drive")}}
	for _, tc := range cases {
		if _, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: tc[0], WorkspaceRoot: tc[1]}); err == nil {
			t.Fatalf("InitProject local=%q workspace=%q unexpectedly succeeded", tc[0], tc[1])
		}
	}
}

func TestCapabilityAckMustMatchNonceAndUTF8Probe(t *testing.T) {
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	project, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}

	var challenge struct {
		Nonce         string `json:"nonce"`
		MarkdownProbe string `json:"markdown_probe"`
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "setup", "capability-challenge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &challenge); err != nil {
		t.Fatal(err)
	}

	writeAck := func(nonce string) {
		t.Helper()
		ack, _ := json.Marshal(map[string]any{
			"project_id": "book-1", "protocol_version": protocol.CurrentVersion, "nonce": nonce,
			"capabilities": map[string]bool{"read": true, "write_utf8_json": true, "write_utf8_md": true},
		})
		if err := os.WriteFile(filepath.Join(workspace, "setup", "capability-ack.json"), ack, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "setup", "capability-write-test.md"), []byte(challenge.MarkdownProbe), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeAck("wrong")
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Capability != "invalid" {
		t.Fatalf("capability=%q want invalid", status.Capability)
	}

	writeAck(challenge.Nonce)
	status, err = project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Capability != "passed" {
		t.Fatalf("capability=%q problem=%q want passed", status.Capability, status.CapabilityProblem)
	}
}

func TestCapabilityMissingAckRemainsPendingAndNoReady(t *testing.T) {
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	project, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.Capability != "pending" {
		t.Fatalf("capability=%q want pending", status.Capability)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("READY must not exist while pending: %v", err)
	}
}

func TestInitProjectMarksCoreInitializedAndKeepsChallengeStable(t *testing.T) {
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	project, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Initialized || status.ProjectID != "book-1" {
		t.Fatalf("status after init=%+v", status)
	}
	first, err := os.ReadFile(filepath.Join(workspace, "setup", "capability-challenge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace}); err != nil {
		t.Fatalf("second InitProject: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(workspace, "setup", "capability-challenge.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("idempotent init changed capability challenge")
	}
}

func TestDriveWorkspaceCannotOverrideLocalProjectIdentity(t *testing.T) {
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	project, err := InitProject(InitOptions{ProjectID: "book-1", LocalRoot: local, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	fakeDir := filepath.Join(workspace, "meta", "core")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeDir, "project.json"), []byte(`{"project_id":"evil"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := project.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status.ProjectID != "book-1" {
		t.Fatalf("Drive changed local project identity: %+v", status)
	}
}
