package cfgsync

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	editDest   = "/home/u/.config/app/app.conf"
	editSource = "/repo/config/app/app.conf"
	editTmpDir = "/run/user/1000/nuydeck-edit"
	editBase   = "app.conf"
)

type tempWrite struct {
	data []byte
	mode fs.FileMode
}

// fakeTemp is an in-memory TempFS that records every call.
type fakeTemp struct {
	files     map[string]tempWrite
	mkdirErr  error
	removeErr error
	removed   []string
	mkdirs    int
}

func newFakeTemp() *fakeTemp { return &fakeTemp{files: map[string]tempWrite{}} }

func (f *fakeTemp) MkdirPrivate() (string, error) {
	f.mkdirs++
	if f.mkdirErr != nil {
		return "", f.mkdirErr
	}
	return editTmpDir, nil
}

func (f *fakeTemp) WriteFile(p string, data []byte, mode fs.FileMode) error {
	f.files[p] = tempWrite{data: append([]byte(nil), data...), mode: mode}
	return nil
}

func (f *fakeTemp) ReadFile(p string) ([]byte, error) {
	w, ok := f.files[p]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return w.data, nil
}

func (f *fakeTemp) RemoveAll(p string) error {
	f.removed = append(f.removed, p)
	return f.removeErr
}

// fakeTerminal runs onRun in place of the editor and keeps the command it was given.
type fakeTerminal struct {
	cmd   *exec.Cmd
	calls int
	onRun func(cmd *exec.Cmd) error
}

func (f *fakeTerminal) Run(_ context.Context, cmd *exec.Cmd) error {
	f.calls++
	f.cmd = cmd
	if f.onRun != nil {
		return f.onRun(cmd)
	}
	return nil
}

type editFixture struct {
	temp  *fakeTemp
	term  *fakeTerminal
	sides map[string]Side
	env   map[string]string
	ports EditPorts
}

func newEditFixture() *editFixture {
	fx := &editFixture{
		temp: newFakeTemp(),
		term: &fakeTerminal{},
		sides: map[string]Side{
			editDest:   {Exists: true, Kind: plan.KindFile, Size: 4, Content: []byte("old\n"), Mode: 0o644},
			editSource: {Exists: true, Kind: plan.KindFile, Size: 4, Content: []byte("new\n"), Mode: 0o644},
		},
		env: map[string]string{"EDITOR": "vim"},
	}
	fx.ports = EditPorts{
		Getenv:     func(k string) string { return fx.env[k] },
		HasCommand: func(string) bool { return false },
		Terminal:   fx.term,
		Temp:       fx.temp,
		ReadSide:   func(p string) (Side, error) { return fx.sides[p], nil },
	}
	return fx
}

func (fx *editFixture) compose() (EditResult, error) {
	return ComposeEdit(context.Background(), fx.ports, EditRequest{Destination: editDest, Source: editSource, Base: editBase})
}

func tempPath(prefix string) string { return path.Join(editTmpDir, prefix+"."+editBase) }

func TestComposeEdit_RefusesNonEditableDestination(t *testing.T) {
	cases := map[string]Side{
		"missing":   {},
		"binary":    {Exists: true, Kind: plan.KindFile, Binary: true},
		"too large": {Exists: true, Kind: plan.KindFile, TooLarge: true, Size: MaxEditBytes + 1},
		"link":      {Exists: true, Kind: plan.KindLink, LinkTarget: "elsewhere"},
	}
	for name, side := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newEditFixture()
			fx.sides[editDest] = side

			_, err := fx.compose()

			if !errors.Is(err, ErrNotEditable) {
				t.Fatalf("err = %v, want ErrNotEditable", err)
			}
			if fx.temp.mkdirs != 0 || fx.term.calls != 0 {
				t.Errorf("mkdirs=%d terminal calls=%d, want nothing created or run", fx.temp.mkdirs, fx.term.calls)
			}
		})
	}
}

func TestComposeEdit_NoEditorCreatesNothing(t *testing.T) {
	fx := newEditFixture()
	fx.env = nil

	_, err := fx.compose()

	if err == nil || !strings.Contains(err.Error(), "no editor") {
		t.Fatalf("err = %v, want no editor error", err)
	}
	if fx.temp.mkdirs != 0 || fx.term.calls != 0 {
		t.Errorf("mkdirs=%d terminal calls=%d, want nothing created or run", fx.temp.mkdirs, fx.term.calls)
	}
}

