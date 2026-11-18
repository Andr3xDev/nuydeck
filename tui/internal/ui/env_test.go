package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/envvars"
)

// openEnv builds the Env screen as the shell does and enters it; the source popup is open and nothing is read.
func openEnv(t *testing.T, f *fakeEnv, deps Deps) (*Env, envvars.Locations) {
	t.Helper()
	root := NewRoot("", false, f.deps(deps))
	s, _, ok := sectionOf[*Env](root)
	if !ok {
		t.Fatal("no Env section")
	}
	locations, err := envvars.Paths(root.svc.Entries)
	if err != nil {
		t.Fatal(err)
	}
	feed(s, s.Enter())
	return s, locations
}

// The popup lists the repo copy first and the live copy second.
func chooseRepo(s *Env) { send(s, keyEnter) }
func chooseLive(s *Env) { send(s, keyDown, keyEnter) }

func TestEnvEntryAsksForASourceAndReadsNothing(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, _ := openEnv(t, f, Deps{})

	view := viewOf(s)

	if len(f.reads) != 0 {
		t.Errorf("reads before a choice = %v, want none", f.reads)
	}
	if !containsAny(view, "repo") || !containsAny(view, "live") {
		t.Errorf("the popup does not offer both copies:\n%s", view)
	}
}

func TestEnvChosenSourceDrivesThePathAndTheList(t *testing.T) {
	cases := []struct {
		name   string
		choose func(*Env)
		path   func(envvars.Locations) string
		want   string
	}{
		{"repo copy", chooseRepo, func(l envvars.Locations) string { return l.RepoPath }, "repo-value"},
		{"live copy", chooseLive, func(l envvars.Locations) string { return l.DestPath }, "live-value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeEnv(envRepoText, envLiveText)
			s, loc := openEnv(t, f, Deps{})

			tc.choose(s)

			if want := []string{tc.path(loc)}; !slices.Equal(f.reads, want) {
				t.Errorf("reads = %v, want %v", f.reads, want)
			}
			view := viewOf(s)
			if !strings.Contains(view, "THEME") || !strings.Contains(view, tc.want) {
				t.Errorf("list lacks THEME=%s:\n%s", tc.want, view)
			}
			if strings.Contains(view, "COMMENTED") {
				t.Errorf("a commented hl.env line was listed:\n%s", view)
			}
		})
	}
}

func TestEnvShowsLoadingUntilTheReadArrivesAndDropsStaleResults(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, _ := openEnv(t, f, Deps{})

	_, readRepo := s.Update(keyEnter)
	if !strings.Contains(viewOf(s), envLoadingText) {
		t.Errorf("no loading state before the read arrives:\n%s", viewOf(s))
	}
	if len(f.reads) != 0 {
		t.Errorf("reads = %v, want none before the cmd runs", f.reads)
	}
	staleMsg := readRepo()

	send(s, keyPress('s'), keyDown, keyEnter)
	s.Update(staleMsg)

	if view := viewOf(s); strings.Contains(view, "repo-value") || !strings.Contains(view, "live-value") {
		t.Errorf("a stale read overwrote the live copy:\n%s", view)
	}
	if strings.Contains(viewOf(s), envLoadingText) {
		t.Errorf("loading state still shown after the read:\n%s", viewOf(s))
	}
}

func TestEnvDismissedPopupLeavesAnEmptyListAndNoEdit(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, _ := openEnv(t, f, Deps{})

	send(s, keyEsc)
	send(s, keyPress('e'))

	if len(f.reads) != 0 || len(f.edits) != 0 {
		t.Errorf("reads=%v edits=%v, want neither without a source", f.reads, f.edits)
	}
	if !containsAny(viewOf(s), "choose a source") {
		t.Errorf("no message asking for a source:\n%s", viewOf(s))
	}
	if !claims(s.Keys(), "s") {
		t.Error("s is not available to reopen the popup")
	}
}

func TestEnvSwitchingSourceRereadsTheOtherCopy(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, loc := openEnv(t, f, Deps{})
	chooseRepo(s)

	send(s, keyPress('s'))
	send(s, keyDown, keyEnter)

	if want := []string{loc.RepoPath, loc.DestPath}; !slices.Equal(f.reads, want) {
		t.Errorf("reads = %v, want %v", f.reads, want)
	}
	if view := viewOf(s); !strings.Contains(view, "live-value") || strings.Contains(view, "repo-value") {
		t.Errorf("list did not switch to the live copy:\n%s", view)
	}
}

