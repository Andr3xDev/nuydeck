package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
	"github.com/Andr3xDev/nuydeck/tui/internal/services"
	"github.com/Andr3xDev/nuydeck/tui/internal/ui/kit"
)

// openServices builds the Services screen as the shell does and loads its state through Enter.
func openServices(t *testing.T, f *fakeServices, term *fakeTerminal, dryRun bool) *Services {
	t.Helper()
	deps := Deps{Services: f.deps(), RunPlan: f.Run}
	if term != nil {
		deps.Terminal = term
	}
	s, _, ok := sectionOf[*Services](NewRoot("", dryRun, deps))
	if !ok {
		t.Fatal("no Services section")
	}
	feed(s, s.Enter())
	return s
}

func onRow(s screen, row int) { send(s, downTimes(row)...) }

func TestServicesEnterListsEveryUnitWithItsLiveState(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)

	view := viewOf(s)

	for _, want := range []string{
		unitNetworkManager, unitBluetooth, unitGhost, unitMasked, unitBroken, unitClipboardTimer,
		"system", "--user", "enabled", "disabled", "active", "inactive", "masked", "failed", "not found", "critical",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

func TestServicesRenderAsATableWithAHeaderAndFlags(t *testing.T) {
	s := openServices(t, newFakeServices(), nil, false)

	lines := strings.Split(viewOf(s), "\n")
	var header string
	for _, l := range lines {
		if strings.Contains(l, "Unit") && strings.Contains(l, "Scope") {
			header = l
		}
	}
	for _, col := range []string{"Unit", "Scope", "Enabled", "Active", "Flags"} {
		if !strings.Contains(header, col) {
			t.Fatalf("header lacks %q:\n%s", col, viewOf(s))
		}
	}
	for _, l := range lines {
		if strings.Contains(l, unitNetworkManager) && !strings.Contains(l, "critical") {
			t.Errorf("critical flag missing from its row: %q", l)
		}
	}
}

func TestServicesUnreadableStateShowsUnknownInTheStateCells(t *testing.T) {
	f := newFakeServices()
	f.busErr = services.ErrNoUserBus
	s := openServices(t, f, nil, false)

	for _, l := range strings.Split(viewOf(s), "\n") {
		if strings.Contains(l, unitClipboardTimer) && strings.Count(l, "unknown") != 2 {
			t.Errorf("row %q must show unknown for enabled and active", l)
		}
	}
}

func TestServicesNarrowPanelDropsColumnsButKeepsTheUnit(t *testing.T) {
	s := openServices(t, newFakeServices(), nil, false)

	view := ansi.Strip(s.View(40, 30))

	if !strings.Contains(view, unitBluetooth) || strings.Contains(view, "Scope") {
		t.Errorf("narrow view must keep the unit and drop low-priority columns:\n%s", view)
	}
}

func TestServicesJumpKeysMoveFiveRowsAndClamp(t *testing.T) {
	s := openServices(t, newFakeServices(), nil, false)

	send(s, keyPress('J'))
	if s.cursor != 5 {
		t.Fatalf("J moved the cursor to %d, want 5", s.cursor)
	}
	send(s, keyPress('J'))
	send(s, keyPress('K'), keyPress('K'))
	if s.cursor != 0 || !kit.Claims(s.Keys(), "J") || !kit.Claims(s.Keys(), "K") {
		t.Errorf("cursor = %d, want clamped at 0 with J and K listed", s.cursor)
	}
}

func TestServicesMaskedUnitShowsMaskedOnce(t *testing.T) {
	f := newFakeServices()
	f.states[unitMasked] = services.UnitState{Found: true, Enabled: "masked", Active: services.ActiveInactive, Masked: true}
	s := openServices(t, f, nil, false)

	for _, line := range strings.Split(viewOf(s), "\n") {
		if !strings.Contains(line, unitMasked) {
			continue
		}
		if got := strings.Count(strings.Replace(line, unitMasked, "", 1), "masked"); got != 1 {
			t.Errorf("masked appears %d times in row %q, want 1", got, line)
		}
		return
	}
	t.Fatal("no row for the masked unit")
}

func TestServicesReadsEachScopeOnceWithOnlyItsCatalogUnits(t *testing.T) {
	f := newFakeServices()
	openServices(t, f, nil, false)

	if len(f.reads) != 2 {
		t.Fatalf("state reads = %d, want one per scope", len(f.reads))
	}
	for _, r := range f.reads {
		var want []string
		for _, e := range f.catalog {
			if e.Scope == r.scope {
				want = append(want, e.Unit)
			}
		}
		if got := slices.Sorted(slices.Values(r.units)); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
			t.Errorf("scope %s read %v, want %v", r.scope, got, want)
		}
	}
}

func TestServicesWithoutUserBusShowsUnknownUserRowsAndKeepsSystemRows(t *testing.T) {
	f := newFakeServices()
	f.busErr = services.ErrNoUserBus
	s := openServices(t, f, nil, false)

	view := viewOf(s)

	if !strings.Contains(view, "user systemd bus unavailable") || !strings.Contains(view, "unknown") {
		t.Errorf("no bus message or unknown state:\n%s", view)
	}
	if !strings.Contains(view, "failed") {
		t.Errorf("system rows lost their state:\n%s", view)
	}
}

