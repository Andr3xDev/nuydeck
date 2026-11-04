package rsync

import (
	"context"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func TestParseItemize_IgnoresCreatedDirectoryNotice(t *testing.T) {
	got, _, err := parseItemize("created directory /home/u/.config/cfg\n>f+++++++++ a.txt\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Item{"a.txt", domain.Create, domain.KindFile}) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseItemize_IgnoresCannotDeleteNonEmptyDirectoryNotice(t *testing.T) {
	got, _, err := parseItemize("cannot delete non-empty directory: sub/dir\n>f+++++++++ a.txt\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (Item{"a.txt", domain.Create, domain.KindFile}) {
		t.Fatalf("got %+v", got)
	}
}

func TestParseItemize_UnknownLineStillErrors(t *testing.T) {
	if _, _, err := parseItemize("something unexpected here\n"); err == nil {
		t.Fatal("unknown line must still error")
	}
}

func TestIntegrationApplyCreatesMissingParentsForDirAndFileEntries(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	write(t, root+"/repo/dir/a.txt", "a", 0o644)
	write(t, root+"/repo/.zshrc", "z", 0o644)

	apply(t, dirReq(root+"/repo/dir", root+"/x/y/dir"), "a.txt")
	apply(t, Request{Src: root + "/repo/.zshrc", Dst: root + "/p/q/.zshrc"})

	if read(t, root+"/x/y/dir/a.txt") != "a" || read(t, root+"/p/q/.zshrc") != "z" {
		t.Fatal("missing destination parents were not created")
	}
}

func TestIntegrationPlanMissingDestinationDir(t *testing.T) {
	requireRsync(t)
	root := t.TempDir()
	write(t, root+"/src/a.txt", "a", 0o644)

	res, err := Plan(context.Background(), dirReq(root+"/src", root+"/missing/dst"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Path != "a.txt" {
		t.Fatalf("got %+v", res.Items)
	}
}
