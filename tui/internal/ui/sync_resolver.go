package ui

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Andr3xDev/nuydeck/tui/internal/cfgsync"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

const minPopupLines = 4

// subStage is a view the preview opens over its list.
type subStage int

const (
	subNone   subStage = iota
	subDiff            // destination against source of the item under the cursor
	subResult          // the editor's result against the destination, waiting for y or n
)

// sideState is what the resolver knows of an item's two ends; it exists once the read was requested.
type sideState struct {
	dst, src cfgsync.Side
	err      error
	loaded   bool
}

// sidesMsg and editedMsg carry the staged that asked, so a result from an earlier plan is dropped.
type sidesMsg struct {
	st       *syncStaged
	key      cfgsync.ItemKey
	dst, src cfgsync.Side
	err      error
}

type editedMsg struct {
	st  *syncStaged
	key cfgsync.ItemKey
	res cfgsync.EditResult
	err error
}

func (st *syncStaged) Nested() bool { return st.sub != subNone }

func (st *syncStaged) Keys() []key.Binding {
	k := st.svc.keys
	switch st.sub {
	case subDiff:
		return append(k.Move(), k.Back)
	case subResult:
		return append(k.Move(), kit.Described(k.Yes, "use edit"), kit.Described(k.No, "discard"), k.Back)
	}
	return append(k.Move(), k.Toggle, k.All, k.Edit, k.Diff)
}

// Load reads both ends of the item under the cursor, once.
func (st *syncStaged) Load() tea.Cmd {
	it, ok := st.current()
	if !ok || st.svc.ReadSide == nil || st.sides[it.Key()] != nil {
		return nil
	}
	k, read := it.Key(), st.svc.ReadSide
	dstPath, srcPath := it.DstPath(), it.SrcPath()
	st.sides[k] = &sideState{}
	return func() tea.Msg {
		dst, dstErr := read(dstPath)
		src, srcErr := read(srcPath)
		return sidesMsg{st: st, key: k, dst: dst, src: src, err: errors.Join(dstErr, srcErr)}
	}
}

func (st *syncStaged) Handle(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case sidesMsg:
		if msg.st == st {
			st.sides[msg.key] = &sideState{dst: msg.dst, src: msg.src, err: msg.err, loaded: true}
			if it, ok := st.current(); ok && st.diffWanted && it.Key() == msg.key {
				st.diffWanted = false
				st.sub = subDiff
				st.subScroll.Reset()
			}
		}
	case editedMsg:
		if msg.st == st {
			st.receiveEdit(msg)
		}
	}
	return nil
}

// editable is true once the item's sides are read and its destination is a text file the owner may change.
func (st *syncStaged) editable(it cfgsync.PlanItem) bool {
	s := st.sides[it.Key()]
	return st.svc.Edit != nil && s != nil && s.loaded && s.err == nil &&
		slices.Contains(cfgsync.ChoicesFor(it, s.dst), cfgsync.ChoiceEdit)
}

func (st *syncStaged) edit() tea.Cmd {
	it, ok := st.current()
	if !ok || !st.editable(it) {
		return nil
	}
	k, edit, ctx := it.Key(), st.svc.Edit, st.svc.ctx
	req := cfgsync.EditRequest{Destination: it.DstPath(), Source: it.SrcPath(), Base: path.Base(k.Path)}
	return func() tea.Msg {
		res, err := edit(ctx, req)
		return editedMsg{st: st, key: k, res: res, err: err}
	}
}

// receiveEdit keeps the choice on an error, falls back to keep on no change, and otherwise waits for the owner's answer.
func (st *syncStaged) receiveEdit(msg editedMsg) {
	st.editNotices = msg.res.Notices
	switch {
	case msg.err != nil:
		st.editNotices = append(st.editNotices, msg.err.Error())
	case !msg.res.Changed:
		delete(st.res, msg.key)
		st.sel[msg.key] = false
	default:
		st.pending = &msg
		st.sub = subResult
		st.subScroll.Reset()
	}
}

func (st *syncStaged) updateSub(msg tea.KeyPressMsg) tea.Cmd {
	k := st.svc.keys
	switch {
	case k.Step(msg) != 0:
		st.subScroll.By(k.Step(msg))
	case st.sub == subResult && key.Matches(msg, k.Yes):
		st.res[st.pending.key] = cfgsync.Edited{Content: st.pending.res.Content, Mode: st.pending.res.Mode}
		st.sel[st.pending.key] = true
		st.closeSub()
	case key.Matches(msg, k.Back, k.No):
		st.closeSub()
	}
	return nil
}

func (st *syncStaged) closeSub() {
	st.sub, st.pending = subNone, nil
	st.subScroll.Reset()
}

// openDiff opens the diff of the item under the cursor; when its sides are not read yet it waits for them.
func (st *syncStaged) openDiff() tea.Cmd {
	it, ok := st.current()
	if !ok {
		return nil
	}
	if s := st.sides[it.Key()]; s != nil && s.loaded {
		st.sub = subDiff
		st.subScroll.Reset()
		return nil
	}
	st.diffWanted = true
	return st.Load()
}

