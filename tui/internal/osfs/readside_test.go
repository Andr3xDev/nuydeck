package osfs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/osfs"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

func TestReadSide_TextFileIsReadWithModeAndSize(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.conf")
	writeFile(t, p, "hello\n")
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}

	s, err := osfs.ReadSide(p)

	if err != nil {
		t.Fatalf("ReadSide: %v", err)
	}
	if !s.Exists || s.Kind != plan.KindFile || s.Size != 6 || string(s.Content) != "hello\n" || s.Mode.Perm() != 0o640 || s.Binary || s.TooLarge {
		t.Errorf("side = %+v", s)
	}
}

func TestReadSide_MissingPathIsNotExistsAndNoError(t *testing.T) {
	s, err := osfs.ReadSide(filepath.Join(t.TempDir(), "nope"))

	if err != nil || s.Exists {
		t.Errorf("ReadSide(missing) = %+v, %v; want Exists=false, nil", s, err)
	}
}

func TestReadSide_BinaryDetection(t *testing.T) {
	cases := map[string]string{
		"NUL in the probe window":  "ab\x00cd",
		"invalid UTF-8":            "caf\xe9",
		"NUL right before the cap": strings.Repeat("a", 8<<10-1) + "\x00",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "f")
			writeFile(t, p, content)

			s, err := osfs.ReadSide(p)

			if err != nil || !s.Binary {
				t.Errorf("ReadSide = %+v, %v; want Binary", s, err)
			}
		})
	}
}

func TestReadSide_ValidMultibyteUTF8IsText(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	writeFile(t, p, "café ñ 日本\n")

	s, err := osfs.ReadSide(p)

	if err != nil || s.Binary {
		t.Errorf("ReadSide = %+v, %v; want text", s, err)
	}
}

func TestReadSide_AboveCapIsTooLargeAndContentNotRead(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big")
	writeFile(t, p, strings.Repeat("a", cfgsync.MaxEditBytes+1))

	s, err := osfs.ReadSide(p)

	if err != nil {
		t.Fatalf("ReadSide: %v", err)
	}
	if !s.TooLarge || s.Content != nil || s.Size != cfgsync.MaxEditBytes+1 {
		t.Errorf("side = Size %d TooLarge %v Content %d bytes", s.Size, s.TooLarge, len(s.Content))
	}
}

func TestReadSide_ExactlyAtCapIsReadable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "edge")
	writeFile(t, p, strings.Repeat("a", cfgsync.MaxEditBytes))

	s, err := osfs.ReadSide(p)

	if err != nil || s.TooLarge || len(s.Content) != cfgsync.MaxEditBytes {
		t.Errorf("ReadSide at cap: TooLarge %v, %d bytes, err %v", s.TooLarge, len(s.Content), err)
	}
}

func TestReadSide_SymlinkIsReportedAsLinkNotFollowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "target"), "secret\n")
	link := filepath.Join(dir, "l")
	if err := os.Symlink("target", link); err != nil {
		t.Fatal(err)
	}

	s, err := osfs.ReadSide(link)

	if err != nil {
		t.Fatalf("ReadSide: %v", err)
	}
	if s.Kind != plan.KindLink || s.LinkTarget != "target" || s.Content != nil {
		t.Errorf("side = %+v; want a link to target with no content", s)
	}
	if s.LinkBroken {
		t.Error("a link to an existing file must not be broken")
	}
}

func TestReadSide_DanglingLinkIsBroken(t *testing.T) {
	link := filepath.Join(t.TempDir(), "l")
	if err := os.Symlink("missing", link); err != nil {
		t.Fatal(err)
	}

	s, err := osfs.ReadSide(link)

	if err != nil || !s.LinkBroken || s.LinkTarget != "missing" {
		t.Errorf("ReadSide = %+v, %v; want a broken link to missing", s, err)
	}
}

func TestReadSide_DirListsSortedChildrenWithKinds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "b.conf"), "xy")
	writeFile(t, filepath.Join(dir, "sub", "c"), "")
	if err := os.Symlink("b.conf", filepath.Join(dir, "a.lnk")); err != nil {
		t.Fatal(err)
	}

	s, err := osfs.ReadSide(dir)

	if err != nil {
		t.Fatalf("ReadSide: %v", err)
	}
	got := make([]string, len(s.Children))
	for i, c := range s.Children {
		got[i] = c.Name
	}
	if strings.Join(got, ",") != "a.lnk,b.conf,sub" || s.Children[0].Kind != plan.KindLink || s.Children[1].Size != 2 || s.Children[2].Kind != plan.KindDir {
		t.Errorf("children = %+v", s.Children)
	}
}
