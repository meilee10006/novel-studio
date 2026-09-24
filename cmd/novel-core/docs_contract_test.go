package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderFreeDocumentationContract(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	docs := []string{"README.md", "README_EN.md", "README-TECHNICAL.md"}
	required := []string{"novel-core", "ChatGPT App", "Google Drive", "Drive Desktop"}
	forbidden := []string{"novel-studio --pipeline", "At least one working text-model provider", "至少配置一个可用的文本模型 provider"}
	for _, rel := range docs {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(raw)
		for _, want := range required {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q", rel, want)
			}
		}
		for _, bad := range forbidden {
			if strings.Contains(text, bad) {
				t.Errorf("%s still contains retired workflow %q", rel, bad)
			}
		}
	}
	readme, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if !strings.Contains(string(readme), "Xiaoyangy/novel-studio") || !strings.Contains(string(readme), "Apache-2.0") {
		t.Error("README.md must retain upstream attribution and Apache-2.0 notice")
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(license), "Apache License") || !strings.Contains(string(license), "Version 2.0") {
		t.Error("LICENSE is not Apache-2.0")
	}
}

func TestForkReleaseCoordinates(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	checks := map[string][]string{
		".goreleaser.yml":               {"owner: meilee10006", "main: ./cmd/novel-core"},
		"scripts/install.sh":            {"REPO=\"meilee10006/novel-studio\"", "BIN=\"novel-core\"", "raw.githubusercontent.com/meilee10006/novel-studio/provider-free-novel-core/scripts/install.sh"},
		"docker-compose.yml":            {"ghcr.io/meilee10006/novel-studio:latest"},
		".github/workflows/release.yml": {"ghcr.io/meilee10006/novel-studio:"},
	}
	for rel, wants := range checks {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(raw)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing fork release coordinate %q", rel, want)
			}
		}
	}
}

func TestReleaseChecklistSeparatesAutomatedAndManualAcceptance(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "provider-free-release-checklist.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"## Automated acceptance", "## Manual ChatGPT product acceptance", "NOT RUN", "A release is not product-accepted until all manual rows are PASS"} {
		if !strings.Contains(text, want) {
			t.Errorf("release checklist missing %q", want)
		}
	}
}

func TestExpiredRuntimeDocumentationIsRemoved(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	retired := []string{
		"docs/architecture.md", "docs/architecture-overview.html", "docs/engineering-overview.html",
		"docs/subscription-and-pipeline-setup.md", "docs/writing-review-workflow.md",
		"docs/context-management.md", "docs/data-lifecycle-and-progression.md",
		"docs/observability.md", "docs/project-structure.md", "docs/design-audits",
		"docs/assets", "assets/prompts", "scripts/shared_skill_files.json",
		"README-20260714.md", "scripts/check_chapter_wordcount.py", "scripts/novel.png", "scripts/sample.gif",
	}
	for _, rel := range retired {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("expired content still exists: %s", rel)
		}
	}
	deconReadme, err := os.ReadFile(filepath.Join(root, "deconstruction-library", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	decon := string(deconReadme)
	for _, want := range []string{"optional research workspace", "not read by novel-core", "never authoritative"} {
		if !strings.Contains(decon, want) {
			t.Errorf("deconstruction README missing %q", want)
		}
	}

	assetReadme, err := os.ReadFile(filepath.Join(root, "assets", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(assetReadme)
	for _, want := range []string{"optional writing references", "not loaded by novel-core", "never authoritative"} {
		if !strings.Contains(text, want) {
			t.Errorf("assets README missing %q", want)
		}
	}
	for _, forbidden := range []string{"internal/aigc", "internal/rag", "Coordinator", "Drafter"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("assets README still documents retired runtime %q", forbidden)
		}
	}
}

func TestProviderFreePlanAndSpecMatchImplementedCLIAndAuthority(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	planRaw, err := os.ReadFile(filepath.Join(root, "docs", "superpowers", "plans", "2026-09-13-provider-free-novel-core.md"))
	if err != nil {
		t.Fatal(err)
	}
	plan := string(planRaw)
	if !strings.Contains(plan, "go run ./cmd/novel-core verify --project <fixture-local>") {
		t.Error("provider-free plan does not show the implemented verify --project CLI")
	}
	if strings.Contains(plan, "verify --local <fixture-local> --workspace <fixture-workspace>") {
		t.Error("provider-free plan still shows the retired verify --local/--workspace CLI")
	}

	specRaw, err := os.ReadFile(filepath.Join(root, "docs", "superpowers", "specs", "2026-09-13-provider-free-novel-core-design.md"))
	if err != nil {
		t.Fatal(err)
	}
	spec := string(specRaw)
	for _, stale := range []string{"检查点、本地索引", "本地索引和导出元数据"} {
		if strings.Contains(spec, stale) {
			t.Errorf("provider-free spec still describes retired authority state %q", stale)
		}
	}
}

func TestDocumentationExplainsSelfDescribingWorkspaceRecovery(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, rel := range []string{"README.md", "README_EN.md", "README-TECHNICAL.md"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		text := string(raw)
		for _, want := range []string{"exchange/STATUS.json", "foundation_reference"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing self-describing workspace recovery reference %q", rel, want)
			}
		}
	}
	checklist, err := os.ReadFile(filepath.Join(root, "docs", "provider-free-release-checklist.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(checklist), "Self-describing workspace file E2E") {
		t.Error("release checklist does not record the self-describing workspace file E2E gate")
	}
}

func TestREADMEUsesSemanticDesignBeforeFirstReady(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"design_mode=required",
		"exchange/design/inbox",
		"story_locked",
		"foundation_ready",
		"Core 内部 Foundation settlement",
		"chapter:1 READY",
		"不读取、不等待 READY",
		"meta/core/design/",
		"CHATGPT_PROTOCOL.md",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("README.md missing semantic-design workflow %q", want)
		}
	}
	if strings.Contains(text, "Core 只有确认普通 UTF-8 JSON/Markdown 读写能力后才会生成正式任务") {
		t.Error("README.md still says capability immediately creates a production task")
	}
}

func TestReleaseChecklistMarksProtocolOneZeroAcceptanceHistorical(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "provider-free-release-checklist.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"protocol 1.0 / core schema 1",
		"protocol 1.1 / core schema 2",
		"不得继承旧 PASS",
		"Real ChatGPT App + Google Drive protocol 1.1 product chain |",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("release checklist missing semantic-design acceptance marker %q", want)
		}
	}
}
