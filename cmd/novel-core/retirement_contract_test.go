package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyAIRuntimeIsRetiredFromRepository(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	retired := []string{
		"cmd/novel-studio",
		"internal/agents",
		"internal/bootstrap",
		"internal/diag",
		"internal/entry/headless",
		"internal/entry/startup",
		"internal/eval",
		"internal/host",
		"internal/llmcodex",
		"internal/modelinput",
		"internal/tools",
		"internal/userrules",
		"internal/writer/sampler",
	}
	for _, rel := range retired {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("retired AI runtime path still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}

	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(mod)
	for _, forbidden := range []string{"github.com/voocel/agentcore", "github.com/voocel/litellm", "replace github.com/voocel/litellm"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("go.mod still contains retired runtime dependency %q", forbidden)
		}
	}
}

func TestRetiredRuntimeHasNoExecutableReferences(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	retiredFiles := []string{
		"scripts/test-agents-race-shard.sh",
		"scripts/test-agents-race-shard-test.sh",
		"scripts/validate_skill_context.py",
		"scripts/index_workspace_assets.py",
		"config.example.jsonc",
	}
	for _, rel := range retiredFiles {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("retired executable/config path still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(ci)
	for _, forbidden := range []string{"race_agents", "test-agents-race-shard", "./internal/tools", "./internal/agents", "cmd/novel-studio"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("CI still references retired runtime %q", forbidden)
		}
	}
}
