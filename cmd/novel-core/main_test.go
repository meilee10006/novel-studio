package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusAndVerifyNeedNoModelCredentials(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "LITELLM_API_KEY"} {
		t.Setenv(key, "")
	}
	root := t.TempDir()
	for _, args := range [][]string{{"status", "--project", root}, {"verify", "--project", root}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("run(%v)=%d stderr=%s", args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), root) {
			t.Fatalf("run(%v) output missing project root: %s", args, stdout.String())
		}
	}
}

func TestNovelCoreDependencyGraphExcludesAIRuntime(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	deps := string(out)
	for _, forbidden := range []string{
		"github.com/voocel/agentcore",
		"github.com/chenhongyang/novel-studio/internal/agents",
		"github.com/chenhongyang/novel-studio/internal/llmcodex",
	} {
		if strings.Contains(deps, forbidden) {
			t.Fatalf("forbidden dependency %q in novel-core graph", forbidden)
		}
	}
}

func TestInitNeedsNoModelCredentialsAndCreatesCapabilityChallenge(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "LITELLM_API_KEY"} {
		t.Setenv(key, "")
	}
	parent := t.TempDir()
	local := filepath.Join(parent, "local")
	workspace := filepath.Join(parent, "drive")
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--project", local, "--workspace", workspace, "--project-id", "book-1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, "setup", "capability-challenge.json")); err != nil {
		t.Fatalf("capability challenge missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "exchange", "READY.json")); !os.IsNotExist(err) {
		t.Fatalf("READY must not exist before ack: %v", err)
	}
}

func TestExportCommandIsAvailable(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "LITELLM_API_KEY"} {
		t.Setenv(key, "")
	}
	root := t.TempDir()
	out := filepath.Join(t.TempDir(), "book.md")
	var stdout, stderr bytes.Buffer
	code := run([]string{"export", "--project", root, "--out", out}, &stdout, &stderr)
	if code == 2 || strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("export command not routed: code=%d stderr=%s", code, stderr.String())
	}
}

func TestRestoreCommandIsAvailable(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "LITELLM_API_KEY"} {
		t.Setenv(key, "")
	}
	backup := filepath.Join(t.TempDir(), "missing-backup")
	target := filepath.Join(t.TempDir(), "restored")
	var stdout, stderr bytes.Buffer
	code := run([]string{"restore", "--backup", backup, "--project", target}, &stdout, &stderr)
	if code == 2 || strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("restore command not routed: code=%d stderr=%s", code, stderr.String())
	}
}

func TestMigrateCommandIsAvailable(t *testing.T) {
	root := t.TempDir()
	backup := filepath.Join(t.TempDir(), "pre-migration")
	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "--project", root, "--backup", backup}, &stdout, &stderr)
	if code == 2 || strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("migrate command not routed: code=%d stderr=%s", code, stderr.String())
	}
}