func TestEnvRefreshRereadsTheChosenCopy(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, loc := openEnv(t, f, Deps{})
	chooseLive(s)

	send(s, keyPress('r'))

	if want := []string{loc.DestPath, loc.DestPath}; !slices.Equal(f.reads, want) {
		t.Errorf("reads = %v, want %v", f.reads, want)
	}
	if !claims(s.Keys(), "r") {
		t.Error("r is not listed in the Env keys")
	}
}

func TestEnvEditOpensTheChosenCopyThenRereadsIt(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	f.afterEdit = "hl.env(\"THEME\", \"saved-value\")\n"
	s, loc := openEnv(t, f, Deps{})
	chooseLive(s)

	send(s, keyPress('e'))

	if want := []string{loc.DestPath}; !slices.Equal(f.edits, want) {
		t.Errorf("edits = %v, want %v", f.edits, want)
	}
	if len(f.reads) != 2 {
		t.Errorf("reads = %v, want the choice read plus one after the editor", f.reads)
	}
	if !strings.Contains(viewOf(s), "saved-value") {
		t.Errorf("list does not reflect the saved file:\n%s", viewOf(s))
	}
}

func TestEnvEditShowsThePreviewLineAndTheError(t *testing.T) {
	t.Run("dry-run preview", func(t *testing.T) {
		f := newFakeEnv(envRepoText, envLiveText)
		f.editPreview = "would run nvim /home/user/.config/hypr/configs/env_variables.lua"
		s, _ := openEnv(t, f, Deps{})
		chooseLive(s)

		send(s, keyPress('e'))

		if !strings.Contains(viewOf(s), f.editPreview) {
			t.Errorf("preview line not shown:\n%s", viewOf(s))
		}
	})
	t.Run("editor error", func(t *testing.T) {
		f := newFakeEnv(envRepoText, envLiveText)
		f.editErr = errors.New("no editor: set $VISUAL or $EDITOR")
		s, _ := openEnv(t, f, Deps{})
		chooseRepo(s)

		send(s, keyPress('e'))

		if !strings.Contains(viewOf(s), f.editErr.Error()) {
			t.Errorf("edit error not shown:\n%s", viewOf(s))
		}
	})
}

func TestEnvMissingAndEmptyCopiesShowAMessage(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		f := newFakeEnv(envRepoText, envLiveText)
		for path := range f.files {
			delete(f.files, path)
		}
		s, _ := openEnv(t, f, Deps{})

		chooseRepo(s)

		if view := viewOf(s); !containsAny(view, "missing", "not found", "does not exist") || strings.Contains(view, "THEME") {
			t.Errorf("want a missing-file message and no variables:\n%s", view)
		}
	})
	t.Run("no variables", func(t *testing.T) {
		f := newFakeEnv("-- nothing here\nlocal x = 1\n", envLiveText)
		s, _ := openEnv(t, f, Deps{})

		chooseRepo(s)

		if view := viewOf(s); !containsAny(view, "no variables", "no hl.env") {
			t.Errorf("want an empty-list message:\n%s", view)
		}
	})
}

func TestEnvWithoutAHyprEntryReadsNothingAndSaysSo(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	root := NewRoot("", false, Deps{ReadSide: f.ReadSide, Env: EnvDeps{Edit: f.Edit}})
	s, _, _ := sectionOf[*Env](root)
	feed(s, s.Enter())

	send(s, keyEnter)

	if len(f.reads) != 0 || len(f.edits) != 0 {
		t.Errorf("reads=%v edits=%v, want none", f.reads, f.edits)
	}
	if !strings.Contains(viewOf(s), "hypr") {
		t.Errorf("no message about the missing hypr entry:\n%s", viewOf(s))
	}
}

func TestEnvOnlyEverReadsOrEditsTheTwoEnvFileCopies(t *testing.T) {
	f := newFakeEnv(envRepoText, envLiveText)
	s, loc := openEnv(t, f, Deps{})

	chooseRepo(s)
	send(s, keyPress('s'), keyDown, keyEnter, keyPress('r'), keyPress('e'))

	allowed := []string{loc.RepoPath, loc.DestPath}
	for _, path := range slices.Concat(f.reads, f.edits) {
		if !slices.Contains(allowed, path) || strings.Contains(path, ".secrets") || strings.Contains(path, ".zshrc") {
			t.Errorf("path %q is outside the two env file copies", path)
		}
	}
}
