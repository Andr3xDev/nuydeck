package osfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

const (
	userDirMode    fs.FileMode = 0o755
	keptNotSymlink             = "kept (not a symlink)"
)

func writeFile(s plan.WriteFileStep) plan.StepResult {
	if s.Privileged {
		return failed(fmt.Errorf("privileged write of %s needs the system runner", s.Path))
	}
	path := s.Path
	if s.Root != "" {
		// The cleaned path is what gets checked and written; a raw "link/../f" would resolve elsewhere.
		// The check-then-rename TOCTOU window is accepted for a single-user TUI.
		root := filepath.Clean(s.Root)
		path = filepath.Clean(s.Path)
		if !fsutil.StrictlyWithin(root, path) {
			return failed(fmt.Errorf("refusing to write %s: not inside %s", path, root))
		}
		inside, err := parentInsideRoot(root, path)
		if err != nil {
			return failed(err)
		}
		if !inside {
			return failed(fmt.Errorf("refusing to write %s: parent resolves outside %s", path, root))
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), userDirMode); err != nil {
		return failed(err)
	}
	return done(fsutil.WriteAtomic(path, s.Content, s.Mode))
}

func symlink(s plan.SymlinkStep) plan.StepResult {
	if err := os.MkdirAll(filepath.Dir(s.Path), userDirMode); err != nil {
		return failed(err)
	}
	if info, err := os.Lstat(s.Path); err == nil && info.Mode()&fs.ModeSymlink == 0 {
		return plan.StepResult{Status: plan.StepOK, Detail: keptNotSymlink}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failed(err)
	}
	tmp := s.Path + ".nuydeck-tmp"
	_ = os.Remove(tmp) // stale leftover of an interrupted run
	if err := os.Symlink(s.Target, tmp); err != nil {
		return failed(err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		_ = os.Remove(tmp)
		return failed(err)
	}
	return plan.StepResult{Status: plan.StepOK}
}

func done(err error) plan.StepResult {
	if err != nil {
		return failed(err)
	}
	return plan.StepResult{Status: plan.StepOK}
}
