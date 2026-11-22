package plan

import (
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func TestDescribe(t *testing.T) {
	exact := []struct {
		step      Step
		wantLabel string
		wantKind  string
	}{
		{CommandStep{Label: "install pacman", Argv: []string{"pacman"}}, "install pacman", "command"},
		{WriteFileStep{Path: "/etc/x.conf"}, "/etc/x.conf", "file"},
		{SymlinkStep{Path: "/home/u/link"}, "/home/u/link", "link"},
		{secretsStep("hunter2"), "/home/u/.secrets", "secrets"},
		{CopyStep{Entry: manifest.ResolvedEntry{Entry: manifest.Entry{Root: manifest.RootConfig, Path: "hypr"}}}, "config:hypr", "copy"},
		{BackupStep{}, "back up overwritten files", "backup"},
	}
	for _, tt := range exact {
		label, kind := Describe(tt.step)
		if label != tt.wantLabel || kind != tt.wantKind {
			t.Errorf("Describe(%T) = (%q, %q), want (%q, %q)", tt.step, label, kind, tt.wantLabel, tt.wantKind)
		}
	}
}

func TestDescribe_NamesEveryStepTypeWithoutLeakingSecrets(t *testing.T) {
	steps := []Step{
		CopyStep{}, DeleteStep{Path: "/p"}, BackupStep{}, DeleteBackupStep{Dir: "/b"}, SaveStateStep{},
		AppendLogStep{}, CommandStep{Label: "c"}, WriteFileStep{Path: "/f"}, SymlinkStep{Path: "/l"},
		secretsStep("hunter2"), AppendPackageStep{Path: "/packages.toml"},
	}
	for _, s := range steps {
		label, kind := Describe(s)
		if label == "" || kind == "" {
			t.Errorf("Describe(%T) = (%q, %q), want both non-empty", s, label, kind)
		}
		if strings.Contains(label, "hunter2") {
			t.Errorf("Describe(%T) label leaks a secret: %q", s, label)
		}
	}
}
