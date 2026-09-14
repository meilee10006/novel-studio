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