func TestServicesRefreshKeyRereadsEveryScope(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	before := len(f.reads)

	send(s, keyPress('r'))

	if !kit.Claims(s.Keys(), "r") {
		t.Error("r is not listed in the Services keys")
	}
	if got := len(f.reads) - before; got != 2 {
		t.Errorf("r caused %d state reads, want 2 (system and user)", got)
	}
}

func TestServicesStateIsRefreshedWhenTheRunPaneClosesAfterItsOwnRun(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	onRow(s, rowBluetooth)
	if runs := send(s, keyPress('R'), keyEnter, keyPress('y')); len(runs) != 1 {
		t.Fatalf("run requests = %d, want 1", len(runs))
	}
	before := len(f.reads)

	_, cmd := s.Update(runClosed{})
	feed(s, cmd)

	if len(f.reads) == before {
		t.Error("closing the run pane did not re-read the unit states")
	}
	before = len(f.reads)
	_, cmd = s.Update(runClosed{})
	feed(s, cmd)
	if len(f.reads) != before {
		t.Error("a second runClosed re-read again, want one reload per run")
	}
}

func TestServicesIgnoresRunPanesItDidNotStart(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	before := len(f.reads)

	_, cmd := s.Update(runClosed{})
	feed(s, cmd)

	if len(f.reads) != before {
		t.Errorf("runClosed from another screen caused %d state reads, want none", len(f.reads)-before)
	}
}

func TestServicesDropsAStaleReadThatArrivesAfterANewerOne(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	_, stale := s.Update(keyPress('r'))
	staleMsgs := collectMsgs(stale)
	f.states[unitBluetooth] = services.UnitState{Found: true, Enabled: services.EnabledEnabled, Active: services.ActiveActive}
	_, fresh := s.Update(keyPress('r'))
	feed(s, fresh)

	for _, m := range staleMsgs {
		s.Update(m)
	}

	if got, _ := s.stateOf(f.entry(unitBluetooth)); got.Enabled != services.EnabledEnabled {
		t.Errorf("bluetooth enabled = %q, want the newer read to win over the stale one", got.Enabled)
	}
}

func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var msgs []tea.Msg
	for _, sub := range batch {
		msgs = append(msgs, collectMsgs(sub)...)
	}
	return msgs
}

func TestServicesActionKeysPlanTheirAction(t *testing.T) {
	cases := map[rune]plan.Action{
		'e': plan.ActionEnable, 'd': plan.ActionDisable, 's': plan.ActionStart, 'x': plan.ActionStop,
		'R': plan.ActionRestart, 'm': plan.ActionMask, 'u': plan.ActionUnmask,
	}
	for k, want := range cases {
		t.Run(string(k), func(t *testing.T) {
			f := newFakeServices()
			s := openServices(t, f, nil, false)
			onRow(s, rowBluetooth)

			send(s, keyPress(k))

			if len(f.planned) != 1 {
				t.Fatalf("plans requested = %d, want 1", len(f.planned))
			}
			got := f.planned[0]
			if got.Unit != unitBluetooth || got.Scope != plan.ScopeSystem || got.Action != want {
				t.Errorf("request = %+v, want %s %s on %s", got, want, plan.ScopeSystem, unitBluetooth)
			}
			if kit.Claims(s.Keys(), string(k)) {
				t.Errorf("%q is still listed while the flow is open", k)
			}
			if !kit.Claims(s.Keys(), "esc") {
				t.Error("the flow's esc is not listed")
			}
		})
	}
}

func TestServicesUserUnitActionIsPlannedInUserScope(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	onRow(s, rowClipboardTimer)

	send(s, keyPress('R'))

	if len(f.planned) != 1 || f.planned[0].Scope != plan.ScopeUser || f.planned[0].Unit != unitClipboardTimer {
		t.Errorf("planned = %+v, want the user timer in user scope", f.planned)
	}
}

func TestServicesRunsNothingUntilBothConfirms(t *testing.T) {
	cases := []struct {
		name  string
		row   int
		key   rune
		extra bool
	}{
		{"restart a plain unit", rowBluetooth, 'R', false},
		{"restart a critical unit", rowNetworkManager, 'R', false},
		{"stop a critical unit", rowNetworkManager, 'x', true},
		{"disable a critical unit", rowNetworkManager, 'd', true},
		{"mask a plain unit", rowBluetooth, 'm', true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeServices()
			s := openServices(t, f, nil, false)
			onRow(s, tc.row)

			runs := send(s, keyPress(tc.key), keyEnter)
			if len(runs) != 0 || len(f.ran) != 0 {
				t.Fatalf("a run started before the confirm: runs=%d ran=%d", len(runs), len(f.ran))
			}
			runs = send(s, keyPress('y'))
			if tc.extra {
				if len(runs) != 0 {
					t.Fatal("the normal confirm alone started the run, want an extra confirm first")
				}
				if !containsAny(viewOf(s), "critical", "masking") {
					t.Errorf("the extra confirm shows no warning:\n%s", viewOf(s))
				}
				runs = send(s, keyPress('y'))
			}
			if len(runs) != 1 {
				t.Fatalf("run requests = %d, want 1", len(runs))
			}
			if len(f.ran) != 0 {
				t.Error("the Runner was called before the run pane started the work")
			}
			if _, end := runWork(runs[0]); end.err != nil || len(f.ran) != 1 {
				t.Errorf("work end = %v, plans run = %d, want success and one plan", end.err, len(f.ran))
			}
		})
	}
}

