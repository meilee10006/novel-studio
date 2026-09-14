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

func TestExpiredLegacySubsystemsAreRemoved(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	retired := []string{
		"evals", "models", "quality", "services", "skills",
		"internal/aigc", "internal/aitrace", "internal/editor/rules", "internal/logger",
		"internal/models", "internal/notify", "internal/rag", "internal/reviewreport",
		"internal/utils", "internal/version", "internal/writer/prompts",
	}
	for _, rel := range retired {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("expired legacy subsystem still exists: %s", rel)
		}
	}
	for _, rel := range []string{"Dockerfile", ".github/workflows/ci.yml"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, forbidden := range []string{"third_party/litellm", "services/dashboard", "Dashboard tests", "Dashboard client tests"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s still references expired subsystem %q", rel, forbidden)
			}
		}
	}
}

func TestProviderFreeCoreUsesDedicatedStore(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "internal", "core", "project.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "*store.CoreStore") {
		t.Error("core Project does not use dedicated store.CoreStore")
	}
	for _, forbidden := range []string{"*store.Store", "store.NewStore", ".Progress", ".Checkpoints", ".CheckConsistency"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("provider-free Core still depends on legacy Store surface %q", forbidden)
		}
	}
}

func TestProviderFreeStoreContainsOnlyCorePersistence(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	entries, err := os.ReadDir(filepath.Join(root, "internal", "store"))
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"core_store.go": true, "io.go": true, "core_project.go": true,
		"core_production.go": true, "core_control.go": true,
		"core_submission.go": true, "core_commit.go": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		if !allowed[entry.Name()] {
			t.Errorf("legacy store file remains: %s", entry.Name())
		}
	}
}

func TestProviderFreeDomainContainsOnlyCoreTypes(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	entries, err := os.ReadDir(filepath.Join(root, "internal", "domain"))
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"core_commit.go":     true,
		"core_control.go":    true,
		"core_longform.go":   true,
		"core_production.go": true,
		"core_project.go":    true,
		"core_submission.go": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		if !allowed[entry.Name()] {
			t.Errorf("legacy domain file remains: %s", entry.Name())
		}
	}
	for _, dir := range []string{"internal/testutil", "internal/rules", "internal/stylestat"} {
		if _, err := os.Stat(filepath.Join(root, dir)); err == nil {
			t.Errorf("legacy domain-support package remains: %s", dir)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	projectRaw, err := os.ReadFile(filepath.Join(root, "internal", "core", "project.go"))
	if err != nil {
		t.Fatal(err)
	}
	projectText := string(projectRaw)
	for _, forbidden := range []string{"NovelName", "domain.Phase", "CurrentChapter", "TotalChapters", "CompletedChapters"} {
		if strings.Contains(projectText, forbidden) {
			t.Errorf("legacy progress status surface remains: %q", forbidden)
		}
	}
}

func TestProviderFreeRepositoryHasNoOrphanLegacyScaffolding(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	if _, err := os.Stat(filepath.Join(root, "internal", "errs")); err == nil {
		t.Error("orphan internal/errs package remains")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "/novel-core") {
		t.Error(".gitignore does not ignore the supported novel-core binary")
	}
	for _, forbidden := range []string{"/novel-studio", ".novel-studio/", "models/embedding", "Claude Code", ".claude/"} {
		if strings.Contains(text, forbidden) {
			t.Errorf(".gitignore still contains retired scaffolding %q", forbidden)
		}
	}
}
