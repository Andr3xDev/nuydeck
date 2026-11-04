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
