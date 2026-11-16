package manifest_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func lines(l ...string) string { return strings.Join(l, "\n") + "\n" }

func ptr(s string) *string { return &s }

func TestLoadPackages_ValidCatalogWithOptionalFieldsAbsent(t *testing.T) {
	path := writeTemp(t, "packages.toml", lines(
		`version = 1`,
		``,
		`[[package]]`,
		`name = "hyprland"`,
		`source = "pacman"`,
		`profiles = ["minimal", "full"]`,
		`group = "desktop"`,
		`service = "ly.service"`,
		``,
		`[[package]]`,
		`name = "htop"`,
		`source = "aur"`,
	))
	got, err := manifest.LoadPackages(path)
	if err != nil {
		t.Fatalf("LoadPackages() error = %v", err)
	}
	want := []manifest.Package{
		{Name: "hyprland", Source: manifest.SourcePacman, Profiles: []string{"minimal", "full"}, Group: "desktop", Service: "ly.service"},
		{Name: "htop", Source: manifest.SourceAUR},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d packages, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Source != want[i].Source || got[i].Group != want[i].Group ||
			got[i].Service != want[i].Service || len(got[i].Profiles) != len(want[i].Profiles) {
			t.Errorf("package %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLoadPackages_ErrorsNameFileAndLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLine int
		wantText []string
	}{
		{"syntax error", lines(`version = 1`, ``, `[[package]`, `name = "a"`), 3, nil},
		{"unknown key", lines(`version = 1`, ``, `[[package]]`, `name = "a"`, `source = "pacman"`, `colour = "red"`), 6, []string{"colour"}},
		{"unknown source", lines(`version = 1`, ``, `[[package]]`, `name = "a"`, `source = "pacman"`, ``, `[[package]]`, `name = "b"`, `source = "flatpak"`), 7, []string{"flatpak"}},
		{"missing source", lines(`version = 1`, ``, `[[package]]`, `name = "a"`, `source = "pacman"`, ``, `[[package]]`, `name = "b"`), 7, []string{"b"}},
		{"unknown profile", lines(`version = 1`, ``, `[[package]]`, `name = "a"`, `source = "pacman"`, `profiles = ["gold"]`), 3, []string{"gold"}},
		{"duplicate names both lines", lines(`version = 1`, ``, `[[package]]`, `name = "a"`, `source = "pacman"`, ``, `[[package]]`, `name = "a"`, `source = "aur"`), 7, []string{"a", "line 3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.LoadPackages(writeTemp(t, "packages.toml", tt.content))
			if err == nil {
				t.Fatal("LoadPackages() error = nil, want error")
			}
			if prefix := "packages.toml:" + strconv.Itoa(tt.wantLine) + ": "; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error = %q, want prefix %q", err, prefix)
			}
			for _, text := range tt.wantText {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("error = %q, want it to contain %q", err, text)
				}
			}
		})
	}
}

