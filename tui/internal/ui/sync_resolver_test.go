package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/manifest"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	resolverDst = "/home/"
	resolverSrc = "/repo/"
)

// resolverDeps fakes the sync, side-read and edit deps of the resolver; sides is read-only once the test starts.
type resolverDeps struct {
	mu        sync.Mutex
	plan      cfgsync.SyncPlan
	sides     map[string]cfgsync.Side
	gate      chan struct{} // when set, ReadSide blocks until it is closed
	reads     []string
	editReqs  []cfgsync.EditRequest
	editRes   cfgsync.EditResult
	editErr   error
	applied   cfgsync.SyncPlan
	appliedBy cfgsync.Selection

	applyCalls atomic.Int32
}

func newResolverDeps(dir plan.Direction, items ...cfgsync.PlanItem) *resolverDeps {
	return &resolverDeps{
		plan:  cfgsync.SyncPlan{Direction: dir, Items: items, Selection: cfgsync.Selection{}},
		sides: map[string]cfgsync.Side{},
	}
}

func (f *resolverDeps) deps() Deps {
	return Deps{PlanSync: f.PlanSync, ApplySync: f.Apply, ReadSide: f.ReadSide, Edit: f.Edit}
}

func (f *resolverDeps) PlanSync(context.Context, plan.Direction, []manifest.ResolvedEntry) (cfgsync.SyncPlan, error) {
	return f.plan, nil
}

func (f *resolverDeps) ReadSide(path string) (cfgsync.Side, error) {
	f.mu.Lock()
	f.reads = append(f.reads, path)
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-time.After(waitTimeout):
		}
	}
	return f.sides[path], nil
}

func (f *resolverDeps) Edit(_ context.Context, req cfgsync.EditRequest) (cfgsync.EditResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.editReqs = append(f.editReqs, req)
	return f.editRes, f.editErr
}

func (f *resolverDeps) Apply(_ context.Context, p cfgsync.SyncPlan, sel cfgsync.Selection, _ cfgsync.ApplyOptions, _ func(plan.StepProgress)) cfgsync.ApplyResult {
	f.mu.Lock()
	f.applied, f.appliedBy = p, sel
	f.mu.Unlock()
	f.applyCalls.Add(1)
	return cfgsync.ApplyResult{}
}

func (f *resolverDeps) editCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.editReqs)
}

func (f *resolverDeps) readPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reads...)
}

func (f *resolverDeps) withText(name, dst, src string) {
	f.sides[resolverDst+name] = textSide(dst)
	f.sides[resolverSrc+name] = textSide(src)
}

// resolverItem is a file entry, so its source and destination paths are the Src and Dst fields as they are.
func resolverItem(name string, t plan.ChangeType) cfgsync.PlanItem {
	return cfgsync.PlanItem{
		Change:    plan.Change{Entry: manifest.Entry{Root: "home", Path: name}, Type: t, Kind: plan.KindFile},
		FileEntry: true,
		Src:       resolverSrc + name,
		Dst:       resolverDst + name,
	}
}

func textSide(body string) cfgsync.Side {
	return cfgsync.Side{Exists: true, Kind: plan.KindFile, Size: int64(len(body)), Content: []byte(body), Mode: 0o644}
}

func itemKey(name string) cfgsync.ItemKey { return cfgsync.ItemKey{Root: "home", Path: name} }

// resolverRoot opens the Sync preview through a Root, so keys travel the way the program delivers them.
func resolverRoot(t *testing.T, f *resolverDeps) *driver {
	t.Helper()
	root := NewRoot("", false, f.deps())
	root.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	d := newDriver(t, root)
	_, cmd := root.Update(startSyncMsg{direction: f.plan.Direction})
	d.launch(cmd)
	s, _, _ := sectionOf[*Sync](root)
	d.must(t, "the preview", func() bool { return s.flow.stage == stagePreview })
	return d
}

func (d *driver) mustShow(t *testing.T, text string) {
	t.Helper()
	d.must(t, "the view to show "+text, func() bool { return strings.Contains(d.view(), text) })
}

