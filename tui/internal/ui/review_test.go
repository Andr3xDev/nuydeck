package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Andr3xDev/nuydeck/tui/internal/backup"
	"github.com/Andr3xDev/nuydeck/tui/internal/install"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// gatedRun starts a run whose work waits for proceed, then does after.
func gatedRun(t *testing.T, ack chan bool, proceed chan struct{}, after func(ctx context.Context, emit func(plan.Event)) runEnd) *driver {
	t.Helper()
	return runningRoot(t, false, focusPanel, ack, func(ctx context.Context, emit func(plan.Event)) runEnd {
		<-proceed
		return after(ctx, emit)
	})
}

func TestRunNoticeClearsWhenTheRebootPromptAppears(t *testing.T) {
	proceed := make(chan struct{})
	d := gatedRun(t, make(chan bool, 1), proceed, func(ctx context.Context, emit func(plan.Event)) runEnd {
		emit(plan.Event{Kind: plan.EventAwaitAck})
		<-ctx.Done()
		return runEnd{}
	})
	d.press(keyPress('x'))
	d.settle()
	if !strings.Contains(d.view(), runNotice) {
		t.Fatal("the refused key showed no notice")
	}

	close(proceed)
	d.must(t, "the reboot prompt", func() bool { return strings.Contains(d.view(), runAckText) })
	if strings.Contains(d.view(), runNotice) {
		t.Error("the notice still hides the reboot prompt")
	}
}

func TestRunNoticeClearsWhenTheRunFinishes(t *testing.T) {
	proceed := make(chan struct{})
	d := gatedRun(t, nil, proceed, finishesAtOnce)
	d.press(keyPress('x'))
	d.settle()

	close(proceed)
	d.must(t, "the run to finish", d.runDone)
	view := d.view()
	if !strings.Contains(view, runCloseText) || strings.Contains(view, runNotice) {
		t.Errorf("a finished run must show the close hint, not the notice:\n%s", view)
	}
}

func TestRunNoticeClearsOnTheNextAcceptedKey(t *testing.T) {
	d := gatedRun(t, nil, make(chan struct{}), finishesAtOnce)
	d.press(keyPress('x'))
	d.settle()
	d.press(keyUp)
	d.settle()

	if strings.Contains(d.view(), runNotice) {
		t.Error("an accepted key left the notice up")
	}
}

func TestRunFinishedFooterReplacesTheNotice(t *testing.T) {
	r := newRun(runRequest{footer: func(theme, int, *run) []string { return []string{"hook footer"} }}, nil)
	r.notice = true
	r.finish(runEnd{})

	if got := r.footerLines(ForVariant(""), 40); !strings.Contains(strings.Join(got, "\n"), "hook footer") {
		t.Errorf("footer = %q, want the hook's text after the run finished", got)
	}
}

func TestRunFollowResumesWhenScrolledBackToTheBottom(t *testing.T) {
	th := ForVariant("")
	r := newRun(runRequest{title: "t"}, nil)
	for i := range 50 {
		r.push(outLine{fmt.Sprintf("row-%02d", i)})
	}
	r.view(th, 40, 12)
	r.scrollBy(-1)
	r.view(th, 40, 12)
	if r.follow {
		t.Fatal("scrolling up must stop following")
	}

	r.scrollBy(1)
	r.view(th, 40, 12)

	if !r.follow {
		t.Error("scrolling back to the bottom must resume following")
	}
}

// syncRootAt plans a push through the Root and returns it at the preview.
func syncRootAt(t *testing.T, f *fakeSyncDeps, w, h int) (*Root, *Sync) {
	t.Helper()
	root := sizedRoot(f.deps(), w, h)
	_, cmd := root.Update(startSyncMsg{direction: plan.Push})
	root.Update(cmd())
	s, _, _ := sectionOf[*Sync](root)
	if s.flow.stage != stagePreview {
		t.Fatalf("stage = %v, want the preview", s.flow.stage)
	}
	return root, s
}

func TestRootEnterAtTheBackupOfferGoesToTheConfirmWithBackup(t *testing.T) {
	root, s := syncRootAt(t, &fakeSyncDeps{plan: selectedPlan(), needsBackup: true}, 100, 30)
	root.Update(keyEnter)
	if s.flow.stage != stageOffer {
		t.Fatalf("stage = %v, want the offer", s.flow.stage)
	}

	root.Update(keyEnter)

	if s.flow.stage != stageConfirm || !stagedOf(s).confirmed.backup {
		t.Errorf("stage = %v backup = %v, want the confirm with backup", s.flow.stage, stagedOf(s).confirmed.backup)
	}
	keys := s.flow.Keys()
	if last := keys[len(keys)-1]; last.Help().Desc != "back" {
		t.Errorf("esc at the confirm is labelled %q, want back", last.Help().Desc)
	}
}

