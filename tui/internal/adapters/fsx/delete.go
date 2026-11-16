package fsx

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

const keptNotEmpty = "kept (not empty)"

func deletePath(s plan.DeleteStep) plan.StepResult {
	root, target := filepath.Clean(s.Root), filepath.Clean(s.Path)
	if !fsutil.StrictlyWithin(root, target) {
		return failed(fmt.Errorf("refusing to delete %s: not inside %s", target, root))
	}
	// A symlinked parent would redirect the removal outside the root.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return failed(err)
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if errors.Is(err, fs.ErrNotExist) {
		return plan.StepResult{Status: plan.StepOK, Detail: "already absent"}
	}
	if err != nil {
		return failed(err)
	}
	if !fsutil.Within(realRoot, realParent) {
		return failed(fmt.Errorf("refusing to delete %s: parent resolves outside %s", target, root))
	}
	switch err := os.Remove(target); {
	case err == nil:
		return plan.StepResult{Status: plan.StepOK}
	case errors.Is(err, fs.ErrNotExist):
		return plan.StepResult{Status: plan.StepOK, Detail: "already absent"}
	case errors.Is(err, syscall.ENOTEMPTY):
		return plan.StepResult{Status: plan.StepOK, Detail: keptNotEmpty}
	default:
		return failed(err)
	}
}