func (d *driver) mustLoad(t *testing.T) {
	t.Helper()
	d.must(t, "the side to load", func() bool { return !strings.Contains(d.view(), "loading") })
}

// rowOf is the rendered line holding name.
func rowOf(view, name string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

// applyPreview accepts the preview and the confirm and waits for the apply the fake records.
func applyPreview(t *testing.T, d *driver, f *resolverDeps) {
	t.Helper()
	d.press(keyEnter)
	d.press(keyPress('y'))
	d.must(t, "the apply to start", func() bool { return f.applyCalls.Load() == 1 })
}

func (f *resolverDeps) appliedPlan() (cfgsync.SyncPlan, cfgsync.Selection) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.applied, f.appliedBy
}

func TestDirectionLabelsNameTheDirectionOfTheCopy(t *testing.T) {
	if got := directionLabel(plan.Push); got != "Push: repo -> PC" {
		t.Errorf("push label = %q", got)
	}
	if got := directionLabel(plan.Pull); got != "Pull: PC -> repo" {
		t.Errorf("pull label = %q", got)
	}
}

func TestSyncRowMarkersFollowTheChoice(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update), resolverItem("new.conf", plan.Create))
	f.withText("a.conf", "old\n", "new\n")
	f.editRes = cfgsync.EditResult{Changed: true, Content: []byte("mine\n"), Mode: 0o644}
	f.sides[resolverSrc+"new.conf"] = textSide("fresh\n")
	d := resolverRoot(t, f)
	d.mustLoad(t)

	if row := rowOf(d.view(), "a.conf"); !strings.Contains(row, "[ ] keep") {
		t.Fatalf("update row = %q, want [ ] keep", row)
	}
	if row := rowOf(d.view(), "new.conf"); !strings.Contains(row, "[ ] skip") {
		t.Fatalf("new row = %q, want [ ] skip", row)
	}
	d.press(keyPress(tea.KeySpace))
	if row := rowOf(d.view(), "a.conf"); !strings.Contains(row, "[x] take") {
		t.Errorf("update row after space = %q, want [x] take", row)
	}
	d.press(keyPress('e'))
	d.mustShow(t, "mine")
	d.press(keyPress('y'))
	d.must(t, "the edit marker", func() bool { return strings.Contains(rowOf(d.view(), "a.conf"), "[e] edit") })

	d.press(keyPress('j'))
	d.press(keyPress(tea.KeySpace))
	if row := rowOf(d.view(), "new.conf"); !strings.Contains(row, "[x] take") {
		t.Errorf("new row after space = %q, want [x] take", row)
	}
}

func TestSyncEditIsInertWhileTheSideLoads(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.gate = make(chan struct{})
	d := resolverRoot(t, f)

	d.mustShow(t, "loading")
	d.press(keyPress('e'))
	d.settle()
	if f.editCalls() != 0 {
		t.Fatal("e opened the editor before the side loaded")
	}

	close(f.gate)
	d.mustLoad(t)
	d.press(keyPress('e'))
	d.must(t, "the editor to open once loaded", func() bool { return f.editCalls() == 1 })
}

func TestSyncEditIsInertOnItemsThatCannotBeEdited(t *testing.T) {
	binary := cfgsync.Side{Exists: true, Kind: plan.KindFile, Size: 2048, Binary: true}
	large := cfgsync.Side{Exists: true, Kind: plan.KindFile, Size: cfgsync.MaxEditBytes + 1, TooLarge: true}
	link := cfgsync.Side{Exists: true, Kind: plan.KindLink, LinkTarget: "/somewhere"}
	blocked := resolverItem("it.conf", plan.Update)
	blocked.Blocked = true
	linkItem := resolverItem("it.conf", plan.Update)
	linkItem.Change.Kind = plan.KindLink

	tests := []struct {
		name string
		item cfgsync.PlanItem
		side cfgsync.Side
	}{
		{"binary", resolverItem("it.conf", plan.Update), binary},
		{"too large", resolverItem("it.conf", plan.Update), large},
		{"link", linkItem, link},
		{"blocked", blocked, textSide("old\n")},
		{"extra", resolverItem("it.conf", plan.Delete), textSide("old\n")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newResolverDeps(plan.Push, tt.item)
			f.sides[resolverDst+"it.conf"] = tt.side
			f.sides[resolverSrc+"it.conf"] = tt.side
			d := resolverRoot(t, f)
			d.mustLoad(t)
			d.settle()

			d.press(keyPress('e'))
			d.settle()
			if f.editCalls() != 0 {
				t.Error("e opened the editor")
			}
		})
	}
}

