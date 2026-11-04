package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/app"
	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

type fakeDrifter struct {
	ctxs []context.Context
	out  chan app.DriftResult
}

func (f *fakeDrifter) Drift(ctx context.Context, _ []domain.ResolvedEntry) <-chan app.DriftResult {
	f.ctxs = append(f.ctxs, ctx)
	f.out = make(chan app.DriftResult, 8)
	return f.out
}

func resolved(path string) domain.ResolvedEntry {
	return domain.ResolvedEntry{Entry: domain.Entry{Root: "home", Path: path}}
}

func newTestDashboard(f *fakeDrifter, entries ...domain.ResolvedEntry) *Dashboard {
	root := NewRoot("", false, Deps{Drifter: f, Entries: entries})
	return root.sections[0].screen.(*Dashboard)
}

func plainView(d *Dashboard) string { return ansi.Strip(d.View(100, 20)) }

func TestDashboardDiscardsStaleGeneration(t *testing.T) {
	f := &fakeDrifter{}
	d := newTestDashboard(f, resolved("a"), resolved("b"))
	d.Init()
	staleGen := d.gen

	d.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if d.gen != staleGen+1 {
		t.Fatalf("gen = %d, want %d", d.gen, staleGen+1)
	}
	if f.ctxs[0].Err() == nil {
		t.Error("refresh must cancel the prior context")
	}

	_, cmd := d.Update(driftEntryMsg{gen: staleGen, result: app.DriftResult{Entry: resolved("a"), State: app.DriftDrifted, Changes: 3}})
	if cmd != nil || len(d.results) != 0 {
		t.Fatal("stale result must be discarded without re-issuing the read")
	}

	_, cmd = d.Update(driftEntryMsg{gen: d.gen, result: app.DriftResult{Entry: resolved("a"), State: app.DriftInSync}})
	if cmd == nil || len(d.results) != 1 {
		t.Fatal("current result must be stored and the read re-issued")
	}
}

func TestDashboardPerEntryStates(t *testing.T) {
	f := &fakeDrifter{}
	d := newTestDashboard(f, resolved("a"), resolved("b"), resolved("c"))
	d.Init()
	if strings.Count(plainView(d), "in sync") != 0 || !d.running {
		t.Fatal("entries must show a spinner before results arrive")
	}

	d.Update(driftEntryMsg{gen: d.gen, result: app.DriftResult{Entry: resolved("a"), State: app.DriftInSync}})
	d.Update(driftEntryMsg{gen: d.gen, result: app.DriftResult{Entry: resolved("b"), State: app.DriftError, Err: errors.New("boom")}})
	view := plainView(d)
	for _, want := range []string{"in sync", "error: boom"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
	if d.entryStatus(resolved("c").Entry) != d.spinner.View() {
		t.Error("entry without a result must still show the spinner")
	}

	d.Update(driftDoneMsg{gen: d.gen})
	if d.running {
		t.Error("done message must stop the spinner")
	}
}

func TestDashboardPullPushRequestSync(t *testing.T) {
	d := newTestDashboard(&fakeDrifter{}, resolved("a"))
	for key, want := range map[rune]domain.Direction{'p': domain.Pull, 'P': domain.Push} {
		_, cmd := d.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
		if got := cmd().(startSyncMsg).direction; got != want {
			t.Errorf("%q: direction = %v, want %v", key, got, want)
		}
	}
}

func TestRootOpensSyncOnStartSync(t *testing.T) {
	r := NewRoot("", false, Deps{})
	m, _ := r.Update(startSyncMsg{direction: domain.Push})
	r = m.(Root)
	if r.selected != syncSection || r.pendingSync == nil || *r.pendingSync != domain.Push {
		t.Errorf("selected=%d pending=%v", r.selected, r.pendingSync)
	}
}

func TestRootManifestErrorOffersNoOperations(t *testing.T) {
	r := NewRoot("", false, Deps{ManifestError: errors.New("paths.toml:7: duplicate path")})
	if len(r.sections) != 1 {
		t.Fatalf("sections = %d, want only the error screen", len(r.sections))
	}
	m, _ := r.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	r = m.(Root)
	for _, k := range []rune{'p', 'P', 'r'} {
		_, cmd := press(t, r, tea.KeyPressMsg{Code: k, Text: string(k)})
		if cmd != nil {
			t.Errorf("%q must do nothing on manifest error", k)
		}
	}
	if !strings.Contains(ansi.Strip(r.View().Content), "paths.toml:7: duplicate path") {
		t.Error("manifest error not shown")
	}
}
