package cfgsync

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/state"
)

const editedSecret = "API_TOKEN=hunter2\n"

// resolverPlan: a.conf differs, b.conf is new, old.conf exists only on the destination.
func resolverPlan(t *testing.T, dir plan.Direction) SyncPlan {
	t.Helper()
	return planOf(t, dir, []manifest.ResolvedEntry{hypr}, map[string][]plan.Change{
		"hypr": {
			change("a.conf", plan.Update, plan.KindFile),
			change("b.conf", plan.Create, plan.KindFile),
			change("old.conf", plan.Delete, plan.KindFile),
		},
	})
}

func itemNamed(t *testing.T, p SyncPlan, name string) PlanItem {
	t.Helper()
	for _, it := range p.Items {
		if it.Change.Path == name {
			return it
		}
	}
	t.Fatalf("no item %q in plan", name)
	return PlanItem{}
}

func flatten(batches []Batch) []plan.Step {
	var out []plan.Step
	for _, b := range batches {
		out = append(out, b.Step)
	}
	return out
}

func editOf(it PlanItem) Resolutions {
	return Resolutions{it.Key(): Edited{Content: []byte(editedSecret), Mode: 0o600}}
}

func TestSteps_EditedItemEmitsWriteFileStepAndNoCopyOrDelete(t *testing.T) {
	p := resolverPlan(t, plan.Push)
	a := itemNamed(t, p, "a.conf")
	sel := Selection{a.Key(): true}

	steps := flatten(Steps(p, sel, editOf(a), ApplyOptions{}))

	writes := stepsOfType[plan.WriteFileStep](steps)
	want := plan.WriteFileStep{Path: a.DstPath(), Content: []byte(editedSecret), Mode: 0o600, Root: a.Dst}
	if len(writes) != 1 || !reflect.DeepEqual(writes[0], want) {
		t.Errorf("writes = %+v, want [%+v]", writes, want)
	}
	if copies := stepsOfType[plan.CopyStep](steps); len(copies) != 0 {
		t.Errorf("edited item must not be copied: %+v", copies)
	}
	if dels := stepsOfType[plan.DeleteStep](steps); len(dels) != 0 {
		t.Errorf("delete steps = %+v", dels)
	}
}

func TestSteps_TakeCopiesKeepEmitsNothingEditWritesInOneRun(t *testing.T) {
	p := resolverPlan(t, plan.Push)
	a, b := itemNamed(t, p, "a.conf"), itemNamed(t, p, "b.conf")
	sel := Selection{a.Key(): true, b.Key(): true}

	steps := flatten(Steps(p, sel, editOf(a), ApplyOptions{}))

	copies := stepsOfType[plan.CopyStep](steps)
	if len(copies) != 1 || !reflect.DeepEqual(copies[0].Paths, []string{"b.conf"}) {
		t.Errorf("copies = %+v, want only b.conf", copies)
	}
	if writes := stepsOfType[plan.WriteFileStep](steps); len(writes) != 1 || writes[0].Path != a.DstPath() {
		t.Errorf("writes = %+v", writes)
	}
}

func TestSteps_KeepEmitsNoCopyAndNoWrite(t *testing.T) {
	p := resolverPlan(t, plan.Push)

	steps := flatten(Steps(p, Selection{}, Resolutions{}, ApplyOptions{}))

	if c, w := stepsOfType[plan.CopyStep](steps), stepsOfType[plan.WriteFileStep](steps); len(c)+len(w) != 0 {
		t.Errorf("copies %+v writes %+v, want none", c, w)
	}
}

// Deselecting drops the edit (D22): a stale Resolutions entry must not write.
func TestSteps_DeselectedItemWithResolutionIsNotWritten(t *testing.T) {
	p := resolverPlan(t, plan.Push)
	a := itemNamed(t, p, "a.conf")

	steps := flatten(Steps(p, Selection{}, editOf(a), ApplyOptions{}))

	if writes := stepsOfType[plan.WriteFileStep](steps); len(writes) != 0 {
		t.Errorf("writes = %+v, want none", writes)
	}
}

func TestSteps_BackupCoversEditedFileAndPrecedesTheWrite(t *testing.T) {
	p := resolverPlan(t, plan.Push)
	a := itemNamed(t, p, "a.conf")

	steps := flatten(Steps(p, Selection{a.Key(): true}, editOf(a), ApplyOptions{Backup: true}))

	backupAt, writeAt := -1, -1
	for i, s := range steps {
		switch s.(type) {
		case plan.BackupStep:
			backupAt = i
		case plan.WriteFileStep:
			writeAt = i
		}
	}
	if backupAt < 0 || writeAt < 0 || backupAt > writeAt {
		t.Fatalf("backup at %d, write at %d; want backup first", backupAt, writeAt)
	}
	backup := steps[backupAt].(plan.BackupStep)
	if len(backup.Files) != 1 || backup.Files[0].Path != "hypr/a.conf" {
		t.Errorf("backup files = %+v, want hypr/a.conf", backup.Files)
	}
}