func TestSyncExtraRowsAreInformationOnly(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("gone.conf", plan.Delete), resolverItem("b.conf", plan.Update))
	f.withText("gone.conf", "x\n", "")
	d := resolverRoot(t, f)
	d.mustShow(t, "gone.conf")

	row := rowOf(d.view(), "gone.conf")
	if !strings.Contains(row, "extra") || strings.ContainsAny(row, "[]") {
		t.Fatalf("extra row = %q, want the label extra and no marker", row)
	}
	for _, k := range []rune{tea.KeySpace, 'e'} {
		d.press(keyPress(k))
	}
	d.settle()
	if f.editCalls() != 0 || strings.Contains(d.view(), "[x]") {
		t.Errorf("space or e acted on an extra: edits=%d", f.editCalls())
	}
}

func TestSyncSelectAllSkipsExtrasAndBlockedItems(t *testing.T) {
	blocked := resolverItem("blocked.conf", plan.Update)
	blocked.Blocked = true
	f := newResolverDeps(plan.Push,
		resolverItem("a.conf", plan.Update), resolverItem("c.conf", plan.Create),
		resolverItem("gone.conf", plan.Delete), blocked)
	f.withText("a.conf", "old\n", "new\n")
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('a'))
	view := d.view()
	if got := strings.Count(view, "[x] take"); got != 2 {
		t.Errorf("[x] take rows = %d, want the update and the new item", got)
	}
	for _, name := range []string{"gone.conf", "blocked.conf"} {
		if strings.Contains(rowOf(view, name), "[x]") {
			t.Errorf("a selected %s", name)
		}
	}
	d.press(keyPress('a'))
	if strings.Contains(d.view(), "[x]") {
		t.Error("a on a full selection must clear it")
	}
}

func TestSyncPullOffersTheSameChoicesAsPush(t *testing.T) {
	f := newResolverDeps(plan.Pull, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.editRes = cfgsync.EditResult{Changed: true, Content: []byte("mine\n"), Mode: 0o644}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	if view := d.view(); !strings.Contains(view, "Pull: PC -> repo") || !strings.Contains(rowOf(view, "a.conf"), "[ ] keep") {
		t.Fatalf("pull preview:\n%s", view)
	}
	d.press(keyPress(tea.KeySpace))
	if !strings.Contains(rowOf(d.view(), "a.conf"), "[x] take") {
		t.Error("space must take the source in a pull")
	}
	d.press(keyPress('e'))
	d.must(t, "the editor to open", func() bool { return f.editCalls() == 1 })
}

func TestSyncDiffViewLabelsDestinationAndSource(t *testing.T) {
	for _, tt := range []struct {
		dir   plan.Direction
		label string
	}{
		{plan.Push, "Push: repo -> PC"},
		{plan.Pull, "Pull: PC -> repo"},
	} {
		t.Run(tt.label, func(t *testing.T) {
			f := newResolverDeps(tt.dir, resolverItem("a.conf", plan.Update))
			f.withText("a.conf", "old line\n", "new line\n")
			d := resolverRoot(t, f)
			d.mustLoad(t)

			d.press(keyPress('v'))
			d.mustShow(t, "destination")
			view := d.view()
			for _, want := range []string{"source", tt.label, "old line", "new line"} {
				if !strings.Contains(view, want) {
					t.Errorf("diff view lacks %q:\n%s", want, view)
				}
			}

			d.press(keyEsc)
			d.settle()
			s, _, _ := sectionOf[*Sync](d.root)
			if s.flow.stage != stagePreview || strings.Contains(d.view(), "destination") || !strings.Contains(d.view(), "a.conf") {
				t.Errorf("esc must return from the diff to the list: stage=%v", s.flow.stage)
			}
		})
	}
}

func TestSyncDiffViewOfOpaqueItemsSaysDiffers(t *testing.T) {
	linkItem := resolverItem("it.conf", plan.Update)
	linkItem.Change.Kind = plan.KindLink
	tests := []struct {
		name     string
		item     cfgsync.PlanItem
		dst, src cfgsync.Side
		want     []string
	}{
		{
			"binary", resolverItem("it.conf", plan.Update),
			cfgsync.Side{Exists: true, Kind: plan.KindFile, Size: 2048, Binary: true},
			cfgsync.Side{Exists: true, Kind: plan.KindFile, Size: 4096, Binary: true},
			[]string{"differs"},
		},
		{
			"link", linkItem,
			cfgsync.Side{Exists: true, Kind: plan.KindLink, LinkTarget: "/old/target"},
			cfgsync.Side{Exists: true, Kind: plan.KindLink, LinkTarget: "/new/target"},
			[]string{"differs", "/old/target", "/new/target"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newResolverDeps(plan.Push, tt.item)
			f.sides[resolverDst+"it.conf"], f.sides[resolverSrc+"it.conf"] = tt.dst, tt.src
			d := resolverRoot(t, f)
			d.mustLoad(t)

			d.press(keyPress('v'))
			d.mustShow(t, "destination")
			view := d.view()
			for _, want := range tt.want {
				if !strings.Contains(view, want) {
					t.Errorf("diff view lacks %q:\n%s", want, view)
				}
			}
		})
	}
}

func TestSyncDiffViewSanitizesFileContent(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\x1b]0;pwn\x07 line\u202e\n", "new line\n")
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('v'))
	d.mustShow(t, "destination")
	raw := d.root.View().Content
	if strings.Contains(raw, "\x1b]0;pwn") || strings.Contains(raw, "\u202e") {
		t.Errorf("file content reached the screen unsanitized: %q", raw)
	}
}

