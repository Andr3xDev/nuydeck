package fsx_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/adapters/fsx"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const sentinel = "SENTINEL-secret-value-9c1e"

func run(t *testing.T, s plan.Step) plan.StepResult {
	t.Helper()
	return fsx.Runner{Home: t.TempDir()}.Run(context.Background(), s, nil)
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func readString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWriteSecrets_CreatesExportLinesWithMode600EvenUnderRestrictiveUmask(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	// Umask 0o277 would leave 0o400 from OpenFile alone; only Fchmod reaches 0o600.
	old := syscall.Umask(0o277)
	t.Cleanup(func() { syscall.Umask(old) })

	res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{
		{Name: "API_KEY", Value: "plain"},
		{Name: "QUOTED", Value: "it's"},
	}})
	if res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got := mode(t, path); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
	want := "export API_KEY='plain'\nexport QUOTED='it'\\''s'\n"
	if got := readString(t, path); got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestWriteSecrets_ZeroEntriesCreatesEmptyFileWithMode600(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	if res := run(t, plan.WriteSecretsStep{Path: path}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got := readString(t, path); got != "" {
		t.Errorf("content = %q, want empty", got)
	}
	if got := mode(t, path); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestWriteSecrets_ExistingFileIsNeverOverwrittenOrReModed(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	if err := os.WriteFile(path, []byte("export KEEP='me'\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{{Name: "A", Value: sentinel}}})
	if res.Status == plan.StepOK {
		t.Errorf("result = %+v, want not OK", res)
	}
	if got := readString(t, path); got != "export KEEP='me'\n" {
		t.Errorf("existing file changed: %q", got)
	}
	if got := mode(t, path); got != 0o640 {
		t.Errorf("mode = %o, want 640 unchanged", got)
	}
	if res.Err != nil && strings.Contains(res.Err.Error(), sentinel) {
		t.Errorf("error leaks the value: %v", res.Err)
	}
}

func TestWriteSecrets_ValueWithNewlineIsRejectedWithoutLeakingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{{Name: "A", Value: plan.Secret(sentinel + "\nexport EVIL='1'")}}})
	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("result = %+v, want failure", res)
	}
	if strings.Contains(res.Err.Error(), sentinel) {
		t.Errorf("error leaks the value: %v", res.Err)
	}
	if body, err := os.ReadFile(path); err == nil && strings.Contains(string(body), "EVIL") {
		t.Errorf("injected line reached the file: %q", body)
	}
}

func TestWriteSecrets_NulInValueIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".secrets")
	res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{{Name: "A", Value: plan.Secret("a\x00b")}}})
	if res.Status != plan.StepFailed {
		t.Errorf("result = %+v, want failure", res)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("secrets file was created")
	}
}

func TestWriteSecrets_UnsafeOrDuplicateNamesAreRejectedAndNoFileIsCreated(t *testing.T) {
	for _, names := range [][]string{{"X; touch pwned"}, {"A=B"}, {"1A"}, {""}, {"A", "A"}} {
		path := filepath.Join(t.TempDir(), ".secrets")
		var entries []plan.SecretEntry
		for _, n := range names {
			entries = append(entries, plan.SecretEntry{Name: n, Value: "v"})
		}
		if res := run(t, plan.WriteSecretsStep{Path: path, Entries: entries}); res.Status != plan.StepFailed {
			t.Errorf("names %q: result = %+v, want failure", names, res)
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("names %q: secrets file was created", names)
		}
	}
}

func TestWriteSecrets_CreatesMissingParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".secrets")
	if res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{{Name: "A", Value: "v"}}}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got := mode(t, path); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestSymlink_CreatesMissingParentDir(t *testing.T) {
	link := filepath.Join(t.TempDir(), "hypr", "gpu_env.lua")
	if res := run(t, plan.SymlinkStep{Path: link, Target: "gpu_env_amd.lua"}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got, err := os.Readlink(link); err != nil || got != "gpu_env_amd.lua" {
		t.Errorf("link = %q, %v", got, err)
	}
}

func TestWriteSecrets_WriteFailureNamesPathAndCauseButNoValue(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	path := filepath.Join(dir, ".secrets")

	res := run(t, plan.WriteSecretsStep{Path: path, Entries: []plan.SecretEntry{{Name: "A", Value: sentinel}}})
	if res.Status != plan.StepFailed || res.Err == nil {
		t.Fatalf("result = %+v, want failure", res)
	}
	if !strings.HasPrefix(res.Err.Error(), "create "+path+": ") {
		t.Errorf("error = %q, want prefix %q", res.Err, "create "+path+": ")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if out := fmt.Sprintf(format, res); strings.Contains(out, sentinel) {
			t.Errorf("result printed with %s leaks the value: %s", format, out)
		}
	}
}

func TestSymlink_CreatesRelativeLink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "gpu_env.lua")
	if res := run(t, plan.SymlinkStep{Path: link, Target: "gpu_env_nvidia.lua"}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got, err := os.Readlink(link); err != nil || got != "gpu_env_nvidia.lua" {
		t.Errorf("Readlink = %q, %v; want gpu_env_nvidia.lua", got, err)
	}
}

func TestSymlink_ReplacesExistingLinkWithoutLeavingTempFiles(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "gpu_env.lua")
	if err := os.Symlink("gpu_env_intel.lua", link); err != nil {
		t.Fatal(err)
	}
	if res := run(t, plan.SymlinkStep{Path: link, Target: "gpu_env_amd.lua"}); res.Status != plan.StepOK {
		t.Fatalf("result = %+v", res)
	}
	if got, _ := os.Readlink(link); got != "gpu_env_amd.lua" {
		t.Errorf("Readlink = %q, want gpu_env_amd.lua", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("leftover entries: %v", entries)
	}
}

func TestSymlink_ExistingRegularFileIsLeftUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpu_env.lua")
	if err := os.WriteFile(path, []byte("hand written"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, plan.SymlinkStep{Path: path, Target: "gpu_env_amd.lua"})
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("regular file was replaced: %v, %v", info, err)
	}
	if got := readString(t, path); got != "hand written" {
		t.Errorf("content = %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("leftover entries: %v", entries)
	}
}

func TestWriteFile_CreatesParentsAndReplacesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nuydeck", "theme", "name")
	step := plan.WriteFileStep{Path: path, Content: []byte("nuytek\n"), Mode: 0o644}
	if res := run(t, step); res.Status != plan.StepOK {
		t.Fatalf("create: %+v", res)
	}
	step.Content = []byte("other\n")
	if res := run(t, step); res.Status != plan.StepOK {
		t.Fatalf("replace: %+v", res)
	}
	if got := readString(t, path); got != "other\n" {
		t.Errorf("content = %q", got)
	}
	if got := mode(t, path); got != 0o644 {
		t.Errorf("mode = %o, want 644", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("leftover temp files: %v", entries)
	}
}

func TestWriteFile_PrivilegedStepIsRefusedAndNothingIsWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pacman.conf")
	res := run(t, plan.WriteFileStep{Path: path, Content: []byte("x"), Mode: 0o644, Privileged: true})
	if res.Status != plan.StepFailed {
		t.Errorf("result = %+v, want failure", res)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("file was written: %v", err)
	}
}