func TestSteps_NormalModeNeverDeletesEvenWhenExtraIsForcedIntoSelection(t *testing.T) {
	for _, dir := range []plan.Direction{plan.Push, plan.Pull} {
		p := resolverPlan(t, dir)
		extra := itemNamed(t, p, "old.conf")

		steps := flatten(Steps(p, Selection{extra.Key(): true}, Resolutions{}, ApplyOptions{Backup: true}))

		if dels := stepsOfType[plan.DeleteStep](steps); len(dels) != 0 {
			t.Errorf("direction %v: delete steps = %+v", dir, dels)
		}
		if copies := stepsOfType[plan.CopyStep](steps); len(copies) != 0 {
			t.Errorf("direction %v: extra must not be copied: %+v", dir, copies)
		}
	}
}

// The log records that an edit happened, never what was written.
func TestApply_LogRecordsEditedItemAsAppliedFileWithoutContent(t *testing.T) {
	p := resolverPlan(t, plan.Push)
	a := itemNamed(t, p, "a.conf")
	p.Resolutions = editOf(a)
	r := &fakeRunner{}

	res := newService(t, &fakeInspector{state: state.State{RepoPath: testRepo}}, r).
		Apply(context.Background(), p, Selection{a.Key(): true}, ApplyOptions{}, nil)

	if res.Err != nil {
		t.Fatalf("Apply: %v", res.Err)
	}
	logs := stepsOfType[plan.AppendLogStep](r.steps)
	if len(logs) != 1 {
		t.Fatalf("log steps = %d", len(logs))
	}
	want := state.LogItem{Path: "hypr/a.conf", Kind: "file", Result: ItemApplied}
	if items := logs[0].Entry.Items; len(items) != 1 || items[0] != want {
		t.Errorf("log items = %+v, want [%+v]", items, want)
	}
	raw, err := json.Marshal(logs[0].Entry)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hunter2") {
		t.Errorf("log JSON leaks content: %s", raw)
	}
}

func TestChoicesFor_Table(t *testing.T) {
	text := Side{Exists: true, Kind: plan.KindFile, Size: 10}
	cases := []struct {
		name string
		it   PlanItem
		side Side
		want []Choice
	}{
		{"update text offers keep take edit", PlanItem{Change: change("a", plan.Update, plan.KindFile)}, text,
			[]Choice{ChoiceKeep, ChoiceTake, ChoiceEdit}},
		{"create offers skip take", PlanItem{Change: change("a", plan.Create, plan.KindFile)}, Side{},
			[]Choice{ChoiceSkip, ChoiceTake}},
		{"extra offers none", PlanItem{Change: change("a", plan.Delete, plan.KindFile)}, text, nil},
		{"link item offers keep take", PlanItem{Change: change("a", plan.Update, plan.KindLink)}, Side{Exists: true, Kind: plan.KindLink, LinkTarget: "t"},
			[]Choice{ChoiceKeep, ChoiceTake}},
		{"destination link offers keep take", PlanItem{Change: change("a", plan.Update, plan.KindFile)}, Side{Exists: true, Kind: plan.KindLink, LinkTarget: "t"},
			[]Choice{ChoiceKeep, ChoiceTake}},
		{"binary offers keep take", PlanItem{Change: change("a", plan.Update, plan.KindFile)}, Side{Exists: true, Kind: plan.KindFile, Binary: true},
			[]Choice{ChoiceKeep, ChoiceTake}},
		{"too large offers keep take", PlanItem{Change: change("a", plan.Update, plan.KindFile)}, Side{Exists: true, Kind: plan.KindFile, TooLarge: true},
			[]Choice{ChoiceKeep, ChoiceTake}},
		{"blocked offers none", PlanItem{Change: change("a", plan.Update, plan.KindFile), Blocked: true}, text, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ChoicesFor(c.it, c.side)
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("ChoicesFor = %v, want %v", got, c.want)
			}
		})
	}
}

func TestMaxEditBytesIsOneMiB(t *testing.T) {
	if MaxEditBytes != 1<<20 {
		t.Errorf("MaxEditBytes = %d", MaxEditBytes)
	}
}
