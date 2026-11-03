package rsync

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseItemize_MixedOutput(t *testing.T) {
	got, err := ParseItemize(fixture(t, "plan_mixed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{"gone/inner/f.txt", domain.Delete, domain.KindFile},
		{"gone/inner", domain.Delete, domain.KindDir},
		{"gone", domain.Delete, domain.KindDir},
		{"only.txt", domain.Delete, domain.KindFile},
		{"lnk", domain.Create, domain.KindLink},
		{"lnk2", domain.Update, domain.KindLink},
		{"mode.sh", domain.Update, domain.KindFile},
		{"new.txt", domain.Create, domain.KindFile},
		{"tab\tname.txt", domain.Create, domain.KindFile},
		{"upd.txt", domain.Update, domain.KindFile},
		{"we ird#.txt", domain.Create, domain.KindFile},
		{"sub/n.txt", domain.Create, domain.KindFile},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseItemize_DropsDirNoiseUnescapesNames(t *testing.T) {
	got, err := ParseItemize(fixture(t, "plan_dirs_escapes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{"old", domain.Delete, domain.KindDir},
		{`back\slash`, domain.Create, domain.KindFile},
		{"nl\nname", domain.Create, domain.KindFile},
		{"newdir/a", domain.Create, domain.KindFile},
		{"olddir/b", domain.Create, domain.KindFile},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseItemize_Errors(t *testing.T) {
	cases := map[string]string{
		"special file": fixture(t, "special.txt"),
		"garbage":      fixture(t, "garbage.txt"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseItemize(in); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestParseItemize_EmptyOutputHasNoItems(t *testing.T) {
	got, err := ParseItemize("")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPlanExit(t *testing.T) {
	if w, err := PlanExit(0); w != "" || err != nil {
		t.Fatalf("0: %q, %v", w, err)
	}
	if w, err := PlanExit(24); w == "" || err != nil {
		t.Fatalf("24 must warn without error: %q, %v", w, err)
	}
	for _, code := range []int{23, 1, 12} {
		if _, err := PlanExit(code); err == nil {
			t.Fatalf("%d must be an error", code)
		}
	}
}

func TestParseItemize_SkippedNonRegularIsWarningNotItemNorError(t *testing.T) {
	items, skipped, err := parseItemize(fixture(t, "skipping.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Path != "a.txt" || len(skipped) != 1 || skipped[0] != `"fifo"` {
		t.Fatalf("items %+v skipped %v", items, skipped)
	}
}