func TestRefreshIsListedOnlyForScreensThatRefresh(t *testing.T) {
	root, _ := newTestRoot(&fakeInstaller{plan: onePlan()})
	cases := map[string]bool{"Sync": false, "Install": false, "Dashboard": true}
	for i, s := range root.sections {
		want, ok := cases[s.title]
		if !ok {
			continue
		}
		root.selected, root.focus = i, focusPanel
		if got := claims(root.focusedKeys(), "r"); got != want {
			t.Errorf("%s: r listed = %v, want %v", s.title, got, want)
		}
	}
}

func TestPackagesFormTitleKeepsTheSpaceBeforeThePackageName(t *testing.T) {
	f := &fakePackagesDeps{names: []string{"htop"}}
	p, _, _ := sectionOf[*Packages](NewRoot("", false, Deps{Packages: f.deps()}))
	p.Update(unregisteredMsg{names: f.names})
	p.Update(keyPress('m'))

	if view := ansi.Strip(p.View(100, 30)); !strings.Contains(view, "Add htop to the catalog") {
		t.Errorf("form title wrong:\n%s", view)
	}
}

func TestListRowsStayOneLine(t *testing.T) {
	rows := []listRow{{text: "home:a\nb"}, {text: strings.Repeat("long ", 20), badges: []string{"incomplete"}}}

	lines, _ := listLines(ForVariant(""), rows, 0, 20)

	if len(lines) != len(rows) {
		t.Fatalf("%d lines for %d rows: %q", len(lines), len(rows), lines)
	}
	for _, l := range lines {
		if lipgloss.Width(l) > 20 {
			t.Errorf("row wider than 20 cells: %q", ansi.Strip(l))
		}
	}
	if !strings.Contains(ansi.Strip(lines[1]), "[incomplete]") {
		t.Errorf("the badge was cut with the text: %q", ansi.Strip(lines[1]))
	}
}

// assertBoxed checks the rendered Root fits w x h and every body line still ends in the panel's right border.
func assertBoxed(t *testing.T, name string, r *Root, w, h int) {
	t.Helper()
	lines := renderedLines(r)
	if len(lines) > h {
		t.Errorf("%s: %d lines, want at most %d", name, len(lines), h)
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got > w {
			t.Errorf("%s: line %d is %d cells wide: %q", name, i, got, l)
		}
		if i < len(lines)-1 && !strings.HasSuffix(strings.TrimRight(l, " "), "│") && !strings.HasSuffix(strings.TrimRight(l, " "), "╮") && !strings.HasSuffix(strings.TrimRight(l, " "), "╯") {
			t.Errorf("%s: line %d lost its right border: %q", name, i, l)
		}
	}
}

func TestNarrowTerminalsKeepTheRunPaneAndTheFlowDialogBoxed(t *testing.T) {
	for _, size := range []struct{ w, h int }{{40, 8}, {60, 12}} {
		run := sizedRoot(Deps{}, size.w, size.h)
		run.Update(runRequest{title: strings.Repeat("long title ", 8)})
		run.run.add(plan.Event{Kind: plan.EventLine, Line: strings.Repeat("x", 300)})
		run.run.awaitAck = true
		assertBoxed(t, fmt.Sprintf("run %dx%d ack", size.w, size.h), run, size.w, size.h)
		run.run.finish(runEnd{})
		assertBoxed(t, fmt.Sprintf("run %dx%d done", size.w, size.h), run, size.w, size.h)

		dialog, s := syncRootAt(t, &fakeSyncDeps{plan: selectedPlan(), needsBackup: true}, size.w, size.h)
		dialog.Update(keyEnter)
		if s.flow.stage != stageOffer {
			t.Fatalf("stage = %v, want the offer", s.flow.stage)
		}
		assertBoxed(t, fmt.Sprintf("dialog %dx%d", size.w, size.h), dialog, size.w, size.h)
	}
}