// diffView shows both ends of the item under the cursor; the direction is in the flow header.
func (st *syncStaged) diffView(th kit.Theme, w, h int) []string {
	it, _ := st.current()
	s := st.sides[it.Key()]
	if s.err != nil {
		return styled(th.Err, kit.Wrap(s.err.Error(), w))
	}
	lines := kit.Wrap("- destination: "+it.DstPath(), w)
	lines = append(lines, kit.Wrap("+ source: "+it.SrcPath(), w)...)
	if opaque(s.dst) || opaque(s.src) {
		lines = append(lines, kit.Wrap("destination: "+describeSide(s.dst), w)...)
		lines = append(lines, kit.Wrap("source: "+describeSide(s.src), w)...)
		lines = append(lines, opaqueSummary(th, w, s.dst, s.src)...)
	} else {
		lines = append(lines, diffBody(th, w, cfgsync.DiffLines(textLines(s.dst.Content), textLines(s.src.Content)))...)
	}
	return st.subScroll.Window(lines, h)
}

// resultView is the popup under the list: the edit against the destination it would replace.
func (st *syncStaged) resultView(th kit.Theme, w, h int) []string {
	dst := st.sides[st.pending.key].dst
	diff := cfgsync.DiffLines(textLines(dst.Content), textLines(st.pending.res.Content))
	title := th.Title.Render(kit.Truncate("Edited result against the destination: y use it, n discard", w))
	return append([]string{title}, st.subScroll.Window(diffBody(th, w, diff), h-1)...)
}

func diffBody(th kit.Theme, w int, diff []cfgsync.DiffLine) []string {
	var lines []string
	for _, d := range diff {
		style := th.Plain
		switch d.Op {
		case '-':
			style = th.Err
		case '+':
			style = th.OK
		}
		lines = append(lines, styled(style, kit.Wrap(string(d.Op)+" "+d.Text, w))...)
	}
	return lines
}

// textLines splits file content into lines without a phantom last one.
func textLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// opaque sides have no text to diff.
func opaque(s cfgsync.Side) bool {
	return s.Exists && (s.Kind != plan.KindFile || s.Binary || s.TooLarge)
}

// maxDirChildLines caps the differing children a directory diff lists.
const maxDirChildLines = 50

// opaqueSummary says how two sides that cannot be diffed as text differ.
func opaqueSummary(th kit.Theme, w int, dst, src cfgsync.Side) []string {
	switch {
	case dst.Kind == plan.KindLink && src.Kind == plan.KindLink && dst.LinkTarget == src.LinkTarget:
		return styled(th.Muted, kit.Wrap("same link target", w))
	case dst.Kind == plan.KindDir || src.Kind == plan.KindDir:
		return dirDiffLines(th, w, dst, src)
	}
	return styled(th.Warning, kit.Wrap("differs", w))
}

// dirDiffLines lists the children that differ, as making the destination equal the source would change them.
// A child counts as changed when its kind or, for a file, its size differs; same-size edits and links with a changed target are not seen.
func dirDiffLines(th kit.Theme, w int, dst, src cfgsync.Side) []string {
	dstChildren := map[string]cfgsync.Child{}
	for _, c := range dst.Children {
		dstChildren[c.Name] = c
	}
	var lines []string
	count := 0
	note := func(style lipgloss.Style, mark, name string) {
		if count++; count <= maxDirChildLines {
			lines = append(lines, styled(style, kit.Wrap(mark+" "+name, w))...)
		}
	}
	for _, c := range src.Children {
		d, both := dstChildren[c.Name]
		delete(dstChildren, c.Name)
		switch {
		case !both:
			note(th.OK, "+", c.Name)
		case d.Kind != c.Kind || d.Kind == plan.KindFile && d.Size != c.Size:
			note(th.Warning, "~", c.Name)
		}
	}
	for _, c := range dst.Children {
		if _, left := dstChildren[c.Name]; left {
			note(th.Err, "-", c.Name)
		}
	}
	switch {
	case count == 0:
		return styled(th.Muted, kit.Wrap("no child differs; only the directory itself does", w))
	case count > maxDirChildLines:
		lines = append(lines, styled(th.Muted, kit.Wrap(fmt.Sprintf("... and %d more", count-maxDirChildLines), w))...)
	}
	return lines
}

func describeSide(s cfgsync.Side) string {
	switch {
	case !s.Exists:
		return "missing"
	case s.Kind == plan.KindLink && s.LinkBroken:
		return "link -> " + s.LinkTarget + " (broken: target does not exist)"
	case s.Kind == plan.KindLink:
		return "link -> " + s.LinkTarget + " (resolves)"
	case s.Kind == plan.KindDir:
		return fmt.Sprintf("directory, %d entries", len(s.Children))
	case s.Binary:
		return fmt.Sprintf("binary file, %d bytes", s.Size)
	case s.TooLarge:
		return fmt.Sprintf("file too large to show, %d bytes", s.Size)
	case s.Kind == plan.KindFile:
		return fmt.Sprintf("file, %d bytes", s.Size)
	}
	return "not a regular file"
}