func TestLoadPackages_MissingFileIsAnError(t *testing.T) {
	_, err := manifest.LoadPackages(filepath.Join(t.TempDir(), "packages.toml"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadHardware_ValidDriversWithOptionalServices(t *testing.T) {
	path := writeTemp(t, "hardware.toml", lines(
		`version = 1`,
		``,
		`[[driver]]`,
		`id = "nvidia"`,
		`vendor_id = "10de"`,
		`packages = ["nvidia-open-dkms", "nvidia-utils"]`,
		`services = ["nvidia-suspend.service"]`,
		``,
		`[[driver]]`,
		`id = "intel"`,
		`vendor_id = "8086"`,
		`packages = ["vulkan-intel"]`,
	))
	got, err := manifest.LoadHardware(path)
	if err != nil {
		t.Fatalf("LoadHardware() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d drivers, want 2: %+v", len(got), got)
	}
	nvidia, intel := got[0], got[1]
	if nvidia.ID != "nvidia" || nvidia.VendorID != "10de" ||
		!reflect.DeepEqual(nvidia.Packages, []string{"nvidia-open-dkms", "nvidia-utils"}) ||
		!reflect.DeepEqual(nvidia.Services, []string{"nvidia-suspend.service"}) {
		t.Errorf("nvidia = %+v", nvidia)
	}
	if intel.ID != "intel" || intel.VendorID != "8086" || len(intel.Services) != 0 {
		t.Errorf("intel = %+v, want no services", intel)
	}
}

func TestLoadHardware_ErrorsNameFileAndLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantLine int
		wantText []string
	}{
		{"unknown key", lines(`version = 1`, ``, `[[driver]]`, `id = "a"`, `vendor_id = "10de"`, `colour = "red"`), 6, []string{"colour"}},
		{"vendor id not four hex digits", lines(`version = 1`, ``, `[[driver]]`, `id = "a"`, `vendor_id = "10dg"`), 3, []string{"10dg"}},
		{"duplicate driver id", lines(`version = 1`, ``, `[[driver]]`, `id = "a"`, `vendor_id = "10de"`, ``, `[[driver]]`, `id = "a"`, `vendor_id = "8086"`), 7, []string{"line 3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.LoadHardware(writeTemp(t, "hardware.toml", tt.content))
			if err == nil {
				t.Fatal("LoadHardware() error = nil, want error")
			}
			if prefix := "hardware.toml:" + strconv.Itoa(tt.wantLine) + ": "; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error = %q, want prefix %q", err, prefix)
			}
			for _, text := range tt.wantText {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("error = %q, want it to contain %q", err, text)
				}
			}
		})
	}
}

func TestLoadSecrets_NamesOnlyAndAbsentOrEmptyMeansNoNames(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		want    []string
	}{
		{"names in file order", ptr(lines(`version = 1`, ``, `[[secret]]`, `name = "API_KEY"`, ``, `[[secret]]`, `name = "TOKEN"`)), []string{"API_KEY", "TOKEN"}},
		{"no secret table", ptr(lines(`version = 1`)), nil},
		{"empty file", ptr(""), nil},
		{"absent file", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "secrets.toml")
			if tt.content != nil {
				path = writeTemp(t, "secrets.toml", *tt.content)
			}
			got, err := manifest.LoadSecrets(path)
			if err != nil {
				t.Fatalf("LoadSecrets() error = %v", err)
			}
			if len(got) != len(tt.want) || (len(got) > 0 && !reflect.DeepEqual(got, tt.want)) {
				t.Errorf("names = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadSecrets_ValueKeyIsRejectedWithFileLineAndNoValue(t *testing.T) {
	const sentinel = "SENTINEL-secret-value-7f3a"
	path := writeTemp(t, "secrets.toml", lines(`version = 1`, ``, `[[secret]]`, `name = "API_KEY"`, `value = "`+sentinel+`"`))
	_, err := manifest.LoadSecrets(path)
	if err == nil {
		t.Fatal("LoadSecrets() error = nil, want error")
	}
	if !strings.HasPrefix(err.Error(), "secrets.toml:5: ") {
		t.Errorf("error = %q, want prefix %q", err, "secrets.toml:5: ")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Errorf("error leaks the value: %q", err)
	}
}

func TestLoadSecrets_UnsafeOrDuplicateNamesAreRejectedWithFileAndLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		prefix  string
	}{
		{"shell injection", lines(`version = 1`, ``, `[[secret]]`, `name = "OK"`, ``, `[[secret]]`, `name = "X; touch pwned"`), "secrets.toml:6: "},
		{"assignment", lines(`version = 1`, ``, `[[secret]]`, `name = "A=B"`), "secrets.toml:3: "},
		{"duplicate", lines(`version = 1`, ``, `[[secret]]`, `name = "A"`, ``, `[[secret]]`, `name = "A"`), "secrets.toml:6: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifest.LoadSecrets(writeTemp(t, "secrets.toml", tt.content))
			if err == nil || !strings.HasPrefix(err.Error(), tt.prefix) {
				t.Errorf("error = %v, want prefix %q", err, tt.prefix)
			}
		})
	}
}