func TestServicesDecliningAnyConfirmRunsNothing(t *testing.T) {
	t.Run("normal confirm", func(t *testing.T) {
		f := newFakeServices()
		s := openServices(t, f, nil, false)
		onRow(s, rowBluetooth)

		runs := send(s, keyPress('R'), keyEnter, keyPress('n'))

		if len(runs) != 0 || len(f.ran) != 0 {
			t.Errorf("declined confirm ran: runs=%d ran=%d", len(runs), len(f.ran))
		}
	})
	t.Run("extra confirm", func(t *testing.T) {
		f := newFakeServices()
		s := openServices(t, f, nil, false)
		onRow(s, rowNetworkManager)

		runs := send(s, keyPress('x'), keyEnter, keyPress('y'), keyPress('n'))

		if len(runs) != 0 || len(f.ran) != 0 {
			t.Errorf("declined extra confirm ran: runs=%d ran=%d", len(runs), len(f.ran))
		}
	})
}

func TestServicesFailedActionIsReportedWithUnitAndAction(t *testing.T) {
	f := newFakeServices()
	f.runErr = errors.New("exit status 1")
	s := openServices(t, f, nil, false)
	onRow(s, rowBluetooth)

	runs := send(s, keyPress('R'), keyEnter, keyPress('y'))
	if len(runs) != 1 {
		t.Fatalf("run requests = %d, want 1", len(runs))
	}
	_, end := runWork(runs[0])

	if end.err == nil {
		t.Fatal("a failed action ended the run without an error")
	}
	if shown := runs[0].title + " " + end.err.Error(); !strings.Contains(shown, unitBluetooth) || !strings.Contains(shown, "restart") {
		t.Errorf("failure shows %q, want the unit and the action", shown)
	}
}

func TestServicesMutatingKeysAreInertOnANotFoundUnit(t *testing.T) {
	f := newFakeServices()
	s := openServices(t, f, nil, false)
	onRow(s, rowGhost)

	runs := send(s, keyPress('R'), keyEnter, keyPress('y'))

	if len(runs) != 0 || len(f.ran) != 0 {
		t.Errorf("a not-found unit ran an action: runs=%d ran=%d", len(runs), len(f.ran))
	}
	if !containsAny(viewOf(s), "not found", "not installed") {
		t.Errorf("no message for the not-found unit:\n%s", viewOf(s))
	}
}

func TestServicesLogsHandTheJournalToTheTerminal(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		for _, tc := range []struct {
			name string
			row  int
			unit string
		}{{"system unit", rowBluetooth, unitBluetooth}, {"user unit", rowClipboardTimer, unitClipboardTimer}} {
			t.Run(tc.name, func(t *testing.T) {
				f := newFakeServices()
				term := &fakeTerminal{}
				s := openServices(t, f, term, dryRun)
				onRow(s, tc.row)

				runs := send(s, keyPress('L'))

				want, err := services.LogsCommand(f.entry(tc.unit))
				if err != nil {
					t.Fatal(err)
				}
				if len(term.cmds) != 1 || !slices.Equal(term.cmds[0].Args, want.Args) {
					t.Errorf("terminal commands = %v, want one %v (dry-run %v)", term.cmds, want.Args, dryRun)
				}
				if len(runs) != 0 || len(f.ran) != 0 {
					t.Error("logs went through a run or the Runner, want a read handoff only")
				}
			})
		}
	}
}

func TestServicesLogsStartFailureIsShown(t *testing.T) {
	term := &fakeTerminal{err: errors.New("exec: journalctl: executable file not found")}
	s := openServices(t, newFakeServices(), term, false)
	onRow(s, rowBluetooth)

	send(s, keyPress('L'))

	if !strings.Contains(viewOf(s), "journalctl: executable file not found") {
		t.Errorf("the start error is not shown:\n%s", viewOf(s))
	}
}

func TestServicesLowercaseLOpensNoPager(t *testing.T) {
	term := &fakeTerminal{}
	s := openServices(t, newFakeServices(), term, false)
	onRow(s, rowBluetooth)

	_, cmd := s.Update(keyPress('l'))
	feed(s, cmd)

	if len(term.cmds) != 0 {
		t.Errorf("l opened a pager: %v", term.cmds)
	}
	if kit.Claims(s.Keys(), "l") || !kit.Claims(s.Keys(), "L") {
		t.Errorf("keys claim l = %v, L = %v, want only L", kit.Claims(s.Keys(), "l"), kit.Claims(s.Keys(), "L"))
	}
}
