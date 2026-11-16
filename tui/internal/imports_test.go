package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	modulePath     = "github.com/Andr3xDev/nuydeck/tui"
	internalPrefix = modulePath + "/internal/"
	cmdPrefix      = modulePath + "/cmd"
	uiPackage      = "ui"
)

var adapterPackages = []string{"execx", "rsync", "osexec", "osfs", "gitx", "sysx", "dryrun"}

// Adapter-to-adapter edges allowed by design D1; an adapter importing any other adapter fails.
var adapterEdges = map[string][]string{
	"rsync":  {"execx"},
	"gitx":   {"execx"},
	"sysx":   {"execx"},
	"osfs":   {"rsync", "execx"},
	"osexec": {"osfs", "rsync", "execx", "sysx"},
}

// Exact in-module imports allowed per domain package (design D1 table). Domain packages without a row
// (change 3 additions) are bound only by the layer rules.
var domainEdges = map[string][]string{
	"fsutil":   {},
	"editor":   {},
	"manifest": {"fsutil"},
	"state":    {"fsutil"},
	"plan":     {"manifest", "state"},
	"cfgsync":  {"plan", "manifest", "state", "fsutil"},
	"backup":   {"cfgsync", "plan", "manifest", "fsutil"},
	"drift":    {"cfgsync", "backup", "plan", "manifest", "state"},
	"packages": {"plan", "manifest", "state"},
	"install":  {"plan", "cfgsync", "packages", "manifest", "state"},
}

// Package-only leftovers of the old layout; they must import nothing and go once the owner deletes them.
var legacyPackages = []string{"app", "domain", "adapters"}

func TestInternalPackages_AreAllClassified(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		_, isDomain := domainEdges[name]
		if !isDomain && name != uiPackage && !slices.Contains(adapterPackages, name) && !slices.Contains(legacyPackages, name) {
			t.Errorf("internal/%s is in no layer list: classify it in adapterPackages, domainEdges or uiPackage", name)
		}
	}
}

func TestViolation(t *testing.T) {
	cases := []struct {
		name       string
		pkg        string
		importPath string
		wantReject bool
	}{
		{"ui to adapter", "ui", internalPrefix + "osexec", true},
		{"domain to adapter", "cfgsync", internalPrefix + "rsync", true},
		{"unlisted adapter edge", "rsync", internalPrefix + "gitx", true},
		{"execx imports a module package", "execx", internalPrefix + "plan", true},
		{"fsutil imports a module package", "fsutil", internalPrefix + "manifest", true},
		{"editor imports a module package", "editor", internalPrefix + "state", true},
		{"domain outside its row", "manifest", internalPrefix + "plan", true},
		{"legacy package imports a module package", "app", internalPrefix + "plan", true},
		{"allowed adapter edge", "rsync", internalPrefix + "execx", false},
		{"allowed domain edge", "cfgsync", internalPrefix + "plan", false},
		{"ui to domain", "ui", internalPrefix + "install", false},
		{"standard library", "execx", "os/exec", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := violation(tc.pkg, tc.importPath) != ""; got != tc.wantReject {
				t.Errorf("violation(%q, %q) rejected = %v, want %v", tc.pkg, tc.importPath, got, tc.wantReject)
			}
		})
	}
}

func TestInternalPackages_RespectLayerImportRules(t *testing.T) {
	checked := 0
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		checked++
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		pkg := strings.SplitN(filepath.ToSlash(path), "/", 2)[0]
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if reason := violation(pkg, importPath); reason != "" {
				t.Errorf("%s imports %s: %s", path, importPath, reason)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no production files found to check")
	}
}

func violation(pkg, importPath string) string {
	if importPath == cmdPrefix || strings.HasPrefix(importPath, cmdPrefix+"/") {
		return "internal packages never import cmd"
	}
	if !strings.HasPrefix(importPath, internalPrefix) {
		if strings.HasPrefix(importPath, modulePath) {
			return "unexpected in-module import outside internal"
		}
		return ""
	}
	target, _, _ := strings.Cut(strings.TrimPrefix(importPath, internalPrefix), "/")
	pkgIsAdapter := slices.Contains(adapterPackages, pkg)
	targetIsAdapter := slices.Contains(adapterPackages, target)

	switch {
	case target == uiPackage && pkg != uiPackage:
		return "only ui may import ui"
	case pkg == uiPackage && targetIsAdapter:
		return "ui never imports an adapter"
	case pkg == "dryrun" && target != "plan":
		return "dryrun imports plan only"
	case pkgIsAdapter && targetIsAdapter && !slices.Contains(adapterEdges[pkg], target):
		return "adapter-to-adapter edge not in the allow-list"
	case !pkgIsAdapter && pkg != uiPackage && targetIsAdapter:
		return "domain never imports an adapter"
	}
	if allowed, ok := domainEdges[pkg]; ok && !slices.Contains(allowed, target) {
		return "not in the allowed imports of " + pkg
	}
	if pkg == "execx" || slices.Contains(legacyPackages, pkg) {
		return pkg + " imports nothing inside the module"
	}
	return ""
}
