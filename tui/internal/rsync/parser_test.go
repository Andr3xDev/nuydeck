package rsync

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
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
	got, _, err := parseItemize(fixture(t, "plan_mixed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{"gone/inner/f.txt", plan.Delete, plan.KindFile},
		{"gone/inner", plan.Delete, plan.KindDir},
		{"gone", plan.Delete, plan.KindDir},
		{"only.txt", plan.Delete, plan.KindFile},
		{"lnk", plan.Create, plan.KindLink},
		{"lnk2", plan.Update, plan.KindLink},
		{"mode.sh", plan.Update, plan.KindFile},
		{"new.txt", plan.Create, plan.KindFile},
		{"tab\tname.txt", plan.Create, plan.KindFile},
		{"upd.txt", plan.Update, plan.KindFile},
		{"we ird#.txt", plan.Create, plan.KindFile},
		{"sub/n.txt", plan.Create, plan.KindFile},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseItemize_DropsDirNoiseUnescapesNames(t *testing.T) {
	got, _, err := parseItemize(fixture(t, "plan_dirs_escapes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{"old", plan.Delete, plan.KindDir},
		{`back\slash`, plan.Create, plan.KindFile},
		{"nl\nname", plan.Create, plan.KindFile},
		{"newdir/a", plan.Create, plan.KindFile},
		{"olddir/b", plan.Create, plan.KindFile},
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
			if _, _, err := parseItemize(in); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestParseItemize_EmptyOutputHasNoItems(t *testing.T) {
	got, _, err := parseItemize("")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestPlanExit(t *testing.T) {
	if w, err := planExit(0); w != "" || err != nil {
		t.Fatalf("0: %q, %v", w, err)
	}
	if w, err := planExit(24); w == "" || err != nil {
		t.Fatalf("24 must warn without error: %q, %v", w, err)
	}
	for _, code := range []int{23, 1, 12} {
		if _, err := planExit(code); err == nil {
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
