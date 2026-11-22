package osfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Andr3xDev/nuydeck/tui/internal/fsutil"
	"github.com/Andr3xDev/nuydeck/tui/internal/plan"
)

// parentInsideRoot reports whether the parent of target, with symlinks
// resolved, stays inside root: a symlinked parent would redirect the operation.
func parentInsideRoot(root, target string) (bool, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	realParent, err := fsutil.ResolveExisting(filepath.Dir(target))
	if err != nil {
		return false, err
	}
	return fsutil.Within(realRoot, realParent), nil
}

func deletePath(s plan.DeleteStep) plan.StepResult {
	root, target := filepath.Clean(s.Root), filepath.Clean(s.Path)
	if !fsutil.StrictlyWithin(root, target) {
		return failed(fmt.Errorf("refusing to delete %s: not inside %s", target, root))
	}
	inside, err := parentInsideRoot(root, target)
	if err != nil {
		return failed(err)
	}
	if !inside {
		return failed(fmt.Errorf("refusing to delete %s: parent resolves outside %s", target, root))
	}
	switch err := os.Remove(target); {
	case err == nil:
		return plan.StepResult{Status: plan.StepOK}
	case errors.Is(err, fs.ErrNotExist):
		return plan.StepResult{Status: plan.StepOK, Detail: "already absent"}
	case errors.Is(err, syscall.ENOTEMPTY):
		return plan.StepResult{Status: plan.StepOK, Detail: plan.KeptNotEmpty}
	default:
		return failed(err)
	}
}
