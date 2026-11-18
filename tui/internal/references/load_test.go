package references_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/references"
)

const repoReferences = "../../../manifest/references.toml"

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "references.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func lines(l ...string) string { return strings.Join(l, "\n") + "\n" }

func entry(fields ...string) []string {
	return append([]string{`version = 1`, ``, `[[reference]]`}, fields...)
}

func TestLoad_ValidEntryWithOptionalDetection(t *testing.T) {
	path := writeTemp(t, lines(append(entry(
		`name = "arch"`,
		`description = "Arch"`,
		`url = "https://archlinux.org"`,
		`detect_binary = "pacman"`,
		`detect_path = "~/dev"`,
	), ``, `[[reference]]`, `name = "plain"`, `description = "Plain"`, `url = "https://example.com/a?b=1"`)...))
	got, err := references.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []references.Reference{
		{Name: "arch", Description: "Arch", URL: "https://archlinux.org", DetectBinary: "pacman", DetectPath: "~/dev"},
		{Name: "plain", Description: "Plain", URL: "https://example.com/a?b=1"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_ErrorsNameFileLineAndOffender(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantPart []string
	}{
		{"http link", lines(entry(`name = "a"`, `description = "d"`, `url = "http://x.org"`)...), []string{"references.toml:3", `"a"`}},
		{"missing host", lines(entry(`name = "a"`, `description = "d"`, `url = "https://"`)...), []string{"references.toml:3", `"a"`}},
		{"whitespace in url", lines(entry(`name = "a"`, `description = "d"`, `url = "https://x.org/a b"`)...), []string{"references.toml:3", `"a"`}},
		{"empty url", lines(entry(`name = "a"`, `description = "d"`)...), []string{"references.toml:3", `"a"`}},
		{"missing description", lines(entry(`name = "a"`, `url = "https://x.org"`)...), []string{"references.toml:3", `"a"`}},
		{"empty name", lines(entry(`name = ""`, `description = "d"`, `url = "https://x.org"`)...), []string{"references.toml:3"}},
		{"unknown key", lines(entry(`name = "a"`, `description = "d"`, `url = "https://x.org"`, `colour = "red"`)...), []string{"references.toml:7", "colour"}},
		{"duplicate name", lines(append(entry(`name = "a"`, `description = "d"`, `url = "https://x.org"`),
			``, `[[reference]]`, `name = "a"`, `description = "d"`, `url = "https://y.org"`)...), []string{"references.toml:8", `"a"`, "line 3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := references.Load(writeTemp(t, tt.content))
			if err == nil {
				t.Fatal("Load() error = nil")
			}
			for _, part := range tt.wantPart {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q missing %q", err, part)
				}
			}
		})
	}
}

func TestRepoReferences_ShipsOneHTTPSPlaceholder(t *testing.T) {
	got, err := references.Load(repoReferences)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://archlinux.org" {
		t.Errorf("Load() = %+v, want exactly the arch-linux placeholder", got)
	}
}