func TestSyncLoadsOnlyTheItemUnderTheCursor(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update), resolverItem("b.conf", plan.Update), resolverItem("c.conf", plan.Update))
	for _, n := range []string{"a.conf", "b.conf", "c.conf"} {
		f.withText(n, "old\n", "new\n")
	}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	for _, p := range f.readPaths() {
		if !strings.HasSuffix(p, "a.conf") {
			t.Fatalf("read %s before the cursor reached it", p)
		}
	}
	d.press(keyPress('j'))
	d.must(t, "the second item to be read", func() bool {
		for _, p := range f.readPaths() {
			if p == resolverDst+"b.conf" {
				return true
			}
		}
		return false
	})
	for _, p := range f.readPaths() {
		if strings.HasSuffix(p, "c.conf") {
			t.Errorf("read %s without the cursor on it", p)
		}
	}
}

// collect runs a command and flattens batches into the messages the runtime would deliver.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, collect(c)...)
	}
	return out
}

func TestSyncDropsASideReadFromAnEarlierPlan(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	s := newSync(testServices(f.deps()))

	_, readCmd := s.Update(s.startPlan(plan.Push, nil)())
	stale := collect(readCmd)
	if len(stale) == 0 {
		t.Fatal("the plan must return the read of the side under the cursor")
	}

	_, freshCmd := s.Update(s.startPlan(plan.Push, nil)())
	for _, msg := range stale {
		s.Update(msg)
	}
	if view := ansi.Strip(s.View(100, 20)); !strings.Contains(view, "loading") {
		t.Fatalf("a stale side read loaded the new plan:\n%s", view)
	}
	for _, msg := range collect(freshCmd) {
		s.Update(msg)
	}
	if view := ansi.Strip(s.View(100, 20)); strings.Contains(view, "loading") || !strings.Contains(view, "[ ] keep") {
		t.Errorf("the current side read must load the row:\n%s", view)
	}
}

