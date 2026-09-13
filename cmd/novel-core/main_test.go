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
