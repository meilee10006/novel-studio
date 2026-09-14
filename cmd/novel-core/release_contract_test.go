package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNovelCoreHelpAndVersionNeedNoModelCredentials(t *testing.T) {
	for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "LITELLM_API_KEY"} {
		t.Setenv(key, "")
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--help"}, "novel-core"},
		{[]string{"--version"}, "novel-core"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(tc.args, &stdout, &stderr); code != 0 {
			t.Fatalf("run(%v)=%d stderr=%s", tc.args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), tc.want) {
			t.Fatalf("run(%v) stdout=%q want %q", tc.args, stdout.String(), tc.want)
		}
	}
}

func TestSupportedReleasePathsTargetNovelCore(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	checks := []struct {
		path      string
		required  []string
		forbidden []string
	}{
		{path: ".goreleaser.yml", required: []string{"main: ./cmd/novel-core", "binary: novel-core"}, forbidden: []string{"main: ./cmd/novel-studio", "binary: novel-studio"}},
		{path: "Dockerfile", required: []string{"./cmd/novel-core", "/usr/local/bin/novel-core", `ENTRYPOINT ["novel-core"]`}, forbidden: []string{"./cmd/novel-studio", "/usr/local/bin/novel-studio", `ENTRYPOINT ["novel-studio"]`}},
		{path: "docker-compose.yml", required: []string{"novel-core:"}, forbidden: []string{"\n  novel-studio:", "qdrant:", "/root/.novel-studio", "8765:8765"}},
		{path: "scripts/run-local.sh", required: []string{"go run ./cmd/novel-core"}, forbidden: []string{"go run ./cmd/novel-studio", "--pipeline", "service open"}},
		{path: "scripts/install.sh", required: []string{`BIN="novel-core"`, "NOVEL_CORE_VERSION", "NOVEL_CORE_INSTALL_DIR", "provider-free-novel-core/scripts/install.sh"}, forbidden: []string{`BIN="novel-studio"`, "novel-studio doctor", "NOVEL_STUDIO_VERSION", "NOVEL_STUDIO_INSTALL_DIR", "/main/scripts/install.sh"}},
		{path: ".dockerignore", required: []string{"\nnovel-core\n", "\nworkspace\n", "\ndist\n", "\ndeconstruction-library\n"}, forbidden: []string{"\nnovel-studio\n", "\n.novel-studio\n", "\nconfig\n", "\ntasks\n", "\nrefer\n", "\noutput*\n", "\ndata\n", "models/embedding", "__pycache__", "*.pyc", ".codegraph"}},
		{path: ".github/workflows/ci.yml", required: []string{"/tmp/novel-core", "./cmd/novel-core"}, forbidden: []string{"go build -trimpath -o /tmp/novel-studio ./cmd/novel-studio", "service start --host"}},
	}
	for _, check := range checks {
		t.Run(check.path, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, check.path))
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			for _, want := range check.required {
				if !strings.Contains(text, want) {
					t.Fatalf("%s missing required release contract %q", check.path, want)
				}
			}
			for _, bad := range check.forbidden {
				if strings.Contains(text, bad) {
					t.Fatalf("%s still contains retired production path %q", check.path, bad)
				}
			}
		})
	}
}