func TestSyncEditStoresTheResultOnlyAfterConfirmation(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old body\n", "src body\n")
	f.editRes = cfgsync.EditResult{Changed: true, Content: []byte("new body\n"), Mode: 0o640}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('e'))
	d.mustShow(t, "new body")
	if view := d.view(); !strings.Contains(view, "old body") || !strings.Contains(rowOf(view, "a.conf"), "[ ] keep") {
		t.Fatalf("confirmation must show the result against the destination and leave the choice alone:\n%s", view)
	}
	if req := f.editReqs[0]; req.Destination != resolverDst+"a.conf" || req.Source != resolverSrc+"a.conf" {
		t.Errorf("edit request = %+v, want the item's destination and source", req)
	}

	d.press(keyPress('y'))
	applyPreview(t, d, f)
	p, sel := f.appliedPlan()
	got, ok := p.Resolutions[itemKey("a.conf")]
	if !ok || !bytes.Equal(got.Content, []byte("new body\n")) || got.Mode != 0o640 || !sel[itemKey("a.conf")] {
		t.Errorf("applied resolution = %+v (stored %v), selected %v, want the edit stored and the item selected", got, ok, sel)
	}
}

func TestSyncDecliningTheEditResultStoresNothing(t *testing.T) {
	for name, decline := range map[string]tea.KeyPressMsg{"n": keyPress('n'), "esc": keyEsc} {
		t.Run(name, func(t *testing.T) {
			f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
			f.withText("a.conf", "old body\n", "src body\n")
			f.editRes = cfgsync.EditResult{Changed: true, Content: []byte("new body\n"), Mode: 0o644}
			d := resolverRoot(t, f)
			d.mustLoad(t)

			d.press(keyPress('e'))
			d.mustShow(t, "new body")
			d.press(decline)
			d.settle()
			if view := d.view(); strings.Contains(view, "new body") || !strings.Contains(rowOf(view, "a.conf"), "[ ] keep") {
				t.Fatalf("declined edit must close the popup and keep the choice:\n%s", view)
			}

			d.press(keyPress(tea.KeySpace))
			applyPreview(t, d, f)
			p, _ := f.appliedPlan()
			if _, stored := p.Resolutions[itemKey("a.conf")]; stored {
				t.Error("a declined result was stored")
			}
		})
	}
}

func TestSyncDeselectingAnEditedItemDropsItsResult(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old body\n", "src body\n")
	f.editRes = cfgsync.EditResult{Changed: true, Content: []byte("new body\n"), Mode: 0o644}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress('e'))
	d.mustShow(t, "new body")
	d.press(keyPress('y'))
	d.must(t, "the edit marker", func() bool { return strings.Contains(rowOf(d.view(), "a.conf"), "[e] edit") })

	d.press(keyPress(tea.KeySpace))
	if !strings.Contains(rowOf(d.view(), "a.conf"), "[ ] keep") {
		t.Fatal("space on an edited item must go back to keep")
	}
	d.press(keyPress(tea.KeySpace))
	applyPreview(t, d, f)
	if p, _ := f.appliedPlan(); len(p.Resolutions) != 0 {
		t.Errorf("resolutions = %v, want the deselected edit dropped", p.Resolutions)
	}
}

func TestSyncUnchangedEditResultKeepsTheDestination(t *testing.T) {
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.editRes = cfgsync.EditResult{Changed: false}
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress(tea.KeySpace))
	d.press(keyPress('e'))
	d.must(t, "the editor to open", func() bool { return f.editCalls() == 1 })
	d.settle()
	if !strings.Contains(rowOf(d.view(), "a.conf"), "[ ] keep") {
		t.Errorf("an unchanged result must make the choice keep:\n%s", d.view())
	}
}

func TestSyncEditorErrorGoesToTheNoticeBandAndKeepsTheChoice(t *testing.T) {
	const msg = "no editor: set $VISUAL or $EDITOR or install nano"
	f := newResolverDeps(plan.Push, resolverItem("a.conf", plan.Update))
	f.withText("a.conf", "old\n", "new\n")
	f.editErr = errors.New(msg)
	d := resolverRoot(t, f)
	d.mustLoad(t)

	d.press(keyPress(tea.KeySpace))
	d.press(keyPress('e'))
	d.mustShow(t, msg)
	if !strings.Contains(rowOf(d.view(), "a.conf"), "[x] take") {
		t.Errorf("an editor error must leave the choice unchanged:\n%s", d.view())
	}
}