func TestSyncAndBackupsResetAnActiveFlowWhenAnyRunEnds(t *testing.T) {
	for _, msg := range []tea.Msg{runClosed{}, applyDoneMsg{}} {
		s := newPreview(&fakeSyncDeps{plan: selectedPlan()}, false)
		s.Update(msg)
		if s.flow.active() {
			t.Errorf("Sync kept its flow after %T", msg)
		}

		f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
		b := loadedBackups(f)
		b.Update(keyEnter)
		if !b.flow.active() {
			t.Fatal("restore did not start a flow")
		}
		b.Update(msg)
		if b.flow.active() {
			t.Errorf("Backups kept its flow after %T", msg)
		}
	}
}

func TestPinnedKeysShowOnlyCtrlCQuitWhileThePanelCapturesText(t *testing.T) {
	root, in := newTestRoot(&fakeInstaller{plan: onePlan()}, "TOKEN")
	toHardware(in)
	settle(in, tap(in, specialKey(tea.KeyEnter)))
	_, root.selected, _ = sectionOf[*Install](root)
	root.focus = focusPanel
	root.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	pinned := root.pinnedKeys()
	if len(pinned) != 1 || !claims(pinned, "ctrl+c") || claims(pinned, "q") {
		t.Fatalf("pinned keys = %v, want only ctrl+c quit", pinned)
	}
	if view := ansi.Strip(root.View().Content); !strings.Contains(view, "ctrl+c quit") || strings.Contains(view, "? help") {
		t.Errorf("keybar advertises keys the input swallows:\n%s", view)
	}
}

func TestInstallLeaveForgetsSecretsInThePlanAndTheInput(t *testing.T) {
	planWithSecret := install.InstallPlan{Steps: []install.PlannedStep{{Phase: install.PhaseEnv, Step: plan.WriteSecretsStep{
		Path: "/home/u/.secrets", Entries: []plan.SecretEntry{{Name: "TOKEN", Value: secretValue}},
	}}}}
	_, in := newTestRoot(&fakeInstaller{plan: planWithSecret}, "TOKEN")
	toSummaryWithSecret(t, in)
	in.secretInput.SetValue(secretValue)

	in.Leave()

	if dump := fmt.Sprintf("%#v", in.plan); strings.Contains(dump, secretValue) {
		t.Errorf("plan still holds the secret after Leave: %s", dump)
	}
	if in.secretInput.Value() != "" {
		t.Error("the secret input kept its text after Leave")
	}
}

func TestSyncOffersNoApplyWithoutApplySync(t *testing.T) {
	f := &fakeSyncDeps{plan: selectedPlan()}
	s := newSync(testServices(Deps{PlanSync: f.PlanSync}))
	runCmds(s, s.startPlan(plan.Push))

	if s.flow.canApply() || claims(s.Keys(), "enter") {
		t.Error("apply is offered although no ApplySync is wired")
	}
}

func TestBackupsKeysOmitRestoreAndDeleteWithoutASelectionOrWiring(t *testing.T) {
	empty := loadedBackups(&fakeBackups{})
	f := &fakeBackups{list: []backup.Backup{sampleBackup("/b/1")}}
	unwired := newBackups(testServices(Deps{ListBackups: f.ListBackups}))
	unwired.Update(unwired.load()())
	for name, b := range map[string]*Backups{"empty": empty, "unwired": unwired} {
		if claims(b.Keys(), "d") || claims(b.Keys(), "enter") {
			t.Errorf("%s: keys %v list delete or restore", name, b.Keys())
		}
	}
	if keys := loadedBackups(f).Keys(); !claims(keys, "d") || !claims(keys, "enter") {
		t.Errorf("wired list with a selection must list delete and restore: %v", keys)
	}
}

func TestInstallSummaryWrapsLongLinesInsteadOfCuttingThem(t *testing.T) {
	long := plan.CommandStep{Label: strings.Repeat("word ", 20) + "TAILMARK"}
	_, in := newTestRoot(&fakeInstaller{plan: install.InstallPlan{Steps: []install.PlannedStep{{Phase: install.PhaseEnv, Step: long}}}})
	toSummary(in)

	view := ansi.Strip(in.View(40, 30))

	if joined := strings.NewReplacer("│", "", "\n", "", " ", "").Replace(view); !strings.Contains(joined, "TAILMARK") {
		t.Errorf("the summary cut the end of a long step:\n%s", view)
	}
	for _, l := range strings.Split(view, "\n") {
		if lipgloss.Width(l) > 40 {
			t.Errorf("line wider than 40 cells: %q", l)
		}
	}
}
