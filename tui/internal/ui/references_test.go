package ui

import (
	"errors"
	"strings"
	"testing"
)

func openReferences(t *testing.T, f *fakeReferences, dryRun bool) *References {
	t.Helper()
	s, _, ok := sectionOf[*References](NewRoot("", dryRun, Deps{References: f.deps()}))
	if !ok {
		t.Fatal("no References section")
	}
	feed(s, s.Enter())
	return s
}

func TestReferencesListNameDescriptionAndStatus(t *testing.T) {
	s := openReferences(t, newFakeReferences(), false)

	view := viewOf(s)

	for _, want := range []string{
		"arch-linux", "Distribution home", "present",
		"nuytek-theme", "GTK theme", "absent",
		"mystery", "No probe", "unknown",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestReferencesEnterDetectsStatusesOffTheEventLoop(t *testing.T) {
	s, _, _ := sectionOf[*References](NewRoot("", false, Deps{References: newFakeReferences().deps()}))

	cmd := s.Enter()

	if cmd == nil || strings.Contains(viewOf(s), "present") {
		t.Fatalf("Enter probed synchronously (cmd nil = %v), want the detection in a cmd:\n%s", cmd == nil, viewOf(s))
	}
	feed(s, cmd)
	if !strings.Contains(viewOf(s), "present") {
		t.Errorf("statuses missing after the detection msg arrived:\n%s", viewOf(s))
	}
}

func TestReferencesOpenKeyOpensTheSelectedLinkAndStaysActive(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		f := newFakeReferences()
		s := openReferences(t, f, dryRun)

		send(s, keyPress('o'), keyDown, keyPress('o'))

		want := []string{f.refs[0].URL, f.refs[1].URL}
		if len(f.opened) != 2 || f.opened[0] != want[0] || f.opened[1] != want[1] {
			t.Errorf("dry-run %v: opened = %v, want %v", dryRun, f.opened, want)
		}
	}
}

func TestReferencesOpenFailureShowsAnErrorLine(t *testing.T) {
	f := newFakeReferences()
	f.openErr = errors.New("xdg-open: executable file not found")
	s := openReferences(t, f, false)

	send(s, keyPress('o'))

	if !strings.Contains(viewOf(s), f.openErr.Error()) {
		t.Errorf("open error not shown:\n%s", viewOf(s))
	}
}

func TestReferencesClaimOpenButNotRefresh(t *testing.T) {
	s := openReferences(t, newFakeReferences(), false)

	if !claims(s.Keys(), "o") || claims(s.Keys(), "r") {
		t.Errorf("claims o = %v, r = %v, want o and no r", claims(s.Keys(), "o"), claims(s.Keys(), "r"))
	}
}