func TestComposeEdit_StagesPrivateFilesAndPassesArgvInOrder(t *testing.T) {
	fx := newEditFixture()
	fx.env = map[string]string{"VISUAL": "code --wait"}
	var staged map[string]tempWrite
	fx.term.onRun = func(*exec.Cmd) error {
		staged = map[string]tempWrite{}
		for k, v := range fx.temp.files {
			staged[k] = v
		}
		return nil
	}

	if _, err := fx.compose(); err != nil {
		t.Fatalf("ComposeEdit: %v", err)
	}

	want := []string{"code", "--wait", tempPath("result"), tempPath("destination"), tempPath("source")}
	if !reflect.DeepEqual(fx.term.cmd.Args, want) {
		t.Errorf("argv = %q, want %q", fx.term.cmd.Args, want)
	}
	contents := map[string]string{
		tempPath("destination"): "old\n",
		tempPath("source"):      "new\n",
		tempPath("result"):      "old\n",
	}
	if len(staged) != len(contents) {
		t.Errorf("staged %d files, want %d: %v", len(staged), len(contents), staged)
	}
	for p, content := range contents {
		w, ok := staged[p]
		if !ok || string(w.data) != content || w.mode != 0o600 {
			t.Errorf("%s = %q mode %o (present %v), want %q mode 600", p, w.data, w.mode, ok, content)
		}
	}
}

func TestComposeEdit_NeverWritesOutsideTheTempDir(t *testing.T) {
	fx := newEditFixture()
	fx.term.onRun = func(*exec.Cmd) error {
		fx.temp.files[tempPath("result")] = tempWrite{data: []byte("edited\n"), mode: 0o600}
		return nil
	}

	if _, err := fx.compose(); err != nil {
		t.Fatalf("ComposeEdit: %v", err)
	}

	for p := range fx.temp.files {
		if path.Dir(p) != editTmpDir {
			t.Errorf("wrote %s outside %s", p, editTmpDir)
		}
	}
}

func TestComposeEdit_ReturnsEditedContentOnExitZero(t *testing.T) {
	fx := newEditFixture()
	fx.term.onRun = func(*exec.Cmd) error {
		fx.temp.files[tempPath("result")] = tempWrite{data: []byte("edited\n"), mode: 0o600}
		return nil
	}

	res, err := fx.compose()

	if err != nil {
		t.Fatalf("ComposeEdit: %v", err)
	}
	if !res.Changed || string(res.Content) != "edited\n" || res.Mode != 0o644 {
		t.Errorf("result = %+v, want Changed, edited content, destination mode 644", res)
	}
}

func TestComposeEdit_UnchangedResultIsNotChanged(t *testing.T) {
	fx := newEditFixture()

	res, err := fx.compose()

	if err != nil {
		t.Fatalf("ComposeEdit: %v", err)
	}
	if res.Changed {
		t.Errorf("Changed = true for a result equal to the destination")
	}
}

func TestComposeEdit_EditorFailureStoresNoResultAndIsReported(t *testing.T) {
	fx := newEditFixture()
	editorErr := errors.New("exit status 1")
	fx.term.onRun = func(*exec.Cmd) error {
		fx.temp.files[tempPath("result")] = tempWrite{data: []byte("half edit\n"), mode: 0o600}
		return editorErr
	}

	res, err := fx.compose()

	if !errors.Is(err, editorErr) {
		t.Errorf("err = %v, want the editor error", err)
	}
	if res.Changed || res.Content != nil {
		t.Errorf("result = %+v, want nothing stored", res)
	}
}

func TestComposeEdit_RemovesTempDirOnEveryOutcome(t *testing.T) {
	cases := map[string]func(fx *editFixture){
		"success": func(*editFixture) {},
		"editor failure": func(fx *editFixture) {
			fx.term.onRun = func(*exec.Cmd) error { return errors.New("exit status 1") }
		},
		"cancel": func(fx *editFixture) {
			fx.term.onRun = func(*exec.Cmd) error { return context.Canceled }
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newEditFixture()
			setup(fx)

			_, _ = fx.compose()

			if !reflect.DeepEqual(fx.temp.removed, []string{editTmpDir}) {
				t.Errorf("removed = %v, want [%s]", fx.temp.removed, editTmpDir)
			}
		})
	}
}

func TestComposeEdit_FailedCleanupIsReportedAsNotice(t *testing.T) {
	fx := newEditFixture()
	fx.temp.removeErr = errors.New("device busy")

	res, err := fx.compose()

	if err != nil {
		t.Fatalf("ComposeEdit: %v", err)
	}
	if len(res.Notices) != 1 || !strings.Contains(res.Notices[0], "device busy") {
		t.Errorf("Notices = %q, want one naming the removal error", res.Notices)
	}
}
